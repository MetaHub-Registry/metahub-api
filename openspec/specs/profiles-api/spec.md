## ADDED Requirements

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
