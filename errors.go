// Package agentbudget — production retry/budget primitive for LLM and agent calls.
//
// Go port of the Python sibling at https://github.com/MukundaKatta/agent-budget.
// Same three-thing core: cost cap, structured per-attempt events, and
// adversarial-loop detection.
package agentbudget

import "fmt"

// BudgetKind names which budget tripped in BudgetExceededError.
type BudgetKind string

const (
	BudgetKindAttempts  BudgetKind = "attempts"
	BudgetKindWallClock BudgetKind = "wallClock"
	BudgetKindCostUSD   BudgetKind = "costUSD"
)

// BudgetExceededError is returned by Run when a configured budget is exhausted
// before the call could succeed. Wraps the last underlying error so
// errors.Is and errors.As walk through to it.
type BudgetExceededError struct {
	Kind     BudgetKind
	Limit    float64
	Observed float64
	Attempts int
	Last     error
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf(
		"agentbudget: %s budget exceeded — observed %v > limit %v after %d attempt(s)",
		e.Kind, e.Observed, e.Limit, e.Attempts,
	)
}

// Unwrap returns the underlying error so errors.Is(err, target) works for
// the original cause as well as for the BudgetExceededError itself.
func (e *BudgetExceededError) Unwrap() error { return e.Last }

// AdversarialLoopDetectedError is returned by Run when the same exception
// fingerprint repeats AdversarialThreshold times in a row. Catches the
// retry-amplification class of bug from Instructor #2056: a prompt-injected
// or malformed LLM response that always fails validation, silently burning
// budget.
type AdversarialLoopDetectedError struct {
	Repetitions int
	Fingerprint string
	Last        error
}

func (e *AdversarialLoopDetectedError) Error() string {
	return fmt.Sprintf(
		"agentbudget: adversarial loop detected — %d consecutive identical failures (fingerprint=%q)",
		e.Repetitions, e.Fingerprint,
	)
}

func (e *AdversarialLoopDetectedError) Unwrap() error { return e.Last }
