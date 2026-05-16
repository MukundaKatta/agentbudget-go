// Runnable example showing the four main agentbudget-go capabilities:
// retry on a sentinel error, fatal-error short-circuit, adversarial-loop
// detection, and the OnAttempt event stream.
//
// Run with:
//
//	cd examples/basic && go run .
//
// This program is also exercised by the CI build-example job, so it
// serves as a smoke test of the public API surface.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentbudget "github.com/MukundaKatta/agentbudget-go"
)

// Pretend these are the kind of errors a real LLM client returns.
var (
	errRateLimit = errors.New("rate_limit_exceeded")
	errPolicy    = errors.New("content_policy_violation")
)

// fakeLLMCall simulates an LLM call that returns rate_limit twice and
// then succeeds. Stand-in for whatever your client actually does.
func makeFakeLLMCall() func(context.Context) (string, error) {
	attempts := 0
	return func(ctx context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errRateLimit
		}
		return "the model finally responded", nil
	}
}

// alwaysFails simulates an LLM that keeps returning the same validation
// failure. agentbudget-go's adversarial-loop detection catches this.
func alwaysFails(_ context.Context) (string, error) {
	return "", errors.New("json schema validation failed: missing field 'name'")
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Demo 1: retry on rate-limit, succeed on third attempt
	fmt.Println("--- demo 1: retry then succeed ---")
	result, err := agentbudget.Run(ctx, makeFakeLLMCall(), agentbudget.Options{
		MaxAttempts:    5,
		IsRetryable:    agentbudget.IsAny(errRateLimit),
		BackoffInitial: 10 * time.Millisecond,
		OnAttempt: func(e agentbudget.AttemptEvent) {
			fmt.Printf("  [%s] attempt=%d latency=%v\n", e.Kind, e.Attempt, e.CumulativeLatency)
		},
	})
	if err != nil {
		fmt.Println("  unexpected error:", err)
	} else {
		fmt.Println("  result:", result)
	}

	// Demo 2: fatal error short-circuits before any retry
	fmt.Println("\n--- demo 2: fatal error, no retry ---")
	_, err = agentbudget.Run(ctx, func(context.Context) (string, error) {
		return "", errPolicy
	}, agentbudget.Options{
		MaxAttempts: 5,
		IsRetryable: agentbudget.IsAny(errRateLimit, errPolicy),
		IsFatal:     agentbudget.IsAny(errPolicy),
	})
	if errors.Is(err, errPolicy) {
		fmt.Println("  expected fatal error caught:", err)
	} else {
		fmt.Println("  unexpected:", err)
	}

	// Demo 3: adversarial loop (same error repeats) aborts after 3
	fmt.Println("\n--- demo 3: adversarial loop detection ---")
	_, err = agentbudget.Run(ctx, alwaysFails, agentbudget.Options{
		MaxAttempts:           20,
		IsRetryable:           func(error) bool { return true },
		DetectAdversarialLoop: true,
		AdversarialThreshold:  3,
		BackoffInitial:        time.Microsecond,
	})
	var adErr *agentbudget.AdversarialLoopDetectedError
	if errors.As(err, &adErr) {
		fmt.Printf("  caught adversarial loop after %d repeats; fingerprint=%q\n",
			adErr.Repetitions, adErr.Fingerprint)
	} else {
		fmt.Println("  unexpected:", err)
	}

	// Demo 4: budget exhausted, returns typed BudgetExceededError
	fmt.Println("\n--- demo 4: attempts budget exhausted ---")
	_, err = agentbudget.Run(ctx, func(context.Context) (string, error) {
		return "", errRateLimit
	}, agentbudget.Options{
		MaxAttempts:           3,
		IsRetryable:           agentbudget.IsAny(errRateLimit),
		DetectAdversarialLoop: false, // turn off so we exhaust attempts instead
		BackoffInitial:        time.Microsecond,
	})
	var beErr *agentbudget.BudgetExceededError
	if errors.As(err, &beErr) {
		fmt.Printf("  budget exhausted kind=%s after=%d attempts (last error: %v)\n",
			beErr.Kind, beErr.Attempts, beErr.Last)
	} else {
		fmt.Println("  unexpected:", err)
	}
}
