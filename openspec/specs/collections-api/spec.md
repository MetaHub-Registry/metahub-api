## Requirements

### Requirement: Repo containing only ORAS artifacts does not appear in collections listing

The `POST /api/v1/collections` endpoint SHALL NOT return a repo in the `next` map if that repo contains only ORAS artifact manifests (non-image config media types). A repo MUST have at least one container image `:Manifest` (renamed from `:ManifestData`) to appear in the listing. The graph walk SHALL use the new `:Repo`/`:Slot`/`:Manifest` label set.

During the transition period, the response body SHALL include BOTH a `collections` field (deprecated) and a `repos` field (new canonical name), carrying identical data. The `collections` field is a transitional alias and will be removed in a follow-up change.

#### Scenario: ORAS-only repo is absent from top-level collections listing
- **WHEN** the graph contains a repo that has only ORAS artifact manifests
- **AND** a client sends `POST /api/v1/collections` with `{"repo": ""}`
- **THEN** that repo SHALL NOT appear in the `next` map of the response (neither under `collections` nor under `repos`)

#### Scenario: Image repo appears in top-level collections listing
- **WHEN** the graph contains a repo that has at least one container image `:Manifest`
- **AND** a client sends `POST /api/v1/collections` with `{"repo": ""}`
- **THEN** that repo SHALL appear in the `next` map of the response under both `collections` and `repos` keys

#### Scenario: Mixed repo appears in collections listing
- **WHEN** a repo contains both container image and ORAS artifact manifests
- **AND** a client sends `POST /api/v1/collections` with `{"repo": ""}`
- **THEN** that repo SHALL appear in the response (because it has at least one container image manifest)

#### Scenario: Transitional response has both collections and repos keys
- **WHEN** a client sends `POST /api/v1/collections` with any request body
- **THEN** the response SHALL include both a `collections` key and a `repos` key at the top level
- **AND** both keys SHALL contain identical repo data
- **AND** a deprecation notice SHALL be documented in the API OpenAPI spec for the `collections` field
