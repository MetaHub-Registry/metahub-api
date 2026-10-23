## ADDED Requirements

### Requirement: Public responses contain no storage element IDs

No JSON response from the public API SHALL contain a database-assigned element
ID. A resource SHALL be identified in a response by its natural key: `label`
for profiles, `repo` + `tag` for collections, `digest` for manifests and
blobs, and `repo` + `digest` for ORAS artifacts.

This applies to nested objects: a profile response SHALL NOT carry an `id` on
the profile, its rules or its manipulations, and a collection listing SHALL NOT
carry `collection_id` or `tag_id` on its entries.

#### Scenario: Collections listing

- **WHEN** a client calls `POST /api/v1/collections`
- **THEN** every `next` entry carries `repo` and `tag`
- **AND** no entry carries `collection_id` or `tag_id`

#### Scenario: Profile detail

- **WHEN** a client calls `GET /api/v1/profiles/{label}`
- **THEN** the profile, its rules and its manipulations carry no `id`
- **AND** the profile is identified by `label` and a rule by its
  (`type`, `op`, `key`, `value`) tuple

#### Scenario: ORAS artifact

- **WHEN** a client calls `GET /api/v1/oras/artifact`
- **THEN** the artifact carries no `id`
- **AND** it is identified by `repo` and `digest`

### Requirement: Requests address resources by natural key

A request body or query string SHALL NOT be required to carry a storage
element ID. The collection request SHALL address a collection by `repo_name` +
`tag_name`; the ORAS artifact request SHALL address an artifact by `repo` +
`digest` or `repo` + `artifact_type`. A request that carries only a storage ID
SHALL be rejected with a client error explaining the natural key.

#### Scenario: Collection request

- **WHEN** a client sends `POST /api/v1/collection` with `repo_name` and
  `tag_name`
- **THEN** the collection is resolved
- **AND** a payload that carries only `collection_id`/`tag_id` is rejected

#### Scenario: ORAS lookup by id

- **WHEN** a client sends `GET /api/v1/oras/artifact?id=<storage-id>`
- **THEN** the request is rejected
- **AND** `repo` + `digest` resolves the same artifact

### Requirement: SQL-backed resources are addressed by natural key

`www.User` and `www.Secret` SHALL NOT serialise their SQLite row IDs. A user
SHALL be addressed by `username`, a secret by `name`, and the secret delete
route SHALL take the name. The redundant `/users/{id}` routes SHALL NOT exist.

#### Scenario: Secret by name

- **WHEN** a client lists or deletes a secret
- **THEN** the payload carries no `id`
- **AND** `DELETE /user/secret/{name}` removes it

#### Scenario: User by username

- **WHEN** a client reads the current user
- **THEN** the payload carries no `id`
- **AND** the user is identified by `username`

### Requirement: The API uses natural-key core lookups

The API SHALL hydrate manifest layers through
`GetLayersForManifestDigest` and profile manipulations through
`ManipulationsWithConfigByProfileLabel`. An internal element ID MAY be used
inside the core after the key is resolved, but it SHALL NOT cross the handler
boundary or be serialised.

#### Scenario: Manifest layers

- **WHEN** the API hydrates the layers of a manifest
- **THEN** each layer is identified by its `digest`
- **AND** no manifest or layer node ID appears in the response

#### Scenario: Profile manipulations

- **WHEN** the API serves a profile
- **THEN** the manipulations are hydrated by `label`
- **AND** no manipulation node ID appears in the response

### Requirement: A resource is the same across an API restart

Fetching a resource by its natural key after an API restart SHALL return the
same logical resource (same digest, size and relationships) as before the
restart, without relying on a storage ID.

#### Scenario: Restart round-trip

- **WHEN** a collection and a manifest have been written
- **AND** the API is restarted
- **THEN** the collection resolves by `repo` + `tag`
- **AND** the manifest resolves by `digest`
- **AND** the response is equal apart from time-dependent fields

### Requirement: The breaking change is documented

`CHANGELOG.md` SHALL record the removal of storage IDs from the public API as a
breaking change, naming the affected endpoints and the replacement natural keys.

#### Scenario: Release note

- **WHEN** a user reads the changelog for the release that contains this change
- **THEN** the removed fields and their replacements are listed
