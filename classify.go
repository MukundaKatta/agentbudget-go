package agentbudget

import (
	"errors"
	"fmt"
	"reflect"
)

// Classification is the human-readable category an error gets, used in
// AttemptEvent.ErrorClassification.
type Classification string

// The set of classifications Classify can assign to an error.
const (
	ClassificationRetryable Classification = "retryable"
	ClassificationFatal     Classification = "fatal"
	ClassificationUnknown   Classification = "unknown"
	ClassificationNone      Classification = "none"
)

// Classify an error against caller-supplied predicates.
//
// Precedence: isFatal beats isRetryable. An error matching neither is
// classified Unknown and Run will re-raise it — never blindly retrying.
//
// Either predicate may be nil; nil predicates are treated as "no match".
func Classify(err error, isRetryable, isFatal func(error) bool) Classification {
	if err == nil {
		return ClassificationNone
	}
	if isFatal != nil && isFatal(err) {
		return ClassificationFatal
	}
	if isRetryable != nil && isRetryable(err) {
		return ClassificationRetryable
	}
	return ClassificationUnknown
}

// Fingerprint produces a short stable fingerprint for adversarial-loop
// detection. Combines the error's runtime type with the first 200 chars
// of its Error() string.
//
// Two retries with the same fingerprint indicate the LLM is producing the
// same failure repeatedly — the retry-amplification pattern flagged by
// Instructor #2056.
func Fingerprint(err error) string {
	if err == nil {
		return "<nil>"
	}
	t := reflect.TypeOf(err)
	name := "error"
	if t != nil {
		// For pointer types, surface the underlying name so two pointers to the
		// same type fingerprint identically.
		if t.Kind() == reflect.Ptr {
			name = t.Elem().String()
		} else {
			name = t.String()
		}
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return fmt.Sprintf("%s:%s", name, msg)
}

// IsAny is a convenience predicate builder — returns a func(error) bool
// matching any of the given sentinel errors via errors.Is.
//
// Example:
//
//	opts.IsRetryable = agentbudget.IsAny(ErrThrottled, ErrTimeout)
func IsAny(targets ...error) func(error) bool {
	if len(targets) == 0 {
		return nil
	}
	return func(err error) bool {
		for _, t := range targets {
			if errors.Is(err, t) {
				return true
			}
		}
		return false
	}
}
