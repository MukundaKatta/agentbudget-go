package agentbudget_test

import (
	"errors"
	"strings"
	"testing"

	agentbudget "github.com/MukundaKatta/agentbudget-go"
)

// The README documents that BudgetExceededError.Unwrap() returns the last
// cause so errors.Is/errors.As traverse through to it. Exercise the contract
// directly on the error type (not just via Run).
func TestBudgetExceededErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying boom")
	be := &agentbudget.BudgetExceededError{
		Kind:     agentbudget.BudgetKindAttempts,
		Limit:    5,
		Observed: 5,
		Attempts: 5,
		Last:     cause,
	}
	if !errors.Is(be, cause) {
		t.Error("errors.Is(BudgetExceededError, cause) should be true via Unwrap")
	}
	if got := be.Unwrap(); !errors.Is(got, cause) {
		t.Errorf("Unwrap() = %v; want %v", got, cause)
	}
}

func TestBudgetExceededErrorUnwrapNilLastIsSafe(t *testing.T) {
	// A budget error with no recorded cause must not panic on traversal and
	// must report no match for an unrelated target.
	be := &agentbudget.BudgetExceededError{Kind: agentbudget.BudgetKindWallClock}
	if be.Unwrap() != nil {
		t.Errorf("Unwrap() with no Last should be nil; got %v", be.Unwrap())
	}
	if errors.Is(be, errors.New("unrelated")) {
		t.Error("errors.Is should not match an unrelated error")
	}
}

func TestBudgetExceededErrorMessageMentionsKindAndAttempts(t *testing.T) {
	be := &agentbudget.BudgetExceededError{
		Kind:     agentbudget.BudgetKindCostUSD,
		Limit:    0.10,
		Observed: 0.25,
		Attempts: 4,
	}
	msg := be.Error()
	for _, want := range []string{"costUSD", "budget exceeded", "4 attempt"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q; want substring %q", msg, want)
		}
	}
}

func TestAdversarialLoopErrorUnwrap(t *testing.T) {
	cause := errors.New("json validation failed")
	ad := &agentbudget.AdversarialLoopDetectedError{
		Repetitions: 3,
		Fingerprint: "errorString:json validation failed",
		Last:        cause,
	}
	if !errors.Is(ad, cause) {
		t.Error("errors.Is(AdversarialLoopDetectedError, cause) should be true via Unwrap")
	}
}

func TestAdversarialLoopErrorMessageIncludesFingerprint(t *testing.T) {
	ad := &agentbudget.AdversarialLoopDetectedError{
		Repetitions: 3,
		Fingerprint: "myType:always the same",
		Last:        errors.New("x"),
	}
	msg := ad.Error()
	if !strings.Contains(msg, "adversarial loop") {
		t.Errorf("Error() = %q; want it to mention an adversarial loop", msg)
	}
	if !strings.Contains(msg, "always the same") {
		t.Errorf("Error() = %q; want it to include the fingerprint", msg)
	}
	if !strings.Contains(msg, "3") {
		t.Errorf("Error() = %q; want it to include the repetition count", msg)
	}
}
