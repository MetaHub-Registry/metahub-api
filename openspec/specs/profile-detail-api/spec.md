# profile-detail-api Specification

## Purpose
TBD - created by archiving change add-profile-detail-and-filtered-collections. Update Purpose after archive.
## Requirements
### Requirement: Single profile detail endpoint
The API SHALL expose `GET /api/v1/profiles/{label}` that returns a single profile's detail including rules, collection count, and manifest count.

#### Scenario: Existing profile with stats
- **WHEN** a GET request is made to `/api/v1/profiles/cpu%23zen3`
- **THEN** the response SHALL be 200 with JSON body containing `id`, `name`, `tag`, `label`, `rules[]`, `collection_count`, and `manifest_count`

#### Scenario: Unknown profile label
- **WHEN** a GET request is made to `/api/v1/profiles/nonexistent`
- **THEN** the response SHALL be 404

#### Scenario: Profile with URL-encoded label
- **WHEN** a GET request is made to `/api/v1/profiles/node%2Faws%23g5` (label `node/aws#g5`)
- **THEN** the response SHALL correctly resolve the label after URL decoding

### Requirement: Per-manifest profile labels in collection detail
The collection detail response SHALL include a `profiles` field on each manifest listing the profile labels that serve it.

#### Scenario: Manifest served under multiple profiles
- **WHEN** a collection detail is fetched containing a manifest served under `generic` and `cpu#zen3`
- **THEN** the manifest's `profiles` field SHALL be `["generic", "cpu#zen3"]`

#### Scenario: Manifest with no profile association
- **WHEN** a collection detail is fetched containing a manifest not served under any profile
- **THEN** the manifest's `profiles` field SHALL be an empty array `[]` (not null)

