# oras-artifact-browsing Specification

## Purpose
Registry-side browsing of ORAS artifacts (listing and retrieval) and push handling that keeps them discoverable.
## Requirements
### Requirement: Image-only repo does not appear in ORAS listing

The `GET /api/v1/oras` endpoint SHALL NOT return a repo in the `next` map if that repo contains only container image manifests. A repo MUST have at least one ORAS artifact `:Manifest` (non-image `cfg.mediaType`, reachable via `(:Repo)<-[:OF_REPO]-(:Slot)-[:HAS_PUSHED]->(:Manifest)-[:HAS_CONFIG]->(:Blob)`) to appear in the listing.

The graph walk SHALL use the new `:Repo`/`:Slot`/`:Manifest`/`:Blob` label set. The previous walk that referenced `:Collection` and the legacy `:Manifest` wrapper SHALL be deleted.

#### Scenario: Image-only repo is absent from top-level ORAS listing
- **WHEN** the graph contains a repo that has only container image manifests
- **AND** a client sends `GET /api/v1/oras` with no query parameters
- **THEN** that repo SHALL NOT appear in the `next` map of the response

#### Scenario: ORAS-only repo appears in top-level ORAS listing
- **WHEN** the graph contains a repo that has at least one ORAS artifact manifest (non-image config mediaType)
- **AND** a client sends `GET /api/v1/oras` with no query parameters
- **THEN** that repo SHALL appear in the `next` map of the response

#### Scenario: Mixed repo appears in ORAS listing
- **WHEN** a repo contains both container image and ORAS artifact manifests
- **AND** a client sends `GET /api/v1/oras` with no query parameters
- **THEN** that repo SHALL appear in the `next` map (because it has at least one ORAS artifact)

#### Scenario: Query uses the new label set
- **WHEN** the ORAS listing endpoint executes its graph walk
- **THEN** the Cypher query SHALL match on `:Repo`, `:Slot`, `:Manifest`, and `:Blob` labels
- **AND** SHALL NOT reference `:Collection`, `:ManifestData`, or the legacy `:Manifest` wrapper label

### Requirement: Manifest PUT skips compute for ORAS artifacts

The `ManifestPutHandler` SHALL skip `computeAfterManifestPush` when the pushed manifest is an ORAS artifact. ORAS artifacts do not participate in multi-arch Index aggregation or profile-based filtering, so the compute step is unnecessary.

#### Scenario: ORAS artifact push skips compute
- **WHEN** a client pushes an ORAS artifact manifest (with non-empty `artifactType`)
- **THEN** the handler SHALL NOT call `computeAfterManifestPush`
- **AND** the handler SHALL still call `ManifestPutHandlerWrapper` to return the correct `201 Created` response

#### Scenario: Container image push still runs compute
- **WHEN** a client pushes a container image manifest (with empty `artifactType`)
- **THEN** the handler SHALL call `computeAfterManifestPush` as before
- **AND** the handler SHALL call `ManifestPutHandlerWrapper` to return the correct response

### Requirement: Pushed ORAS artifacts appear in browsing endpoints

After pushing an ORAS artifact via `oras push`, the artifact SHALL be visible in the `GET /api/v1/oras` listing and retrievable via `GET /api/v1/oras/artifact`. This requires the metalib layer to wire the Slot graph correctly (companion change in metalib).

#### Scenario: Pushed artifact appears in ORAS listing
- **WHEN** a client pushes an ORAS artifact with repo `hello-artifact` and artifact type `application/vnd.acme.rocket.config`
- **AND** a client sends `GET /api/v1/oras`
- **THEN** the response `next` map SHALL contain an entry for `hello-artifact`

#### Scenario: Pushed artifact is retrievable by repo and type
- **WHEN** a client pushes an ORAS artifact with repo `hello-artifact` and artifact type `application/vnd.acme.rocket.config`
- **AND** a client sends `GET /api/v1/oras/artifact?repo=hello-artifact&artifact_type=application/vnd.acme.rocket.config`
- **THEN** the response SHALL contain the artifact's digest, media type, and artifact type

