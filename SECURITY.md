# Security Policy

## Supported Versions

agentbudget-go is at v0.1.x. Security fixes will be issued for the current minor (0.1.x). Older minors will not receive backports.

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅        |

## Reporting a Vulnerability

Please **do not** open a public issue for security vulnerabilities.

Report privately by emailing `mukunda.vjcs6@gmail.com` with the subject `[agentbudget-go security]`. Include:

- A description of the vulnerability and its impact.
- The version of agentbudget-go affected (your `go.sum` entry).
- Reproduction steps or a minimal proof-of-concept.
- Any suggested mitigation, if you have one.

You can expect:

- An acknowledgment within 5 business days.
- A status update within 14 days.
- A coordinated disclosure window of at most 90 days from the acknowledgment.

## Specific Risk Surfaces

agentbudget-go is the Go port of the Python `agent-budget` retry/budget primitive. Areas worth special attention:

- **`Run[T]` retry loop** — agentbudget-go exists to prevent the [Instructor #2056](https://github.com/jxnl/instructor/issues/2056) class of retry-amplification attack. If you find a way for an adversarial error payload to bypass `AdversarialLoopDetectedError` and drive unbounded retries, that's a high-severity report.
- **`Fingerprint(err)`** — fingerprinting is `reflect.TypeOf(err).Elem().String()` + first 200 chars of `err.Error()`. If two semantically-identical failures can fingerprint differently (defeating adversarial detection), that's a bug worth reporting. Likewise if two distinct error types collide on the same fingerprint string.
- **Context cancellation** — `Run[T]` honors `ctx.Done()` in both the function dispatch and the inter-attempt sleep. If you find a path where a cancelled context still triggers more attempts or hangs, that's a real issue.
- **`CostExtractor` overflow** — cumulative cost is a `float64`. If a malicious `CostExtractor` can return NaN or `+Inf` to evade `MaxCostUSD`, please report.

## Dependencies

agentbudget-go has **zero third-party Go dependencies**. The entire surface is `context`, `errors`, `fmt`, `reflect`, and `time` from the standard library. This is enforced in CI; see `.github/workflows/test.yml`.

We will not pay bug bounties at this time.
