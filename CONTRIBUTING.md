# Contributing to agentbudget-go

agentbudget-go is the Go port of [`agent-budget`](https://github.com/MukundaKatta/agent-budget) (Python) and the `withBudget` half of [`@mukundakatta/agentbudget`](https://www.npmjs.com/package/@mukundakatta/agentbudget) (JS). Contributions are welcome where they fit that scope.

## In scope

- Bug fixes in `Run[T]`, `BudgetExceededError`, `AdversarialLoopDetectedError`, `AttemptEvent`, `Classify`, `Fingerprint`, `IsAny`.
- Stronger adversarial-loop detection (alternative fingerprint strategies) behind option fields.
- Additional cost-cap modalities (sliding-window, per-tenant) as additive fields on `Options`.
- Test coverage improvements (current target: 95%+ line coverage).
- Go-idiom improvements that preserve the API.

## Out of scope

- **Generic retry library.** `cenkalti/backoff`, `avast/retry-go`, `hashicorp/go-retryablehttp` cover that. agentbudget-go exists for LLM-specific failure modes those don't address.
- **Rate limiting.** Use `golang.org/x/time/rate`.
- **Routing / fallback across providers.** That's a separate concern.
- **Tracing backends.** agentbudget-go emits typed `AttemptEvent`s; you wire them to your tracer (OTel, Datadog, etc.).
- **Third-party dependencies.** The zero-dep promise is enforced by CI; do not introduce a `go.mod require` entry without a strong justification reviewed by the maintainer.

## Sibling libraries

agentbudget-go has language siblings; aim for API parity in spirit (not byte-identical signatures):

- Python: [`agent-budget`](https://github.com/MukundaKatta/agent-budget) — `@budget` decorator
- JavaScript: [`@mukundakatta/agentbudget`](https://www.npmjs.com/package/@mukundakatta/agentbudget) — `withBudget(fn, opts)`

If you add a new feature here that also makes sense in the Python or JS lib, mention it in the PR so the siblings can stay aligned.

## Development setup

```bash
git clone https://github.com/MukundaKatta/agentbudget-go.git
cd agentbudget-go
go test -race -cover ./...                   # 22 tests
go test -race -coverprofile=cover.out ./...
go tool cover -func=cover.out                # per-function coverage
go vet ./...
```

Go 1.21+ required. **Zero third-party dependencies.**

If you have `golangci-lint` installed:

```bash
golangci-lint run
```

## Workflow

1. Open an issue first for anything bigger than a one-file change.
2. Branch from `main`.
3. Write tests covering the change. Use the `fakeSleep` pattern from `budget_test.go` so tests don't depend on real time.
4. Run `go test -race -cover ./...` and confirm full suite passes.
5. Run `go vet ./...` and `golangci-lint run` if installed.
6. Open a PR against `main`. Fill in the template.
7. CI must be green before review.

## Coding conventions

- Public types and functions get godoc comments. Examples (`ExampleXxx` in `_test.go`) preferred for non-trivial APIs.
- Errors are pointer types implementing `Error() string` and `Unwrap() error` where applicable.
- `Run[T]` honors `context.Context` cancellation in both function dispatch and inter-attempt sleep.
- `OnAttempt` hooks that panic are recovered; instrumentation never crashes the wrapped call.
- New options on `Options` are additive (zero-value preserves existing behavior).

## Release cadence

Releases follow semver. Patches: bug fixes only. Minor versions: new public symbols. Major versions: breaking changes (unlikely in v0.x).

Releases are cut by the maintainer via tag push. pkg.go.dev auto-indexes within minutes of the tag being visible on GitHub. See `.github/workflows/release.yml` for the GitHub Release flow.
