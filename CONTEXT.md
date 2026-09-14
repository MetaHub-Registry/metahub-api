# Context: metahub-api

> This file helps AI agents understand the repo quickly.

## Session: readd-pull-through-cache — repo4 routes + Option-A delivery policy (2026-09-14)

Branch `116-readd-pull-through-cache`, commits `7020083`…`3241270`.
Sonar gate OK (new coverage 81.3%, 0 new violations).

### 1. repo4 router routes (5-segment /v2 paths)

Deep upstream repos pulled through the cache
(`/v2/harbor.example.com/<proj>/<sub>/<sub>/<image>/manifests/<tag>`) need 5 path
segments, but every `/v2/` route family stopped at 4 (`repo3`) → chi 404 with
the unhelpful body `404 page not found`. Fix: `repo4ImgManifestRefPath`,
`repo4ImgBlobRefPath` and `Repo4ImgBlobUploadUuidPath` registered for
manifests GET/HEAD/PUT, blobs GET/HEAD, blob uploads POST/PATCH/PUT/GET, tags
and referrers (`pkg/api/v1/router/router.go`). metalib's `extractRepoName`
joins `repo4` accordingly.

### 2. Option-A pull-through delivery policy (`upstream-credential-proxy`)

Two holes closed in the blob delivery path:

- **`tryBlobGetRedirect`** (`pkg/api/v1/handlers/oci_blob.go`): the upstream
  `307` was gated on `AllowsUnauthorizedPull` (= `AllowAnon`) only — a
  credentialed registry with `allow_anon: true` (harbor!) would redirect
  clients to private upstreams they cannot authenticate against. New
  `upstreamRedirectAllowed(lyrUp)` helper: pull-through registries are gated
  on `client_redirect` alone (`false` = proxy default, `true` = explicit
  opt-in where the client holds its own upstream creds); non-pull-through
  registries keep the legacy AllowAnon gate.
- **`PolicyFor.Enabled`** (`pkg/pullthrough/magazine.go`): was
  `AllowAnon && Enabled`, silently excluding `allow_anon:false` +
  credentials registries (registry.gitlab.com!) from blob-miss proxying and
  cache enqueue. Now uses metalib's `RegistryConfig.PullThroughEligible()` —
  the single source of truth.

Semantics (session addendum D-cred1): proxy is the default for credentialed
registries; `client_redirect: true` is the operator's statement that clients
hold their own upstream credentials (their login answers the redirect
challenge). MetaHub never injects its own credentials into a redirect
(`Authorization` stripped, `Location` from registry name + digest only).

### 3. Live-verified behavior

GoCD `pull-through-test_MR116_dev` scenario (verified locally via
`run-pull-through.sh`, 4/4 each): anonymous
(`harbor.example.com/library/alpine:3.24`), private harbor
(`harbor.example.com/metahub/api/dev/metahub-api_mr116_dev:14`), private GitLab
(`registry.github.com/metahub-registry/metahub-api:prod.2026-03-16-1`) — manifest
resolution, Neo4j graph ingest, layer delivery, warm pull.

### Gotchas

- The `upstream` import in handlers is metalib's `pkg/upstream` (`UpstreamTriple`);
  the `:Upstream` graph node type is `types.Upstream` (metalib `pkg/types`).
  `UpstreamGetByLayerDigest` returns the latter.
- `pullthrough.Cache.Policy()` is nil-safe (`c == nil` guard) — calling it on a
  zero `Handlers` is fine.
- OpenSpec: `add-upstream-registry-credentials` (5/10) carries the remaining
  live-e2e + graph tasks; spec deltas amended to Option A.

## Session: waldur-outfitter-improvements — fix-digest-manifest-existence handler test (2026-09-08)

**Change:** `fix-digest-manifest-existence`, commit `99620ed`.

Handler-level regression test (`oci_manifest_digest_unlinked_test.go`,
`TestManifestDigest_UnlinkedNeverFiveHundred`) asserts that `HEAD` or `GET`
by digest of an **unlinked** manifest (pushed but no profile REF edge) returns
**200 + `Docker-Content-Digest`**, never 5xx. The fix is entirely metalib-side
(`ManifestFromResult` returns `ok=false, nil` for unlinked manifests); the
handler's `err→500` path remains for genuine DB errors. The test exercises
Path Z (`resolveMissingManifest` → `handleManifest` → `ManifestGetByDigest`).

**Sonar gate:** 0 new violations, 0.0% duplication — passed in GoCD
`metahub-api_MR115_dev` run 20.

## Session: waldur-outfitter-integration — gate-model-serving-on-model-centric (2026-08-24)

No code changes in this repo — the existing handlers already pass/set context
correctly for the new gate:

- `resolveModelCentricRuntime` (`oci_manifest.go:394-403`) sets
  `ctx = types.WithRequestedQuant(ctx, quant)` + `ctx = types.WithRequestedModel(ctx, modelName)`.
- `manipulateSingleIndex` (line 216) passes `ctx` through to
  `ApplyManipulationsToManifests`.
- `manipulateIndexInPlace` (`oci_index.go:75`) also passes `ctx` through.

The gate in metalib (`applyManipulationsToCandidate`) now checks
`RequestedModelFromContext(ctx) != ""`, so these paths are sufficient — collection
pulls are served un-injected, model-centric pulls compose runtime + weights +
config. No metahub-api code change needed.

The `836a20e` empty commit on this branch is a CI retrigger (TLS timeout, pre-dind-MTU-fix); it carries no code change.

## model-centric-pull (session isc-session-v2, 2026-07-07) — branch `109-isc-session-v2`

**Done:**
- `a0d0a1f` — `pickBlobLayerDigest` (pure, 100%) + guard tests for `ManifestBlobContentGet`.
- `1e8d20d` — **#8 serve interception SHIPPED**: `resolveModelCentricRuntime` + the rewrite in
  `ManifestHandler` (`oci_manifest.go`). Guarded to `models/*` (can't regress); guard branches tested.

**#8 remaining validation (NOT yet done):**
- **Injection happy-path integration test** (local harness): seed a model-serving profile → repackage
  a model to it → seed the runtime collection → `GET /v2/models/<name>/manifests/<quant>` → assert the
  composed manifest (runtime+model+config). This validates the open assumption that serving the runtime
  collection under the profile triggers the per-profile manipulation injection. **Do this before dev.**
- **Multi-model scoping**: a profile with >1 model would inject all; thread the requested model name
  (like the quant, `WithRequestedQuant`) to scope. Non-issue for the single-model first e2e.

**#8 serve interception — the plan (as implemented):**
- Serve flow: `ManifestGetHandler` → `ManifestHandler` (`pkg/api/v1/handlers/oci_manifest.go:362`) →
  `ManifestExists(up{repo,tag,profile})` (`:387`) → `dispatchByMediaType`. A `models/<org>/<name>:<tag>`
  pull 404s today (repackage pushes `…-layer`/`…-config`, not the bare repo).
- **Reuse the existing manipulation engine** (much less code): #7's `parse models` already attaches
  `model/<name>` + `config/<name>` manipulations to each model-serving profile. So at the top of
  `ManifestHandler`: if `req.Repo` is `models/<org>/<name>`, profile is model-serving, and a
  `model/<name>` exists → **rewrite req to the model's runtime collection** (repo+tag), keep
  `req.Profile`, and set `ctx = types.WithRequestedQuant(ctx, req.Tag)` (metalib `1194295`, already
  threads into resolution). The engine then injects model+config for the profile.
- **Needs a model→runtime lookup** (given `model/<name>` + profile → runtime collection). Runtime is
  the config-artefact annotation from repackage; `metalib neo/model_detail.go:modelCompositionsGet`
  surfaces compositions with runtime — likely the query, or a small addition.
- A3 (runtime resolution) is already profile-scoped: `ManifestGetByRequest({Repo,Tag,Profile})`.

**Gotchas:**
- Live pull path — cover with a serve-path integration test (local testcontainers harness, CPU-only)
  BEFORE `frame`. `TestIndexPullAfterTwoArchPushes` is a **pre-existing flake** under full-package
  load (passes in isolation; not model-centric-pull related).
- metahub-api `replace`s metalib `=> ../metahub-core`, so local metalib changes are live in tests.
- Sonar new-code gate (80%): `oci_blob.go` new lines + `manifest_blob` happy path still uncovered —
  do the **consolidated coverage pass AFTER #8** (serve code is new code too).

Full design + step-by-step: DevDiary `Metahub/DevDiary/…/2026-07-07-1438-model-centric-pull-implementation`.
> Updated after each session that modifies this repo.

## What This Repo Is

The main API server for MetaHub — an OCI Distribution Spec compliant
container registry with a neo4j graph backend. Serves both the OCI
registry API (`/v2/...`) and a UI-facing REST API (`/api/v1/...`).

## Architecture

```
cmd/                         CLI entrypoint (cobra)
internal/cmd/                version command
pkg/
  api/v1/
    handlers/                HTTP handlers (chi router)
      oci_manifest.go        OCI manifest GET/PUT/HEAD/DELETE
      oci_blob.go            OCI blob upload/download
      oci_tag.go             OCI tag listing
      oci_referrers.go       OCI referrers API (v1.1.1)
      oci_index.go           Index handling (handleIndex, handleIndexPut)
      oci_catalog.go         OCI catalog (_catalog)
      oci_errors.go          Centralised writeOCIError helper
      repos.go               Collection/repo listing for the UI
      profiles.go            Profile endpoints for the UI
      ui_types.go            DTO types for the UI
      auth.go                Authentication middleware
      oidc.go                OIDC/OAuth integration
      godmode.go             God-mode bypass (dev only)
    router/
      router.go              Route registration (chi)
  config/                    Configuration loading
  lib/helper/                Shared utilities
```

## Key Patterns

### Handler Structure

Handlers live in `pkg/api/v1/handlers/`. They receive a `magazine.Magazine`
interface (from metalib) that provides all backend operations. Handlers
don't talk to neo4j directly — everything goes through the magazine.

### OCI Content Negotiation

The manifest handler (`oci_manifest.go`) implements Accept-header content
negotiation. When a tag resolves to an `:Index` but the client only
accepts manifest types and the index has one descriptor, the underlying
manifest is served instead. See `clientAcceptsIndex`, `handleIndexAsManifest`,
`dispatchByMediaType`.

### UI vs OCI Endpoints

- OCI endpoints: `/v2/{repo}/{image}/manifests/{reference}` etc.
- UI endpoints: `/api/v1/collections`, `/api/v1/profiles` etc.
- Both share the same `magazine.Magazine` backend.

## Testing

```bash
./test.sh 2>&1 > test.log    # full suite with coverage
```

Coverage reports: `coverage.txt` (Go), `report.json` (test report).
SonarQube project key is in `.env`.

## Dependencies

- **metalib** — shared library providing the `magazine.Magazine` interface,
  neo4j backend, types, and Cypher queries. This is the repo you change
  when you need new graph queries or type changes.
- **chi** — HTTP router
- **cobra** — CLI framework

## Recent Changes

- OCI conformance: Accept-header content negotiation (59/59 v1.0.1, 74/0/5 v1.1.1)
- E2E pipeline infrastructure (Taskfile_e2e_dev.yml)
- Session workflow automation (Taskfile_session.yml)
- Dev versioning with `-devN` suffix (.bumpversion.cfg)
- SonarQube version auto-detection (misc/bin/sonar.sh)

## Gotchas

- SonarQube cognitive complexity limit is 15 — decompose large handlers
- The `sessions/` directory is gitignored — worktrees live there
- `docker-compose.yaml` uses namespace `conformance` (was typo `confirmance`)
- Version parsing in `internal/cmd/version.go` handles `-devN` suffix

## 2026-04-18 (ui-and-manipulation session)

### What changed
- **`manifest-expand-and-manipulations-api`**:
  - `UIManifest` gains `Config *UIDescriptor` + `Layers []UIDescriptor`.
  - `ProfileDetailResponse` gains `Manipulations []UIManipulation`.
  - `RepoGetOne` hydrates layers per manifest via the new metalib
    `GetLayersForManifestID` loop inside `hydrateManifestLayers`.
  - `ProfileGetOne` calls `ManipulationsWithConfigByProfileID` and
    serializes through `toUIManipulation(s)` in
    `pkg/api/v1/handlers/ui_manipulations.go`.
- **New fixtures**:
  - `misc/yaml/manipulations/layer/test-extended/1.0.0.yaml` uses the
    new defaults + variants schema without image references so it
    doesn't require upstream/manifest bootstrapping.
  - `misc/yaml/profiles/test-extended/1.0.0.yaml` links to that
    manipulation for tests.
- **`misc/fixtures/manipulations/layer/glow/1.5.0.yaml`** migrated to
  the new variants form (local `registry:5050` mirror).
- **E2E test** (build tag `e2e`):
  `pkg/api/v1/handlers/manipulations_compose_e2e_test.go` probes the
  compose stack and verifies the glow profile's manipulation tree.
- **`go.mod`** replace is now relative (`../metahub-core`) so both the host
  test runs and the compose dev container resolve the session metalib
  worktree correctly.
- **`docker-compose.yaml`** community-edition volume points at the
  session CE worktree (`../community-edition`) so parses inside the
  dev container see the new schema.
- **Drive-by**: `pkg/api/v1/handlers/profiles_test.go` relaxed the
  `resp.Rules[0]` indexing to avoid a pre-existing panic that took the
  whole test suite down.

### Key files
- `pkg/api/v1/handlers/ui_types.go` — DTOs
- `pkg/api/v1/handlers/ui_manipulations.go` — `toUIManipulation(s)`
- `pkg/api/v1/handlers/profiles.go`,
  `pkg/api/v1/handlers/repos.go` — handler hydration
- Tests: `ui_manipulations_test.go`,
  `profiles_manipulations_test.go`, `repos_expand_test.go`,
  `manipulations_compose_e2e_test.go`

### Gotchas
- Pre-existing failing tests still fail on the branch:
  `TestProfileGetOne_URLEncodedLabel`,
  `TestProfileGetOne_SlashInLabel` (both rely on legacy
  action-based profile YAMLs that no longer parse under the current
  profile schema), and `TestIndexPullAfterTwoArchPushes`. None of
  them are caused by this change.
- The dev container takes a long time to come up after a rebuild —
  allow 30+ seconds before probing `/api/v1/heartbeat`.
- Profile linking requires `rule: <expr>` on the profile YAML;
  otherwise `ProfileGetInsert` returns early and manipulation links
  are skipped silently.

## 2026-04-17 Continuation

### What changed
- **Name/URL registry config split**: `RegistryConfig.Hostname` -> `Name` (slug alias in image paths) + `URL` (actual network endpoint). Config uses `name: local-reg, url: registry:5050` format. Backward compatible with legacy `hostname:` field.
- **TLS-aware blob redirects**: Replaced `lyrUp.GetBlobUrl()` (hardcoded https) with `h.mag.GetBlobRedirectURL()` that derives scheme from `RegistryConfig.TLS`. Fixes "server gave HTTP response to HTTPS client" for plain-HTTP upstreams.
- **GET /api/v1/registries endpoint**: New public handler `RegistriesGetAll` + route + 3 tests in `ui_registries_test.go`.
- **Compose task refactor**: All compose tasks moved to `Taskfile_compose.yaml` under `compose:*` namespace. New seed tasks: `compose:seed`, `compose:seed:glow-images`, `compose:seed:graph`, `compose:seed:clean-orphans`, `compose:seed:migrate-labels`.
- **Admin service removed**: `docker-compose.yaml` no longer has an `admin` service. Admin commands run via `go run main.go` inside the `dev` container which has metahub-admin source mounted.
- **Glow manipulation fixture**: `misc/fixtures/manipulations/layer/glow/1.5.0.yaml` pointing at `registry:5050` mirror (overrides community-edition's quay.io-pointing YAML).

### Key files
- `Taskfile.yml` — slimmed to just includes + localConfig
- `Taskfile_compose.yaml` — all compose tasks with shared vars
- `pkg/api/v1/handlers/ui_registries.go` — registries endpoint
- `pkg/api/v1/handlers/oci_blob.go` — uses `GetBlobRedirectURL`
- `pkg/api/v1/handlers/oci_manifest.go` — `resolveUpstreamTarget` returns URL
- `misc/config/compose-config.yaml` — name/url registries + blobs bucket
- `misc/fixtures/manipulations/layer/glow/1.5.0.yaml` — local glow fixture

### Gotchas
- Old `task seedAdmin` etc. no longer work — use `task compose:seed:admin` (namespace changed)
- `compose-config.yaml` uses `name`/`url` (not `hostname`) for registries with ports — clients like manifest-tool reject colons in image path segments
- The `dev` container must have both metalib and metahub-admin source mounted for `compose:seed` to work

## 2026-06-08 deprecate-manifest-wrapper + fix-profile-label-lookup

### What changed
- **Push handler migrated off `:ManifestWrapper`** (`oci_manifest.go`,
  `handleDockerManifestPut`): replaced the `ManifestWrapperGetByLabel` fetch +
  `ComputeManifest(wrapper)` with a single `ComputeManifestForLeaf(ctx, m)` call,
  passing the content-leaf `types.Manifest` from `StoreManifestUpload`. The
  entrypoint is a no-op when the leaf has no reshape target (preserves the prior
  `found` guard) and keeps identical error semantics. **metahub-api now has zero
  functional `ManifestWrapper` references.** Consumed via the in-session
  `replace => ../metahub-core` directive (no go.mod version bump in-session).
- **Profile label lookup fixed** (`profiles.go`): `GET /api/v1/profiles/{label}`
  returned 404 for `#`-delimited labels. Profiles are stored as `name#tag` where
  the name keeps slashes (e.g. `aws/zen3#latest`); the public/URL form maps every
  `/` → `#` (`aws#zen3#latest`). Added `publicProfileLabel(stored)` (the one
  mapping) and `resolveProfileWithStats` (direct lookup → fallback matching the
  public form against stored labels). `ProfilesGetAll` now also emits the public
  label so listing + single-GET agree.

### Key files
- `pkg/api/v1/handlers/oci_manifest.go` — reshape via `ComputeManifestForLeaf`
- `pkg/api/v1/handlers/profiles.go` — `publicProfileLabel`, `resolveProfileWithStats`, listing projection
- `pkg/api/v1/handlers/profiles_test.go` — fixed tests + `TestPublicProfileLabel` unit test

### Gotchas
- The reshape path is exercised by existing handler tests, notably
  `TestManifestOCITest` which asserts the exact reshaped `Docker-Content-Digest`
  (byte-stable OCI output) — good regression guard for the migration.
- Profile single-GET tests bootstrap a `platforms:generic` node before the
  profile so it isn't neo4j node id 0 (otherwise `assert.NotZero(resp.ID)` fails;
  neo ids legitimately start at 0).
- `TestIndexPullAfterTwoArchPushes` flakes under the shared-container
  `-parallel=1` full run (unrelated multi-arch index test); passes in isolation
  and on re-run.
- sonar (`sonar.example.com`) unreachable off-network — quality gate not yet submitted.

### Open / deferred
- The deeper `:ManifestWrapper` read-Cypher migration is metalib-side
  (`migrate-wrapper-read-cypher`); no further metahub-api work expected for it.
- `fix-profile-label-lookup` is a separate OpenSpec change (pre-existing bug,
  orthogonal to the wrapper work).

---

## Session: waldur-outfitter-integration (2026-08-04)

### There are TWO push-authorization layers

This bit hard. `registry.enforce_write_ownership` switches on the namespace
matrix in `OCIWriteAuthMiddleware` (`canPush`: personal → owner, project →
`mh-<org>-admin`/`mh-<org>-<project>-admin`, system → global admin only). But
`ManifestPutHandler` **also** called `ProfileAuthorizedPushHandler`, which
applies a far older rule:

```go
// metalib pkg/backends/neo/profiles.go
if !strings.HasPrefix(req.Repo+"/", req.User) && req.UserRole != "admin" { deny }
```

i.e. a non-admin may push only to a repo whose path starts with their own
**username**. So the middleware would authorise a project admin via their tier
group and the handler refused the same push — the groups were consulted and then
overruled, and **no non-admin could ever push to a project namespace**. Fixed by
skipping the legacy call when enforcement is on; `resolvePushProfile` right
after still validates that the profile exists, which was the other half of what
that call did. With enforcement off, behaviour is unchanged.

If you add a third authz check, put it in the middleware, not the handler.

### Registry-token management is Authentik-backed

`RegistryToken{Create,List,Revoke}` forward the caller's **own bearer** to
Authentik's `/core/tokens/`. `OIDCAuthMiddleware` accepts god-mode tokens too, so
a god-mode session reaches those handlers and hands Authentik a JWT metahub
signed itself → 403 → the old code reported "access token lacks required scope",
sending you after a scope problem that cannot exist. God-mode sessions are now
rejected up front with an actionable message. Detect them via
`AuthRoleFromContext`, which only the god-mode branch sets.

Related symptom: the UI stores the bearer in `sessionStorage`, so a stale token
in one browser and a fresh SSO login in another produce different results for
the same user.

### Dev OIDC points at auth.dev.example.com

Dev Authentik moved into ns `metahub-dev`. `secret-config.yaml` (SOPS) carries
the issuer and `authentik_api_url`; the deployment's `hostAlias` pins that
hostname to the **gateway LB `192.168.5.204`**, not a ClusterIP — a ClusterIP
does not survive recreating the release. metahub-api treats failed OIDC
discovery at startup as fatal, so it crash-loops with `503 authentik starting`
while Authentik boots. That is expected and self-correcting.

### Testing

`./pkg/api/v1/handlers/` is **flaky as a whole package** under testcontainers
contention — `TestIndexPullAfterTwoArchPushes` failed twice and passed twice on
identical code, including one CI red that cleared on rerun. Re-run before
believing a failure; run the single test to confirm.

## Update 2026-09-09 — session close: two OpenSpec changes carried forward

Two proposal-stage OpenSpec changes were carried from the session level into
this repo at session close (both validated with `openspec validate`):

- `openspec/changes/hostname-profile-resolution/` — profile resolution from
  network identity (DNS PTR → hostname regex → profile) at `GET /auth`.
  Proposal + design (D1–D5) + delta spec (6 requirements / 11 scenarios) +
  tasks are complete and ready to implement; nothing is implemented yet.
  Full research incl. deferred CIDR/Netbox resolver designs:
  Obsidian `Metahub/docs/research/profile-resolution-from-network-identity.md`
  (OpenProject WP #47).
- `openspec/changes/add-pull-through-manifest-intercept/` — metahub-api slice
  of the archived session change `pull-through-with-manipulation`
  (OpenProject WP #48). Proposal + intent-level delta spec; `design.md`
  (blob-delivery strategy matrix) is the next artifact. Depends on the metalib
  change `add-upstream-registry-categories`.
