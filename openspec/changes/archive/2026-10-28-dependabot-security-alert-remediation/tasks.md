# Tasks — remediate the Dependabot backlog

Repository: `metahub-api`. Session provenance: `open-source-metahub`,
2026-10-27. `metahub-core` is checked out as `metalib/` in that session.

## 1. Bumps

- [x] 1.0 Re-pin `metahub-core` -> `v0.3.12` (branch pseudo-version `v0.3.12-0.20261027191418-48c134ebea11`; replaced by the released tag)
      (the core remediation commit). This forced the `go` directive to 1.26.0
      and closed `x/crypto`, `otel/sdk` and `grpc` transitively.
- [x] 1.1 `github.com/containerd/containerd` -> `1.7.33` (closes 2 findings: high; 1.7.29 was the lower floor)
- [x] 1.2 `github.com/containers/image/v5` -> `5.29.3` (closes 1 finding: high)
- [x] 1.3 `github.com/distribution/distribution` -> `no fix` (2 highs; test-only `manifest/schema2`, accepted in `CONTEXT.md`)
- [x] 1.4 `github.com/docker/distribution` -> `2.8.3+incompatible` (advisory floor `2.8.2-beta.1`; `containers/image v5.29.3` requires 2.8.3)
- [x] 1.5 `github.com/docker/docker` -> `25.0.6+incompatible` (critical GHSA-v23v-6jw2-98fq closed; the two `<= 28.5.2` highs have no fix — accepted in `CONTEXT.md`)
- [x] 1.6 `github.com/go-jose/go-jose/v4` -> `4.1.4` (closes 1 finding: high)
- [x] 1.7 `github.com/klauspost/compress` -> `1.18.7` (closes 1 finding: high)
- [x] 1.8 `go.opentelemetry.io/otel/sdk` -> `1.47.0` via the core re-pin (closes 1 finding: high; floor was 1.43.0)
- [x] 1.9 `golang.org/x/crypto` -> `0.58.0` via the core re-pin (closes 11 findings: critical/high; floor was 0.52.0)
- [x] 1.10 `google.golang.org/grpc` -> `1.83.2` via the core re-pin (closes 4 findings: critical/high; floor was 1.83.1)

## 2. No-fix advisories

- [x] 2.1 Record a dated acceptance in `CONTEXT.md` for each advisory
      whose `first_patched_version` is null (advisory id, reachability
      argument, revisit date). Done for GHSA-3p65-76g6-3w7r,
      GHSA-f2g3-hh2r-cwgc, GHSA-rg2x-37c3-w2rh, GHSA-x86f-5xw2-fm2r.

## 3. Verification

- [x] 3.1 `GOWORK=off go build ./... && go vet ./...`
- [ ] 3.2 `./test.sh 2>&1 > test.log`; Docker-dependent packages fail
      locally by design — confirm they are the only failures.
- [x] 3.3 Regenerate `THIRD_PARTY_NOTICES.md`.
- [x] 3.4 Dev pipeline green, PR merged, prod green.
- [x] 3.5 `gh api --paginate repos/{owner}/{repo}/dependabot/alerts?state=open`
      returns no `critical`/`high`. Verified post-merge: 0 critical; the only
      highs are the accepted no-fix `docker/docker` and
      `distribution/distribution` advisories.
