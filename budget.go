package agentbudget

import (
	"context"
	"time"
)

// AttemptEvent is one observable lifecycle event in a Run call. Emitted to
// the OnAttempt hook (if set) at "start", "retry", "success", and "failure".
type AttemptEvent struct {
	Kind                string         // "start" | "retry" | "success" | "failure"
	Attempt             int            // 1-indexed; 0 only for "start"
	CumulativeCostUSD   float64
	CumulativeLatency   time.Duration
	LastError           error
	ErrorClassification Classification
}

// Options configures a Run call. The zero value is sensible: 5 attempts,
// adversarial detection on at threshold 3, 500ms initial backoff doubling
// to 30s, no cost or wall-clock cap.
type Options struct {
	// MaxAttempts is the cap on how many times Run will call fn. Default 5.
	MaxAttempts int

	// MaxCostUSD is the cumulative-cost cap across successful sub-calls
	// (cost extracted via CostExtractor). Zero means "no cap".
	MaxCostUSD float64

	// MaxWallClock is the cumulative time cap across all attempts and
	// backoffs. Zero means "no cap".
	MaxWallClock time.Duration

	// IsRetryable returns true for errors that should be retried.
	IsRetryable func(error) bool

	// IsFatal returns true for errors that should be re-raised immediately
	// without retry. Beats IsRetryable.
	IsFatal func(error) bool

	// CostExtractor returns the USD cost contributed by a successful result.
	// If nil, cost is not tracked and MaxCostUSD has no effect.
	CostExtractor func(any) float64

	// DetectAdversarialLoop enables fingerprint-based loop detection.
	// Default false on the zero value; set true to opt in.
	DetectAdversarialLoop bool

	// AdversarialThreshold is the number of consecutive identical-fingerprint
	// failures that triggers AdversarialLoopDetectedError. Default 3 when
	// DetectAdversarialLoop is true.
	AdversarialThreshold int

	// BackoffInitial is the first inter-attempt sleep. Default 500ms.
	BackoffInitial time.Duration

	// BackoffMax is the cap on exponentially-grown backoff. Default 30s.
	BackoffMax time.Duration

	// BackoffFactor multiplies BackoffInitial on each retry. Default 2.0.
	BackoffFactor float64

	// OnAttempt is called for every lifecycle event. Hook errors are
	// swallowed; instrumentation never breaks the wrapped call.
	OnAttempt func(AttemptEvent)

	// Sleep is the time-passing function. Defaults to a context-aware sleep
	// that honors ctx cancellation. Override for tests.
	Sleep func(ctx context.Context, d time.Duration) error
}

func (o Options) defaults() Options {
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 5
	}
	if o.AdversarialThreshold <= 0 {
		o.AdversarialThreshold = 3
	}
	if o.BackoffInitial <= 0 {
		o.BackoffInitial = 500 * time.Millisecond
	}
	if o.BackoffMax <= 0 {
		o.BackoffMax = 30 * time.Second
	}
	if o.BackoffFactor <= 0 {
		o.BackoffFactor = 2.0
	}
	if o.Sleep == nil {
		o.Sleep = ctxSleep
	}
	return o
}

func ctxSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Run wraps fn with retry + budget + adversarial detection.
//
// Behavior:
//
//   - Success path: fn's result is returned. If CostExtractor is set, its
//     return value is added to cumulative cost; if cumulative > MaxCostUSD,
//     returns BudgetExceededError{Kind: BudgetKindCostUSD}.
//
//   - On error classified Fatal (per IsFatal): immediately returned. No retry.
//
//   - On error classified Retryable (per IsRetryable): retried with
//     exponential backoff (clamped to BackoffMax and to remaining
//     MaxWallClock). Backoff sleeps honor ctx cancellation.
//
//   - On Unknown error (in neither set): returned immediately. Run never
//     blindly retries an exception class you didn't opt into.
//
//   - When DetectAdversarialLoop is true and the same Fingerprint(err) repeats
//     AdversarialThreshold times in a row, returns AdversarialLoopDetectedError.
//     This catches retry-amplification (e.g. an LLM always failing JSON
//     validation with the same payload).
//
// Run is generic over the result type T and is the canonical way to use
// agentbudget — no decorator needed, just call it where you'd otherwise have
// a bare retry loop.
func Run[T any](ctx context.Context, fn func(context.Context) (T, error), opts Options) (T, error) {
	o := opts.defaults()
	var zero T

	emit := func(evt AttemptEvent) {
		if o.OnAttempt == nil {
			return
		}
		defer func() { _ = recover() }() // hooks must not crash the call
		o.OnAttempt(evt)
	}

	started := time.Now()
	cumulativeCost := 0.0
	attempt := 0
	lastFingerprint := ""
	consecutiveIdentical := 0
	backoff := o.BackoffInitial

	emit(AttemptEvent{
		Kind:                "start",
		Attempt:             0,
		ErrorClassification: ClassificationNone,
	})

	for {
		attempt++

		elapsed := time.Since(started)
		if o.MaxWallClock > 0 && elapsed >= o.MaxWallClock {
			return zero, &BudgetExceededError{
				Kind:     BudgetKindWallClock,
				Limit:    float64(o.MaxWallClock),
				Observed: float64(elapsed),
				Attempts: attempt - 1,
			}
		}

		if err := ctx.Err(); err != nil {
			return zero, err
		}

		result, err := fn(ctx)
		if err == nil {
			if o.CostExtractor != nil {
				cumulativeCost += o.CostExtractor(result)
				if o.MaxCostUSD > 0 && cumulativeCost > o.MaxCostUSD {
					return zero, &BudgetExceededError{
						Kind:     BudgetKindCostUSD,
						Limit:    o.MaxCostUSD,
						Observed: cumulativeCost,
						Attempts: attempt,
					}
				}
			}
			emit(AttemptEvent{
				Kind:                "success",
				Attempt:             attempt,
				CumulativeCostUSD:   cumulativeCost,
				CumulativeLatency:   time.Since(started),
				ErrorClassification: ClassificationNone,
			})
			return result, nil
		}

		classification := Classify(err, o.IsRetryable, o.IsFatal)
		fingerprint := Fingerprint(err)

		if o.DetectAdversarialLoop {
			if fingerprint == lastFingerprint {
				consecutiveIdentical++
			} else {
				consecutiveIdentical = 1
			}
			lastFingerprint = fingerprint
			if consecutiveIdentical >= o.AdversarialThreshold {
				emit(AttemptEvent{
					Kind:                "failure",
					Attempt:             attempt,
					CumulativeCostUSD:   cumulativeCost,
					CumulativeLatency:   time.Since(started),
					LastError:           err,
					ErrorClassification: classification,
				})
				return zero, &AdversarialLoopDetectedError{
					Repetitions: consecutiveIdentical,
					Fingerprint: fingerprint,
					Last:        err,
				}
			}
		}

		if classification == ClassificationFatal || classification == ClassificationUnknown {
			emit(AttemptEvent{
				Kind:                "failure",
				Attempt:             attempt,
				CumulativeCostUSD:   cumulativeCost,
				CumulativeLatency:   time.Since(started),
				LastError:           err,
				ErrorClassification: classification,
			})
			return zero, err
		}

		if attempt >= o.MaxAttempts {
			emit(AttemptEvent{
				Kind:                "failure",
				Attempt:             attempt,
				CumulativeCostUSD:   cumulativeCost,
				CumulativeLatency:   time.Since(started),
				LastError:           err,
				ErrorClassification: classification,
			})
			return zero, &BudgetExceededError{
				Kind:     BudgetKindAttempts,
				Limit:    float64(o.MaxAttempts),
				Observed: float64(attempt),
				Attempts: attempt,
				Last:     err,
			}
		}

		emit(AttemptEvent{
			Kind:                "retry",
			Attempt:             attempt,
			CumulativeCostUSD:   cumulativeCost,
			CumulativeLatency:   time.Since(started),
			LastError:           err,
			ErrorClassification: classification,
		})

		sleepFor := backoff
		if sleepFor > o.BackoffMax {
			sleepFor = o.BackoffMax
		}
		if o.MaxWallClock > 0 {
			remaining := o.MaxWallClock - time.Since(started)
			if remaining < sleepFor {
				sleepFor = remaining
			}
		}
		if sleepFor > 0 {
			if err := o.Sleep(ctx, sleepFor); err != nil {
				return zero, err
			}
		}
		backoff = time.Duration(float64(backoff) * o.BackoffFactor)
	}
}
