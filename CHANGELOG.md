# Changelog

All notable changes to `agentbudget-go` are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `BudgetExceededError` from the **wall-clock** path now carries the most recent
  failure cause in `Last`, so `errors.Is` / `errors.As` traverse through to it
  just like the attempts path already did. Previously the wall-clock budget
  error had a `nil` `Last`, making error-chain inspection inconsistent.
- CI: the "verify zero third-party dependencies" step had broken shell logic
  (`grep -q ... | grep -v ...` operates on empty input, so the guard never
  actually fired). Rewrote it to inspect `go.mod` requirements correctly and to
  use `go list -deps` so the stdlib-only guarantee is genuinely enforced.

### Added

- Tests for the `BudgetExceededError` / `AdversarialLoopDetectedError` message
  formats and `Unwrap()` chains, for context cancellation during the *default*
  backoff sleep, and for the wall-clock budget error carrying its last cause.

## [0.1.0] — 2026-05-09

Initial release. Go port of the Python sibling at [MukundaKatta/agent-budget](https://github.com/MukundaKatta/agent-budget). Closes the same three-thing core: cost cap, structured per-attempt events, and adversarial-loop detection.

### Added

- `Run[T any](ctx, fn, opts) (T, error)` — generic retry + budget wrapper.
- `Options` struct: `MaxAttempts`, `MaxCostUSD`, `MaxWallClock`, `IsRetryable`, `IsFatal`, `CostExtractor`, `DetectAdversarialLoop`, `AdversarialThreshold`, `BackoffInitial`, `BackoffMax`, `BackoffFactor`, `OnAttempt`, `Sleep`.
- `AttemptEvent` struct: structured event taxonomy emitted to `OnAttempt` (`start`, `retry`, `success`, `failure`).
- `BudgetExceededError` and `AdversarialLoopDetectedError` with `Unwrap()` for `errors.Is` traversal.
- `Classify(err, isRetryable, isFatal) Classification` utility.
- `Fingerprint(err) string` — `reflect.TypeOf(err).Elem().String()` + first 200 chars of message; stable across runs.
- `IsAny(targets...) func(error) bool` — convenience predicate builder using `errors.Is`.

### Notes

- 22 unit tests, ~88% line coverage.
- Zero third-party dependencies; standard library only.
- Honors `context.Context` cancellation in both function dispatch and inter-attempt sleep.
- `OnAttempt` hook panics are recovered so instrumentation bugs don't crash the wrapped call.
- Closes [Instructor #2056](https://github.com/jxnl/instructor/issues/2056) (retry-amplification class of bug) and [Instructor #2222](https://github.com/jxnl/instructor/issues/2222) (attempt-metadata gap) for Go users.
