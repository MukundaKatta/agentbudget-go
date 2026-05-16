---
name: Bug report
about: Report a defect in agentbudget-go
title: ''
labels: bug
assignees: ''
---

## What happened

A clear, one-paragraph description of the bug.

## What I expected to happen

What you thought would happen.

## Repro

A minimal Go program that triggers the issue. For retry/budget/adversarial-detection bugs, please include the error type(s) involved.

```go
package main

import (
    "context"
    "fmt"

    agentbudget "github.com/MukundaKatta/agentbudget-go"
)

func main() {
    // ...
}
```

## Environment

- `agentbudget-go` version: (from `go.sum`)
- Go version: (output of `go version`)
- Operating system + arch:
- LLM client library involved (if any): (e.g. `github.com/anthropics/anthropic-sdk-go v0.x`)

## Stack trace

If applicable, paste the full panic/error output.

```
```

## What did the AttemptEvent stream look like?

If you registered an `OnAttempt` hook, paste the events emitted (with any PII redacted).

```
```

## Anything else

Related links, logs, or context.
