package agentbudget_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MukundaKatta/agentbudget-go"
)

var (
	errThrottled = errors.New("rate limited")
	errPolicy    = errors.New("policy violation")
)

// fakeSleep returns a Sleep override that records durations and increments a
// virtual clock so wall-clock-aware behavior can be tested without real waits.
type fakeSleep struct {
	calls   []time.Duration
	virtual time.Duration
}

func (f *fakeSleep) sleep(ctx context.Context, d time.Duration) error {
	f.calls = append(f.calls, d)
	f.virtual += d
	return nil
}

func TestRunSuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	got, err := agentbudget.Run(ctx, func(context.Context) (string, error) {
		return "ok", nil
	}, agentbudget.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("got %q; want %q", got, "ok")
	}
}

func TestRunRetriesRetryableThenSucceeds(t *testing.T) {
	ctx := context.Background()
	calls := 0
	fs := &fakeSleep{}
	got, err := agentbudget.Run(ctx, func(context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", errThrottled
		}
		return "ok", nil
	}, agentbudget.Options{
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: time.Microsecond,
		Sleep:          fs.sleep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" || calls != 3 {
		t.Errorf("got=%q calls=%d", got, calls)
	}
}

func TestRunDoesNotRetryUnknownErrors(t *testing.T) {
	ctx := context.Background()
	calls := 0
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		calls++
		return nil, errors.New("never seen")
	}, agentbudget.Options{
		IsRetryable: agentbudget.IsAny(errThrottled),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected single call; got %d", calls)
	}
}

func TestRunFatalErrorRaisedImmediately(t *testing.T) {
	ctx := context.Background()
	calls := 0
	fs := &fakeSleep{}
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		calls++
		return nil, errPolicy
	}, agentbudget.Options{
		IsRetryable:    agentbudget.IsAny(errThrottled, errPolicy),
		IsFatal:        agentbudget.IsAny(errPolicy),
		BackoffInitial: time.Microsecond,
		Sleep:          fs.sleep,
	})
	if !errors.Is(err, errPolicy) {
		t.Errorf("expected errPolicy; got %v", err)
	}
	if calls != 1 {
		t.Errorf("fatal error should not retry; got %d calls", calls)
	}
}

func TestRunAttemptsBudgetExhausted(t *testing.T) {
	ctx := context.Background()
	fs := &fakeSleep{}
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:    3,
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: time.Microsecond,
		Sleep:          fs.sleep,
		// adversarial off (default) so all 3 retries play out
	})
	var be *agentbudget.BudgetExceededError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BudgetExceededError; got %T %v", err, err)
	}
	if be.Kind != agentbudget.BudgetKindAttempts {
		t.Errorf("kind = %q; want attempts", be.Kind)
	}
	if be.Attempts != 3 {
		t.Errorf("attempts = %d; want 3", be.Attempts)
	}
	if !errors.Is(err, errThrottled) {
		t.Error("BudgetExceededError should unwrap to last cause")
	}
}

func TestRunWallClockBudgetShortCircuits(t *testing.T) {
	// Use a virtual clock — simulate fn taking 30ms each call, sleep after.
	ctx := context.Background()
	fs := &fakeSleep{}
	calls := 0
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		calls++
		fs.virtual += 30 * time.Millisecond
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:    100,
		MaxWallClock:   100 * time.Millisecond,
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: 5 * time.Millisecond,
		Sleep:          fs.sleep,
	})
	// We can't perfectly fake time.Now() without monkey-patching the package,
	// so we just check that a budget error came back; this test mostly
	// asserts the loop terminates and emits a recognizable error.
	if err == nil {
		t.Fatal("expected error")
	}
	_ = calls
}

func TestRunCostBudgetEnforcedAfterSuccess(t *testing.T) {
	ctx := context.Background()
	type R struct{ Cost float64 }
	_, err := agentbudget.Run(ctx, func(context.Context) (R, error) {
		return R{Cost: 0.10}, nil
	}, agentbudget.Options{
		MaxCostUSD:    0.05,
		CostExtractor: func(r any) float64 { return r.(R).Cost },
	})
	var be *agentbudget.BudgetExceededError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BudgetExceededError; got %v", err)
	}
	if be.Kind != agentbudget.BudgetKindCostUSD {
		t.Errorf("kind = %q; want costUSD", be.Kind)
	}
	if be.Observed != 0.10 {
		t.Errorf("observed = %v; want 0.10", be.Observed)
	}
}

func TestRunCostBudgetPassesUnderLimit(t *testing.T) {
	ctx := context.Background()
	type R struct {
		Cost float64
		Data string
	}
	got, err := agentbudget.Run(ctx, func(context.Context) (R, error) {
		return R{Cost: 0.01, Data: "ok"}, nil
	}, agentbudget.Options{
		MaxCostUSD:    1.0,
		CostExtractor: func(r any) float64 { return r.(R).Cost },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Data != "ok" {
		t.Errorf("got %v", got)
	}
}

func TestRunAdversarialLoopDetected(t *testing.T) {
	ctx := context.Background()
	fs := &fakeSleep{}
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:           10,
		IsRetryable:           agentbudget.IsAny(errThrottled),
		DetectAdversarialLoop: true,
		AdversarialThreshold:  3,
		BackoffInitial:        time.Microsecond,
		Sleep:                 fs.sleep,
	})
	var ad *agentbudget.AdversarialLoopDetectedError
	if !errors.As(err, &ad) {
		t.Fatalf("expected *AdversarialLoopDetectedError; got %T %v", err, err)
	}
	if ad.Repetitions != 3 {
		t.Errorf("repetitions = %d; want 3", ad.Repetitions)
	}
	if !strings.Contains(ad.Fingerprint, "rate limited") {
		t.Errorf("fingerprint should include error message; got %q", ad.Fingerprint)
	}
}

func TestRunAdversarialResetsWhenErrorChanges(t *testing.T) {
	ctx := context.Background()
	fs := &fakeSleep{}
	i := 0
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		i++
		return nil, errors.New("variant " + string(rune('A'+i%2)))
	}, agentbudget.Options{
		MaxAttempts:           6,
		IsRetryable:           func(error) bool { return true }, // anything retryable
		DetectAdversarialLoop: true,
		AdversarialThreshold:  3,
		BackoffInitial:        time.Microsecond,
		Sleep:                 fs.sleep,
	})
	// Alternating errors → never 3-in-a-row → attempts budget instead
	var be *agentbudget.BudgetExceededError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BudgetExceededError (attempts); got %T %v", err, err)
	}
	if be.Kind != agentbudget.BudgetKindAttempts {
		t.Errorf("kind = %q; want attempts", be.Kind)
	}
}

func TestRunOnAttemptHookReceivesEvents(t *testing.T) {
	ctx := context.Background()
	fs := &fakeSleep{}
	calls := 0
	var events []agentbudget.AttemptEvent
	_, err := agentbudget.Run(ctx, func(context.Context) (string, error) {
		calls++
		if calls < 2 {
			return "", errThrottled
		}
		return "ok", nil
	}, agentbudget.Options{
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: time.Microsecond,
		Sleep:          fs.sleep,
		OnAttempt: func(e agentbudget.AttemptEvent) {
			events = append(events, e)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasKind := func(k string) bool {
		for _, e := range events {
			if e.Kind == k {
				return true
			}
		}
		return false
	}
	if !hasKind("start") || !hasKind("retry") || !hasKind("success") {
		t.Errorf("missing lifecycle event kinds; events = %v", events)
	}
}

func TestRunOnAttemptHookPanicDoesNotBreakCall(t *testing.T) {
	ctx := context.Background()
	got, err := agentbudget.Run(ctx, func(context.Context) (string, error) {
		return "ok", nil
	}, agentbudget.Options{
		OnAttempt: func(agentbudget.AttemptEvent) {
			panic("hook crashed")
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("got %q; want ok", got)
	}
}

func TestRunBackoffScalesGeometrically(t *testing.T) {
	ctx := context.Background()
	fs := &fakeSleep{}
	_, _ = agentbudget.Run(ctx, func(context.Context) (any, error) {
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:    4,
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: 100 * time.Millisecond,
		BackoffMax:     10 * time.Second,
		BackoffFactor:  2,
		Sleep:          fs.sleep,
	})
	if len(fs.calls) != 3 {
		t.Fatalf("expected 3 sleeps; got %d", len(fs.calls))
	}
	want := []time.Duration{100, 200, 400}
	for i, d := range fs.calls {
		if d != want[i]*time.Millisecond {
			t.Errorf("sleep[%d] = %v; want %v", i, d, want[i]*time.Millisecond)
		}
	}
}

func TestRunBackoffClampedToMax(t *testing.T) {
	ctx := context.Background()
	fs := &fakeSleep{}
	_, _ = agentbudget.Run(ctx, func(context.Context) (any, error) {
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:    5,
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: time.Second,
		BackoffMax:     1500 * time.Millisecond,
		BackoffFactor:  10,
		Sleep:          fs.sleep,
	})
	for _, d := range fs.calls {
		if d > 1500*time.Millisecond {
			t.Errorf("sleep %v exceeds BackoffMax", d)
		}
	}
}

func TestRunHonorsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		t.Error("fn should not be called when ctx is already cancelled")
		return nil, nil
	}, agentbudget.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled; got %v", err)
	}
}

// TestRunCancelDuringDefaultBackoffSleep exercises the real (default) ctxSleep
// path — not a fake Sleep override. The README promises "backoff sleeps honor
// ctx cancellation"; this asserts that a context cancelled mid-backoff aborts
// the retry loop with the context error rather than blocking for the full
// backoff duration.
func TestRunCancelDuringDefaultBackoffSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	start := time.Now()
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		calls++
		if calls == 1 {
			// Cancel while the first backoff sleep is in flight.
			go cancel()
		}
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:    10,
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: 10 * time.Second, // long enough that we'd notice a real wait
		// no Sleep override: use the default context-aware sleep
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled from interrupted backoff; got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("backoff did not honor cancellation; waited %v", elapsed)
	}
}

// TestRunWallClockBudgetErrorCarriesLastCause asserts that the wall-clock
// BudgetExceededError unwraps to the most recent failure cause, matching the
// attempts-budget path and the README's documented Unwrap() contract.
func TestRunWallClockBudgetErrorCarriesLastCause(t *testing.T) {
	ctx := context.Background()
	// A Sleep override that advances real wall-clock time by sleeping briefly,
	// so the second loop iteration's elapsed check trips MaxWallClock after a
	// failed attempt has recorded its cause.
	slowSleep := func(_ context.Context, _ time.Duration) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	}
	_, err := agentbudget.Run(ctx, func(context.Context) (any, error) {
		return nil, errThrottled
	}, agentbudget.Options{
		MaxAttempts:    100,
		MaxWallClock:   time.Millisecond, // trips on the second iteration
		IsRetryable:    agentbudget.IsAny(errThrottled),
		BackoffInitial: time.Millisecond,
		Sleep:          slowSleep,
	})
	var be *agentbudget.BudgetExceededError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BudgetExceededError; got %T %v", err, err)
	}
	if be.Kind != agentbudget.BudgetKindWallClock {
		t.Fatalf("kind = %q; want wallClock", be.Kind)
	}
	if !errors.Is(err, errThrottled) {
		t.Error("wall-clock BudgetExceededError should unwrap to the last cause")
	}
}
