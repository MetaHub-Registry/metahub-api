## MODIFIED Requirements

### Requirement: Repo containing only ORAS artifacts does not appear in collections listing
The `POST /api/v1/collections` endpoint SHALL NOT return a repo in the `next` map if that repo contains only ORAS artifact manifests (non-image config media types). A repo MUST have at least one container image manifest to appear in the listing.

#### Scenario: ORAS-only repo is absent from top-level collections listing
- **WHEN** the graph contains a repo that has only ORAS artifact manifests
- **AND** a client sends `POST /api/v1/collections` with `{"repo": ""}`
- **THEN** that repo SHALL NOT appear in the `next` map of the response

#### Scenario: Image repo appears in top-level collections listing
- **WHEN** the graph contains a repo that has at least one container image manifest
- **AND** a client sends `POST /api/v1/collections` with `{"repo": ""}`
- **THEN** that repo SHALL appear in the `next` map of the response

#### Scenario: Mixed repo appears in collections listing
- **WHEN** a repo contains both container image manifests and ORAS artifact manifests
- **AND** a client sends `POST /api/v1/collections` with `{"repo": ""}`
- **THEN** that repo SHALL appear in the `next` map (because it has at least one container image manifest)
