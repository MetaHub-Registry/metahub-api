# Tasks — remediate the Dependabot backlog

Repository: `metahub-api`. Session provenance: `open-source-metahub`,
2026-10-27. `metahub-core` is checked out as `metalib/` in that session.

## 1. Bumps

- [ ] 1.1 `github.com/containerd/containerd` -> `1.7.29 , 1.7.33` (closes 1 finding(s): high)
- [ ] 1.2 `github.com/containers/image/v5` -> `5.29.3` (closes 1 finding(s): high)
- [ ] 1.3 `github.com/distribution/distribution` -> `no fix` (closes 1 finding(s): high)
- [ ] 1.4 `github.com/docker/distribution` -> `2.8.2-beta.1` (closes 1 finding(s): high)
- [ ] 1.5 `github.com/docker/docker` -> `25.0.6 , no fix` (closes 2 finding(s): critical, high)
- [ ] 1.6 `github.com/go-jose/go-jose/v4` -> `4.1.4` (closes 1 finding(s): high)
- [ ] 1.7 `github.com/klauspost/compress` -> `1.18.7` (closes 1 finding(s): high)
- [ ] 1.8 `go.opentelemetry.io/otel/sdk` -> `1.43.0` (closes 1 finding(s): high)
- [ ] 1.9 `golang.org/x/crypto` -> `0.31.0 , 0.35.0 , 0.52.0` (closes 2 finding(s): critical, high)
- [ ] 1.10 `google.golang.org/grpc` -> `1.79.3 , 1.82.1 , 1.82.2 , 1.83.1` (closes 2 finding(s): critical, high)

## 2. No-fix advisories

- [ ] 2.1 Record a dated acceptance in `CONTEXT.md` for each advisory
      whose `first_patched_version` is null (advisory id, reachability
      argument, revisit date).

## 3. Verification

- [ ] 3.1 `GOWORK=off go build ./... && go vet ./...`
- [ ] 3.2 `./test.sh 2>&1 > test.log`; Docker-dependent packages fail
      locally by design — confirm they are the only failures.
- [ ] 3.3 Regenerate `THIRD_PARTY_NOTICES.md`.
- [ ] 3.4 Dev pipeline green, PR merged, prod green.
- [ ] 3.5 `gh api --paginate repos/{owner}/{repo}/dependabot/alerts?state=open`
      returns no `critical`/`high`.
