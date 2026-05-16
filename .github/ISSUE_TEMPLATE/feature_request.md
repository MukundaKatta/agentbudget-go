---
name: Feature request
about: Suggest a new capability or improvement for agentbudget-go
title: ''
labels: enhancement
assignees: ''
---

## Problem statement

What gap does this close? What is the current pain (concrete: an error you see, a workaround you wrote, a metric you can't get)?

## Proposed surface

A sketch of the API or behavior. Code preferred over prose:

```go
// What you imagine calling
```

## Why this belongs in agentbudget-go (not cenkalti/backoff or avast/retry-go)

agentbudget-go exists for LLM-specific retry/budget failure modes that generic retry libs don't address (see `CONTRIBUTING.md`). Explain why this should live here rather than in your application code on top of a generic retry library.

## Cross-language scope

agentbudget-go has Python (`agent-budget`) and JS (`@mukundakatta/agentbudget`) siblings.

- [ ] This feature is Go-specific (e.g. uses context, generics, channels).
- [ ] This feature should also exist in the Python/JS siblings.
- [ ] Unsure — discuss in this issue.

## Alternatives considered

What else have you tried or considered? (Custom retry loop? Generic retry lib + manual cost tracking? Doing nothing?)

## Willing to contribute?

- [ ] I can open a PR for this
- [ ] I can help test it
- [ ] Looking for someone else to build it
