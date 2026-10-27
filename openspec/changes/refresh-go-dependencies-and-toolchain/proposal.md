# Land the Go 1.26 dependency refresh as one graph per repository

## Why

Six Dependabot pull requests were open on `metahub-core` (Go 1.26, `x/crypto`,
`x/net`, the OpenTelemetry exporter family) and the same packages were stale in
the consumers. Merging them one by one produced six graphs that disagreed with
each other, six `THIRD_PARTY_NOTICES.md` regenerations, and an intermediate
state where the builder image no longer matched the `go` directive.

## What Changes

1. The module floor (`go` directive + `toolchain`) is raised in one commit and
   matches the version the CI builder image ships.
2. The dependency graph is refreshed once per repository — not one Dependabot
   branch at a time — so `go.mod`, `go.sum` and the notices agree.
3. Static-analysis fallout from the raised floor is fixed in the same change
   (Go 1.26 tightened `go vet`'s format-string checks).
4. Consumers re-pin `metahub-core` only after the refreshed branch is final.
5. The local `go.work` tracks the module floor, so a workspace build does not
   silently use an older toolchain.

## Impact

`go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md`, the Dockerfile/CI builder image,
and doc-only edits. The policy that follows from this is captured as the
`go-toolchain-policy` capability in `specs/`.

## Provenance

Decomposed from the `open-source-metahub` session change of the same name
(2026-10-27). Status at decomposition: **implemented and merged** — this change
is the durable record of the policy, not pending work.
