# agentbudget-go

Production retry/budget primitive for LLM and agent calls. Go port of [`agent-budget`](https://github.com/MukundaKatta/agent-budget) (Python) and the `withBudget` half of [`@mukundakatta/agentbudget`](https://www.npmjs.com/package/@mukundakatta/agentbudget) (JavaScript).

```bash
go get github.com/MukundaKatta/agentbudget-go
```

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    agentbudget "github.com/MukundaKatta/agentbudget-go"
)

var (
    errRateLimit  = errors.New("rate limited")
    errPolicy     = errors.New("policy violation")
)

func callLLM(ctx context.Context) (Response, error) { /* ... */ }

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    resp, err := agentbudget.Run(ctx, callLLM, agentbudget.Options{
        MaxAttempts:           5,
        MaxCostUSD:            0.10,
        MaxWallClock:          30 * time.Second,
        IsRetryable:           agentbudget.IsAny(errRateLimit),
        IsFatal:               agentbudget.IsAny(errPolicy),
        CostExtractor:         func(r any) float64 { return r.(Response).CostUSD },
        DetectAdversarialLoop: true,
    })

    var be *agentbudget.BudgetExceededError
    var ad *agentbudget.AdversarialLoopDetectedError
    switch {
    case errors.As(err, &ad):
        log.Printf("validation always failing — fingerprint=%q", ad.Fingerprint)
    case errors.As(err, &be):
        log.Printf("%s budget exhausted after %d attempts", be.Kind, be.Attempts)
    case err != nil:
        log.Fatalf("call failed: %v", err)
    }
    use(resp)
}
```

## Why exist when `cenkalti/backoff`, `avast/retry-go`, and `lestrrat-go/backoff` are right there

Generic retry libs are good at "retry this thing N times with exponential backoff." They don't know:

- **LLM cost.** None of them have a `MaxCostUSD` concept; you'd have to implement it on top.
- **Adversarial-loop detection.** A prompt-injected response that always fails JSON validation drives unbounded retries. Generic retry libs happily comply, billing the provider every loop. `agentbudget-go` fingerprints the error and aborts after `AdversarialThreshold` consecutive identical failures.
- **Per-attempt event taxonomy.** `start` / `retry` / `success` / `failure` events with attempt #, cumulative cost, cumulative latency, last error, and a `retryable` / `fatal` / `unknown` classification. Lets you wire structured logging without re-parsing error strings.

Background: this is the Go port of [`agent-budget` (Python)](https://github.com/MukundaKatta/agent-budget). The motivating issues are documented there: [Instructor #2056](https://github.com/jxnl/instructor/issues/2056) (retry amplification security audit) and [Instructor #2222](https://github.com/jxnl/instructor/issues/2222) (attempt-metadata gap).

## API

```go
func Run[T any](
    ctx context.Context,
    fn func(context.Context) (T, error),
    opts Options,
) (T, error)
```

A single generic function. Pass your callable, your options, get the typed result or one of:

- `*BudgetExceededError` — attempts / wall-clock / cost cap exhausted. `Unwrap()` returns the last cause for `errors.Is` traversal.
- `*AdversarialLoopDetectedError` — same fingerprint repeated `AdversarialThreshold` times.
- `context.Canceled` / `context.DeadlineExceeded` — propagated from `ctx`.
- The original error from `fn` — when classified `Fatal` or `Unknown`.

### `Options`

| Field | Default | Meaning |
|---|---|---|
| `MaxAttempts` | `5` | Hard cap on calls to `fn`. |
| `MaxCostUSD` | `0` (= no cap) | Cumulative-cost cap across successful sub-calls. Requires `CostExtractor`. |
| `MaxWallClock` | `0` (= no cap) | Cumulative time across attempts and backoffs. |
| `IsRetryable` | `nil` | `func(error) bool`. Use [`IsAny`](#isany) to build from sentinel errors. |
| `IsFatal` | `nil` | Same shape; beats `IsRetryable`. |
| `CostExtractor` | `nil` | `func(result any) float64`. |
| `DetectAdversarialLoop` | `false` | Opt in to fingerprint-based loop detection. |
| `AdversarialThreshold` | `3` | Consecutive identical-fingerprint failures before abort. |
| `BackoffInitial` | `500ms` | First inter-attempt sleep. |
| `BackoffMax` | `30s` | Cap on grown backoff. |
| `BackoffFactor` | `2.0` | Multiplier per retry. |
| `OnAttempt` | `nil` | `func(AttemptEvent)`. Hook errors are recovered. |
| `Sleep` | context-aware sleep | Override for tests; signature `func(ctx, duration) error`. |

### `IsAny`

Convenience predicate builder: `IsAny(errA, errB, ...)` returns a function matching any of the given sentinel errors via `errors.Is` (so wrapped errors traverse correctly).

### `AttemptEvent`

```go
type AttemptEvent struct {
    Kind                string         // "start" | "retry" | "success" | "failure"
    Attempt             int            // 1-indexed; 0 only for "start"
    CumulativeCostUSD   float64
    CumulativeLatency   time.Duration
    LastError           error
    ErrorClassification Classification // "retryable" | "fatal" | "unknown" | "none"
}
```

Hook example:

```go
opts.OnAttempt = func(e agentbudget.AttemptEvent) {
    metrics.HistogramVec("llm.attempt", float64(e.Attempt),
        "kind", e.Kind, "classification", string(e.ErrorClassification))
    if e.Kind == "retry" {
        slog.Warn("retry",
            "attempt", e.Attempt,
            "cumulative_cost_usd", e.CumulativeCostUSD,
            "cumulative_latency", e.CumulativeLatency,
            "last_error", e.LastError,
        )
    }
}
```

Hooks that panic are caught — instrumentation bugs never break the wrapped call.

## What it explicitly does NOT do

- Not a rate limiter. Use `golang.org/x/time/rate` for that.
- Not a router or fallback library — doesn't pick another model.
- Not a tracer. Emits structured events; you wire them to your tracer.
- Not provider-specific. Bring your own LLM client.
- No third-party dependencies. Standard library only.

## Sibling libraries

| Language | Package | Status |
|---|---|---|
| Python | [`agent-budget`](https://github.com/MukundaKatta/agent-budget) | v0.1.0 |
| JavaScript / TypeScript | [`@mukundakatta/agentbudget`](https://www.npmjs.com/package/@mukundakatta/agentbudget) | v0.1 (`Budget` accumulator); v0.2 PR open with `withBudget` |
| **Go** | **`github.com/MukundaKatta/agentbudget-go` (this lib)** | **v0.1.0** |

## License

Apache-2.0. See [LICENSE](./LICENSE).

## Repository Health

This repository includes a dependency-free health check for core documentation, metadata, and CI wiring. Run it locally before publishing changes:

```sh
python3 scripts/check_repository_health.py
```

The same check runs in GitHub Actions on pushes and pull requests.
