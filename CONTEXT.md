# Context: metahub-api

> This file helps AI agents understand the repo quickly.
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
