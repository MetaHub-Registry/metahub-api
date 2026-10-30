# Design — public CI for metahub-api

## Stack and boundary

Go 1.26.0 declared in `go.mod` (CI pins `1.26.x`, which resolves to the latest
patch). The old `go 1.24.0` floor was raised by the dependency refresh.

Fast checks:

- `go build ./...`
- `go vet ./...`
- `golangci-lint` with the committed `.golangci.yml`
- `go test -short ./...` for container-free packages
- `gitleaks` with the committed `.gitleaks.toml`
- DCO trailer check
- `go-licenses` allowlist and generated `THIRD_PARTY_NOTICES.md`
- `govulncheck` (advisory while the dependency baseline is cleared)
- outbound-host allowlist

Container-backed checks: `pkg/api/v1/handlers` starts Neo4j, MinIO and Jaeger in `TestMain`; it exits early under `-short`.

The handlers tests remain in the integration lane, which runs the full suite on release tags and on demand.

## Decisions

### Self-contained, fork-safe workflows

The workflows live in this repository. A cross-repository reusable workflow
would need either a public `.github` repository before the launch gate or
private access that a fork does not receive. Every PR-triggered workflow is
therefore self-contained, declares `permissions: contents: read`, pins
third-party actions by SHA and references no secret.

### Transitional state before the visibility flip

`metahub-core` is still private, so the module-dependent fast jobs
(`build`, `vet`, `lint`, `test`, `licenses`, `vuln`) are guarded by
`github.event.pull_request.head.repo.full_name == github.repository` and
configure Git access with the read-only `METAHUB_CORE_RO_TOKEN`. Fork PRs skip
those jobs; only `gitleaks`, `dco`, `telemetry` and the `fast` aggregator run.

This is a deliberate, temporary violation of the fork-safety requirement. It
must be removed in the same change as the visibility flip (core public first,
then API/admin): drop `GOPRIVATE`, drop the token step and the job-level `if`
guards, and confirm the fork run. The `fast` aggregator currently treats a
skipped job as a pass, so removing the guards is a prerequisite for a
meaningful fork check. Tracked as tasks 3.2–3.4.

### One aggregator

`fast` depends on every fast job and fails when any of them fails or is
cancelled. Marking only `fast` as a required check at the visibility flip keeps
the branch-protection configuration stable as checks are added.

### Container-backed checks gate releases

Tests that need Docker run in `integration.yml` on release tags and
`workflow_dispatch`, with no secrets. They never run on a pull request.

### Dependencies and licences

The licence check fails on a dependency outside the allowlist and the notices
file is regenerated in CI; a drift fails the job. `govulncheck` is advisory
(`continue-on-error: true`) until the remaining baseline is cleared. The
2026-10-30 scan reported 49 reachable findings: 37 in the Go standard library
(fixed by `go1.26.9`/`go1.26.10`, which `1.26.x` picks up in CI) and 12 through
modules — `containerd v1.7.33` (7), `docker v25.0.6` (3) and `regclient v0.4.5`
(2). Six of the module findings have no patched version. The job still runs on
every pull request.

### Outbound hosts

Non-test source may reference only hosts on `.github/outbound-allowlist.txt`.
A new host fails the job and must be added deliberately in review. This is a
review gate, not a network sandbox.

## Risks

| Risk | Mitigation |
|---|---|
| A workflow needs a secret when run from a fork | it belongs in the integration lane or is dropped |
| The temporary `METAHUB_CORE_RO_TOKEN` carve-out outlives the visibility flip | tasks 3.2–3.4 gate the flip on removing it and proving a fork run |
| Action supply chain | every action is pinned to a full commit SHA |
| Dependency advisories | Dependabot opens weekly PRs; the Go vulnerability job is advisory until the current baseline is cleared |
