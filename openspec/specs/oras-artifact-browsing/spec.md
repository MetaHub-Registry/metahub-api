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
