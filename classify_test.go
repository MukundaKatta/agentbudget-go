package agentbudget_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MukundaKatta/agentbudget-go"
)

var (
	errRetryable = errors.New("retryable")
	errFatal     = errors.New("fatal")
)

func TestClassifyNilReturnsNone(t *testing.T) {
	if got := agentbudget.Classify(nil, nil, nil); got != agentbudget.ClassificationNone {
		t.Errorf("Classify(nil) = %q; want %q", got, agentbudget.ClassificationNone)
	}
}

func TestClassifyMatchingRetryable(t *testing.T) {
	got := agentbudget.Classify(errRetryable, agentbudget.IsAny(errRetryable), nil)
	if got != agentbudget.ClassificationRetryable {
		t.Errorf("Classify retryable = %q; want retryable", got)
	}
}

func TestClassifyFatalBeatsRetryable(t *testing.T) {
	wrapped := errors.Join(errRetryable, errFatal)
	got := agentbudget.Classify(wrapped, agentbudget.IsAny(errRetryable), agentbudget.IsAny(errFatal))
	if got != agentbudget.ClassificationFatal {
		t.Errorf("Classify with both = %q; want fatal", got)
	}
}

func TestClassifyUnknownWhenNeitherMatches(t *testing.T) {
	got := agentbudget.Classify(errors.New("x"), nil, nil)
	if got != agentbudget.ClassificationUnknown {
		t.Errorf("Classify unrecognised = %q; want unknown", got)
	}
}

func TestFingerprintIncludesTypeAndMessage(t *testing.T) {
	fp := agentbudget.Fingerprint(errors.New("boom"))
	if !strings.Contains(fp, "boom") {
		t.Errorf("fingerprint should contain message; got %q", fp)
	}
}

func TestFingerprintTruncatesLongMessages(t *testing.T) {
	fp := agentbudget.Fingerprint(errors.New(strings.Repeat("x", 5000)))
	if len(fp) >= 300 {
		t.Errorf("fingerprint not truncated; got len=%d", len(fp))
	}
}

func TestFingerprintNilReturnsConstant(t *testing.T) {
	if fp := agentbudget.Fingerprint(nil); fp != "<nil>" {
		t.Errorf("Fingerprint(nil) = %q; want <nil>", fp)
	}
}

func TestIsAnyMatchesViaErrorsIs(t *testing.T) {
	wrapped := errors.New("wrapper: " + errRetryable.Error())
	wrappedJoined := errors.Join(wrapped, errRetryable)
	pred := agentbudget.IsAny(errRetryable)
	if !pred(wrappedJoined) {
		t.Error("IsAny should match errors wrapped by errors.Join")
	}
}

func TestIsAnyEmptyReturnsNil(t *testing.T) {
	if pred := agentbudget.IsAny(); pred != nil {
		t.Error("IsAny() with no targets should return nil")
	}
}
