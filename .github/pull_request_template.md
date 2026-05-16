## Summary

What does this PR change in 1-3 sentences?

## Linked issue

Closes #

## Scope check

- [ ] Confirmed this fits `CONTRIBUTING.md` scope (LLM-specific retry/budget; not generic retry, not rate limiting, not routing).
- [ ] Public API changes (if any) are documented in the README and CHANGELOG.
- [ ] **No new Go dependencies** added to `go.mod`. agentbudget-go is standard-library-only by design.
- [ ] `OnAttempt` hooks that the user can register are panic-recovered so user errors don't break the wrapped call.
- [ ] `Run[T]` still honors `context.Context` cancellation in both function dispatch and inter-attempt sleep.

## Tests

- [ ] Added or updated tests covering the behavior change.
- [ ] `go test -race -cover ./...` passes locally.
- [ ] Coverage at or above 95%.
- [ ] Tests use the `fakeSleep` pattern from `budget_test.go` (no real-time sleeps).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean (if installed locally).

## Sibling-library impact

agentbudget-go has Python (`agent-budget`) and JS (`@mukundakatta/agentbudget`) siblings.

- [ ] This change is Go-specific and doesn't apply to siblings.
- [ ] Applies to siblings; tracked at: <sibling issue/PR link>
- [ ] Unsure — leaving for maintainer to assess.

## Risk and impact

Anything reviewers should look at extra carefully (generics, error unwrapping, fingerprint stability, context handling, etc.)?

## Notes for the reviewer

Anything off-checklist worth surfacing.
