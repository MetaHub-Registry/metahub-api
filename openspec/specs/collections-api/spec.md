# collections-api Specification

## Purpose
HTTP API behaviour for collection listing and detail, including profile-filtered manifests.
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

### Requirement: Collection detail endpoint accepts profile filter
The `POST /api/v1/collection` endpoint SHALL accept an optional `profile` string field in the JSON request body. When provided with a non-empty value, the response's `collection.manifests` array SHALL contain only manifests whose resolved `profiles` slice includes the requested label. An empty or whitespace-only `profile` SHALL be treated identically to an omitted field (no filter): every manifest pinned to the tag is returned. A non-empty profile value SHALL be trimmed of surrounding whitespace before matching.

#### Scenario: Empty profile is treated as no profile given
- **WHEN** a POST request is made with `{"repo_name": "qnib/ftest", "tag_name": "latest", "profile": ""}`
- **OR** with `{"repo_name": "qnib/ftest", "tag_name": "latest", "profile": "   "}`
- **THEN** the response SHALL be identical to a request that omits the `profile` field entirely (every manifest pinned to the tag is returned)

#### Scenario: Profile value is trimmed before matching
- **WHEN** a POST request is made with `{"repo_name": "qnib/ftest", "tag_name": "latest", "profile": " cpu#zen2 "}`
- **THEN** the response SHALL filter as if the profile were `"cpu#zen2"` (leading/trailing whitespace ignored)

#### Scenario: No profile filter (backwards compatible)
- **WHEN** a POST request is made with `{"repo_name": "qnib/ftest", "tag_name": "latest"}` (no `profile` field)
- **THEN** the response's `collection.manifests` SHALL contain every manifest pinned to that tag, each decorated with its `profiles[]` list

#### Scenario: Profile filter narrows manifests
- **WHEN** a POST request is made with `{"repo_name": "qnib/ftest", "tag_name": "latest", "profile": "cpu#zen2"}`
- **AND** exactly one manifest under that tag has `profiles` containing `"cpu#zen2"`
- **THEN** the response's `collection.manifests` SHALL contain only that single manifest
- **AND** the kept manifest's `profiles` slice SHALL remain intact (filter is removal-only, not field-mutating)

#### Scenario: Profile filter with no matches returns empty array
- **WHEN** a POST request is made with `{"repo_name": "qnib/ftest", "tag_name": "latest", "profile": "cpu#strix-halo"}`
- **AND** no manifest under that tag has `profiles` containing `"cpu#strix-halo"`
- **THEN** the response status SHALL be 200
- **AND** `collection.manifests` SHALL be an empty array `[]`
- **AND** the rest of the envelope (`collection.name`, `collection.tag`, `repos`, `bread_crumbs`) SHALL be populated as if no filter was applied

#### Scenario: Profile filter applies after profile-label decoration
- **WHEN** a POST request with a `profile` filter is processed
- **THEN** the handler SHALL first resolve each manifest's `profiles[]` via the existing `ManifestGetProfileLabels` call
- **AND** SHALL then drop manifests whose `profiles[]` does not include the requested label
- **AND** SHALL only hydrate layer descriptors for manifests retained by the filter

### Requirement: Collection listing manifest count reflects the profile filter
When `POST /api/v1/collections` is called with a non-empty `profile`, each returned collection's `manifest_count` in the `next` map SHALL reflect only the manifests served under that profile, not the unfiltered tag total. When no profile is given, `manifest_count` SHALL be the full tag total.

#### Scenario: Listing count narrows with a profile
- **WHEN** a POST request is made with `{"repo": "qnib/ftest", "profile": "cpu#zen2"}`
- **AND** the `qnib/ftest:latest` collection has four manifests, one of which is served under `cpu#zen2`
- **THEN** the `next` entry's `manifest_count` SHALL be 1 (the profile-filtered count), matching the number of manifests the detail endpoint returns for the same profile

#### Scenario: Listing count is the full total without a profile
- **WHEN** a POST request is made with `{"repo": "qnib/ftest"}` (no profile)
- **THEN** each `next` entry's `manifest_count` SHALL be the full number of manifests pinned to that tag

### Requirement: Collection listing endpoint accepts profile filter
The `POST /api/v1/collections` endpoint SHALL accept an optional `profile` field in the JSON request body. When provided, the response SHALL only include collections that have at least one Slot for the specified profile.

#### Scenario: Filter collections by profile
- **WHEN** a POST request is made with `{"repo": "biocontainers", "profile": "generic"}`
- **THEN** the response SHALL only include collections under `biocontainers/` that have a Slot for the `generic` profile

#### Scenario: No profile filter (backwards compatible)
- **WHEN** a POST request is made with `{"repo": "biocontainers"}` (no profile field)
- **THEN** the response SHALL return all collections under `biocontainers/` (same as current behavior)

#### Scenario: Profile filter with no matches
- **WHEN** a POST request is made with `{"repo": "biocontainers", "profile": "cpu#strix-halo"}` and no Slots exist for that profile under `biocontainers/`
- **THEN** the response SHALL return an empty `next` map

### Requirement: Collection detail response includes manifest profiles
The `POST /api/v1/collection` response SHALL include a `profiles` field on each manifest in the `collection.manifests` array.

#### Scenario: Manifests with profile associations
- **WHEN** a collection detail is fetched and its manifests are served under profiles
- **THEN** each manifest object in the response SHALL include a `profiles` field containing an array of profile label strings

#### Scenario: Backwards compatibility of manifest structure
- **WHEN** existing API consumers parse the collection detail response
- **THEN** the new `profiles` field SHALL be additive only — all existing fields remain unchanged

