# profiles-api Specification

## Purpose
HTTP API for listing and reading profiles and their hardware and rules.
## Requirements
### Requirement: Profiles list endpoint
The system SHALL expose a `GET /api/v1/profiles` endpoint that returns all profiles stored in the Neo4j graph, including their name, tag, label, and rules.

#### Scenario: List all profiles
- **WHEN** a client sends `GET /api/v1/profiles` with no query parameters
- **THEN** the server SHALL return HTTP 200 with a JSON body `{"profiles": [...]}` where each entry contains `id`, `name`, `tag`, `label`, and `rules`

#### Scenario: Each profile includes its rules
- **WHEN** a profile in the graph has one or more linked `Rule` nodes
- **THEN** the profile entry in the response SHALL include a `rules` array where each rule has `id`, `type`, `op`, `key`, and `value` fields

#### Scenario: Profile with no rules returns empty rules array
- **WHEN** a profile in the graph has no linked `Rule` nodes
- **THEN** the profile entry SHALL include `"rules": []`

#### Scenario: Empty graph returns empty profiles list
- **WHEN** no `Profile` nodes exist in the Neo4j graph
- **THEN** the server SHALL return HTTP 200 with `{"profiles": []}`

### Requirement: Profiles endpoint is publicly readable
The `GET /api/v1/profiles` endpoint SHALL NOT require authentication, consistent with the existing ORAS and collections listing endpoints.

#### Scenario: Unauthenticated profiles request succeeds
- **WHEN** a client sends `GET /api/v1/profiles` without an `Authorization` header
- **THEN** the server SHALL return HTTP 200 (not 401)

### Requirement: Profile single-GET resolves by canonical public label

`GET /api/v1/profiles/{label}` SHALL resolve a profile by its canonical public
label. The label convention SHALL be consistent between the write path (profile
bootstrap/creation) and the read path (HTTP GET), so that a profile created from
a YAML definition is retrievable by the same label the API and UI use to
reference it.

The handler SHALL accept the public label both percent-encoded (e.g.
`aws%23zen3%23latest` in the request path) and decoded (e.g. `aws#zen3#latest`
as a chi URL param), and SHALL return the profile when it exists.

#### Scenario: Percent-encoded `#`-delimited label resolves

- **WHEN** a profile exists whose canonical label is `arch#zen4#latest`
- **AND** a client requests `GET /api/v1/profiles/arch%23zen4%23latest`
- **THEN** the response status SHALL be `200 OK`
- **AND** the response body SHALL contain a non-zero profile `id`
- **AND** the response `label` SHALL equal `arch#zen4#latest`

#### Scenario: Label derived from a slash-containing name resolves

- **WHEN** a profile is created from a definition with name `aws/zen3` (tag `latest`)
- **AND** a client requests `GET /api/v1/profiles/aws%23zen3%23latest`
- **THEN** the response status SHALL be `200 OK`
- **AND** the response `label` SHALL equal `aws#zen3#latest`

#### Scenario: Unknown label returns 404

- **WHEN** no profile matches the requested canonical label
- **THEN** the response status SHALL be `404 Not Found`
- **AND** the body SHALL carry a `profile not found` error

