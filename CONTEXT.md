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
