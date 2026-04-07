## MODIFIED Requirements

### Requirement: Image-only repo does not appear in ORAS listing
The `GET /api/v1/oras` endpoint SHALL NOT return a repo in the `next` map if that repo contains only container image manifests. A repo MUST have at least one ORAS artifact manifest (non-image `cfg.mediaType`, linked via `CREATED` relationship) to appear in the listing.

#### Scenario: Image-only repo is absent from top-level ORAS listing
- **WHEN** the graph contains a repo that has only container image manifests
- **AND** a client sends `GET /api/v1/oras` with no query parameters
- **THEN** that repo SHALL NOT appear in the `next` map of the response

#### Scenario: ORAS-only repo appears in top-level ORAS listing
- **WHEN** the graph contains a repo that has at least one ORAS artifact manifest
- **AND** a client sends `GET /api/v1/oras` with no query parameters
- **THEN** that repo SHALL appear in the `next` map of the response

#### Scenario: Mixed repo appears in ORAS listing
- **WHEN** a repo contains both container image manifests and ORAS artifact manifests
- **AND** a client sends `GET /api/v1/oras` with no query parameters
- **THEN** that repo SHALL appear in the `next` map (because it has at least one ORAS artifact)
