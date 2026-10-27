# Remediate the Dependabot backlog in metahub-api

## Why

Dependabot alerts were disabled on this repository until 2026-10-27, at the end
of the `open-source-metahub` session. The first scan returned
**59 open alerts — 10 critical, 17 high,
28 medium, 4 low**. The criticals sit in code this
service ships: it terminates TLS, parses OCI archives, and talks to object
storage.

The count overstates the work: GitHub files one alert per advisory, so a single
module appears many times. **13
critical/high findings collapse into 10 module bumps:**

| module | severity | vulnerable | first patched |
| --- | --- | --- | --- |
| `github.com/containerd/containerd` | high | `< 1.7.29 , >= 1.7.0, < 1.7.33` | `1.7.29 , 1.7.33` |
| `github.com/containers/image/v5` | high | `< 5.29.3` | `5.29.3` |
| `github.com/distribution/distribution` | high | `<= 2.8.3` | `no fix` |
| `github.com/docker/distribution` | high | `< 2.8.2-beta.1` | `2.8.2-beta.1` |
| `github.com/docker/docker` | critical/high | `<= 28.5.2 , >= 24.0.0, < 25.0.6` | `25.0.6 , no fix` |
| `github.com/go-jose/go-jose/v4` | high | `< 4.1.4` | `4.1.4` |
| `github.com/klauspost/compress` | high | `<= 1.18.6` | `1.18.7` |
| `go.opentelemetry.io/otel/sdk` | high | `>= 1.15.0, <= 1.42.0` | `1.43.0` |
| `golang.org/x/crypto` | critical/high | `< 0.31.0 , < 0.35.0 , < 0.52.0` | `0.31.0 , 0.35.0 , 0.52.0` |
| `google.golang.org/grpc` | critical/high | `< 1.79.3 , < 1.82.1 , < 1.82.2 , <= 1.83.0` | `1.79.3 , 1.82.1 , 1.82.2 , 1.83.1` |

`golang.org/x/crypto` is the extreme case — eleven findings, one bump.

## What Changes

1. Bump each module above to at least its first patched version, closing every
   advisory in the family in one commit.
2. Where the module arrives through `metahub-core`, prefer re-pinning core over
   a local bump, so the fix is carried once. Core ships these families first; re-pin, then handle only what survives.
3. Record a dated acceptance in `CONTEXT.md` for advisories with no patched
   release (`github.com/distribution/distribution`, one of the
   `github.com/docker/docker` highs).
4. Regenerate `THIRD_PARTY_NOTICES.md` in the same change.

## Impact

`go.mod`/`go.sum`, `THIRD_PARTY_NOTICES.md`, dev + prod pipeline runs. No API or
behaviour change is intended; any bump that forces a code change is called out
in `tasks.md`.

## Provenance

Decomposed from session change `dependabot-security-alert-remediation` of the
`open-source-metahub` session (2026-10-27). The session-level proposal carries
the organisation-wide picture including the two UI repositories, which were
fixed in that session (`metahub-ui#7`, `metahub-admin-ui#15`).
