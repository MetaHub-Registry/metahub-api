# Design — remediating the Dependabot backlog

See the shared reasoning below; the module map for this repository is in
`proposal.md`.

## Context

Alerts were enabled on 2026-10-27, at the very end of the open-source session,
after all dependency-update PRs had been merged and every prod pipeline was
green. The first scan returned 141 open alerts in three repositories.

Three properties of that list shape the plan:

1. **It is advisory-majority, not module-majority.** GitHub files one alert per
   advisory per dependency path. `golang.org/x/crypto` holds 8 critical and 3
   high advisories; fixing the module to `>= 0.52` closes all eleven in one
   commit. Chasing alerts one by one would produce eight near-identical PRs per
   repository.
2. **Two repositories are already behind their own dependency.**
   `metahub-core` ships `x/crypto 0.58` and `otel 1.47` (bumped this session);
   `metahub-api` and `metahub-admin` still resolve older versions through their
   own `go.mod`, not through core. So the fix is partly "consume what core
   already has", which makes the core release sequencing matter.
3. **The alert count is not proportional to work or risk.** 73 of the 141 are
   medium and 9 are low, mostly in transitive test/tool paths. The plan treats
   critical/high as the bar and medium/low as a trailing cleanup.

## Root-module map

Targets are the highest patched version across the advisories touching that
module (so one bump clears every alert in the family):

| module | alert range | target | present in |
| --- | --- | --- | --- |
| `golang.org/x/crypto` | `< 0.52.0` (crit + high) | `0.58` (already in core) | api, admin |
| `google.golang.org/grpc` | `< 1.79.3`, `< 1.82.2`, `<= 1.83.0` | `>= 1.83.1` | api, admin |
| `github.com/containerd/containerd` | `< 1.7.29`, `< 1.7.33` | `>= 1.7.33` | core |
| `github.com/docker/docker` | `>= 24.0.0, < 25.0.6` (critical); `<= 28.5.2` (no fix) | `>= 25.0.6`, then assess | core |
| `github.com/distribution/distribution` | `<= 2.8.3` | **no fix** | api, admin, core |
| `github.com/docker/distribution` | `< 2.8.2-beta.1` | `2.8.2-beta.1` | core |
| `github.com/containers/image/v5` | `< 5.29.3` | `>= 5.29.3` | api, admin, core |
| `github.com/klauspost/compress` | `<= 1.18.6` | `1.18.7` | api, admin, core |
| `go.opentelemetry.io/otel/sdk` | `>= 1.15.0, <= 1.42.0` | `>= 1.43.0` | api, admin |
| `github.com/go-jose/go-jose/v4` | `< 4.1.4` | `4.1.4` | api |

Order matters: **core first**, release it, then re-pin the applications, then
deal with what is left in their own `go.mod`. Doing the applications first
means doing the same bump twice inside a dependency graph that still pulls the
old core.

## Why not automated security fixes

Dependabot's automated fixes open bot PRs on `main`. This organisation's merge
gate needs three things a bot PR does not provide:

- a **DCO sign-off** from a human (`git rebase --signoff` was needed for every
  PR this session — merge commits are unsigned and the DCO check fails);
- a refreshed **`THIRD_PARTY_NOTICES.md`**, which is generated from a
  platform-pinned toolchain and byte-compared in CI;
- a **prod pipeline run** that rebuilds and redeploys on the pinned Go builder.

Automated PRs would either sit red or be merged manually anyway, while adding
notification noise. Version-update PRs stay enabled — they are the mechanism
that produced the 24 PRs this session merged.

## Risk acceptance for the "no fix" advisories

Two families have no patched release:

- `github.com/distribution/distribution <= 2.8.3` — reached through the
  registry client stack. Decide between: (a) drop the code path if the
  conformance work makes it redundant, (b) move to a maintained fork with a
  backport, (c) accept with a reachability note.
- `github.com/docker/docker <= 28.5.2` (one high) — the daemon-side advisory is
  not reachable in a client-only usage; state that explicitly rather than
  relying on a dismissal checkbox.

The acceptance record lives in the repository's `CONTEXT.md` under a dated
heading, with the advisory ID, the reachability argument, and the date to
revisit. A dismissal in the GitHub UI without that record does not satisfy the
capability spec.

## How this is verified

Per repository, in this order:

1. `go get <module>@<target>` (or a `replace`/`// indirect` bump), `go mod tidy`.
2. `GOWORK=off go build ./...` and `go vet ./...` in the session worktree — the
   session `go.work` hides repo-local resolution problems.
3. Full test run into a log file (`./test.sh 2>&1 > test.log`); local Docker is
   unavailable, so testcontainers packages are expected to fail locally and are
   covered by the GoCD dev pipeline instead.
4. Regenerate `THIRD_PARTY_NOTICES.md` with the CI-equivalent recipe.
5. Push, let the GoCD dev pipeline go green, then prod; then
   `gh api repos/.../dependabot/alerts?state=open` returns nothing above `low`.

The last step is the acceptance criterion that matters: the alert list is the
spec.

## Open questions

- Does `metahub-api` still need `github.com/distribution/distribution` after
  the ORAS/conformance refactor, or is it a leftover path?
- Is a weekly scheduled alert check worth a workflow in each repo, or does a
  release-checklist line suffice at this cadence?
- `github.com/docker/docker` arrives through testcontainers and the image
  handling code; splitting build-only from runtime dependencies would shrink
  the surface, but only if Go module graph pruning makes it visible.
