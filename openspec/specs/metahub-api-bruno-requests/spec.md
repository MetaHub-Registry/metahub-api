# metahub-api-bruno-requests Specification

## Purpose
Bruno request-collection coverage for the metahub-api HTTP endpoints.

## Requirements

### Requirement: Bruno request for collections list navigation
The metahub-api Bruno collection SHALL include a request file for `POST /api/v1/collections` under `misc/bruno/metahub-api/mh-api/`.

#### Scenario: List collections request is present and executable
- **WHEN** a developer opens `misc/bruno/metahub-api/mh-api/listCollections.bru` in Bruno
- **THEN** the request SHALL target `POST {{host}}/api/v1/collections` with a JSON body `{"repo": "{{repo}}"}`
- **THEN** Bruno SHALL be able to execute the request against a running metahub-api instance without modification

### Requirement: Bruno request for single collection detail
The metahub-api Bruno collection SHALL include a request file for `POST /api/v1/collection` under `misc/bruno/metahub-api/mh-api/`.

#### Scenario: Get single collection request is present and executable
- **WHEN** a developer opens `misc/bruno/metahub-api/mh-api/getCollection.bru` in Bruno
- **THEN** the request SHALL target `POST {{host}}/api/v1/collection` with a JSON body `{"repo_name": "{{repo}}", "tag_name": "{{tag}}"}`
- **THEN** Bruno SHALL be able to execute the request against a running metahub-api instance without modification

### Requirement: Bruno request for ORAS artifact list navigation
The metahub-api Bruno collection SHALL include a request file for `GET /api/v1/oras` under `misc/bruno/metahub-api/mh-api/`.

#### Scenario: List ORAS artifacts request is present and executable
- **WHEN** a developer opens `misc/bruno/metahub-api/mh-api/listOrasArtifacts.bru` in Bruno
- **THEN** the request SHALL target `GET {{host}}/api/v1/oras` with an optional `repo` query parameter pre-filled with `{{repo}}`
- **THEN** Bruno SHALL be able to execute the request without modification

### Requirement: Bruno request for single ORAS artifact detail
The metahub-api Bruno collection SHALL include a request file for `GET /api/v1/oras/artifact` under `misc/bruno/metahub-api/mh-api/`.

#### Scenario: Get single ORAS artifact request is present and executable
- **WHEN** a developer opens `misc/bruno/metahub-api/mh-api/getOrasArtifact.bru` in Bruno
- **THEN** the request SHALL target `GET {{host}}/api/v1/oras/artifact` with query parameters `repo={{repo}}` and `artifact_type={{artifact_type}}`
- **THEN** Bruno SHALL be able to execute the request without modification

### Requirement: Bruno requests inherit collection-level auth
All request files under `misc/bruno/metahub-api/mh-api/` SHALL declare `auth: inherit` so they use the collection-level basic auth configuration.

#### Scenario: Auth is inherited not duplicated
- **WHEN** a developer inspects any `.bru` file under `mh-api/`
- **THEN** the file SHALL NOT contain an inline `auth` block
- **THEN** the file SHALL inherit authentication from the parent `collection.bru`
