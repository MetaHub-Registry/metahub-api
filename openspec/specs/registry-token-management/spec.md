# registry-token-management Specification

## Purpose
Creation, listing, and revocation of registry auth tokens via the WebUI/API, backed by Authentik.
## Requirements
### Requirement: Create registry token via WebUI
The system SHALL provide `POST /api/v1/protected/registry-token` that creates an Authentik app password for the authenticated user. The endpoint SHALL use the user's own Authentik access token (from the OIDC session) to call Authentik's core API (`POST /api/v3/core/tokens/` with `intent=app_password`), then retrieve the token key via `GET /api/v3/core/tokens/{identifier}/view_key/`. The response SHALL include the app password value and a ready-to-use `docker login` command.

The created app password SHALL be **durable**: by default the request SHALL set `expiring: false` so the token does not inherit Authentik's tenant `default_token_duration` (which is short, e.g. 30 minutes). When a registry-token TTL is configured, the request SHALL instead set `expiring: true` and `expires` to the current time plus the configured TTL. The response SHALL include the token's lifetime (`expiring`, and `expires` when applicable) so the caller knows whether and when the token expires.

#### Scenario: Successful token creation (non-expiring by default)
- **WHEN** an authenticated user sends `POST /api/v1/protected/registry-token` with `{"description": "my laptop"}`
- **AND** no registry-token TTL is configured
- **THEN** the server creates an Authentik app password with identifier `metahub-registry-<timestamp>`, intent `app_password`, and `expiring: false`
- **AND** returns HTTP 201 with:
  ```json
  {
    "identifier": "metahub-registry-1711234567",
    "description": "my laptop",
    "docker_login": "docker login -u alice -p <app-password-key> registry.example.com",
    "username": "alice",
    "password": "<app-password-key>",
    "expiring": false
  }
  ```

#### Scenario: Successful token creation with configured TTL
- **WHEN** a registry-token TTL (e.g. 1 year) is configured
- **AND** an authenticated user sends `POST /api/v1/protected/registry-token`
- **THEN** the server creates the app password with `expiring: true` and `expires` set to now + TTL
- **AND** the response includes `"expiring": true` and the `expires` timestamp

#### Scenario: Authentik API unreachable
- **WHEN** the server cannot reach Authentik's core API
- **THEN** the server responds with HTTP 502 and `{"error": "failed to create registry token: upstream unavailable"}`

#### Scenario: User's access token lacks API scope
- **WHEN** the user's access token does not include the `goauthentik.io/api` scope
- **THEN** Authentik rejects the API call
- **AND** the server responds with HTTP 403 and `{"error": "access token lacks required scope for token management"}`

### Requirement: List registry tokens
The system SHALL provide `GET /api/v1/protected/registry-tokens` that lists the authenticated user's Authentik app passwords (filtered to those with the `metahub-registry-` identifier prefix). The endpoint SHALL call `GET /api/v3/core/tokens/?intent=app_password&identifier__startswith=metahub-registry-` using the user's access token. Each listed token SHALL include its lifetime metadata (`expiring`, and `expires` when the token is expiring) so callers can display whether and when a token expires. The secret key values SHALL NOT be included.

#### Scenario: User has registry tokens
- **WHEN** an authenticated user sends `GET /api/v1/protected/registry-tokens`
- **AND** the user has 2 app passwords with `metahub-registry-` prefix
- **THEN** the server returns HTTP 200 with a JSON array of token metadata (identifier, description, `expiring`, and `expires` when applicable) — excluding the secret key values

#### Scenario: User has no registry tokens
- **WHEN** an authenticated user sends `GET /api/v1/protected/registry-tokens`
- **AND** the user has no app passwords with `metahub-registry-` prefix
- **THEN** the server returns HTTP 200 with an empty JSON array `[]`

### Requirement: Revoke registry token
The system SHALL provide `DELETE /api/v1/protected/registry-tokens/{identifier}` that revokes an Authentik app password. The endpoint SHALL call `DELETE /api/v3/core/tokens/{identifier}/` using the user's access token. The identifier MUST have the `metahub-registry-` prefix to prevent revoking non-registry tokens.

#### Scenario: Successful revocation
- **WHEN** an authenticated user sends `DELETE /api/v1/protected/registry-tokens/metahub-registry-1711234567`
- **THEN** the server deletes the token via Authentik's API
- **AND** returns HTTP 204

#### Scenario: Token not found
- **WHEN** the user sends `DELETE /api/v1/protected/registry-tokens/metahub-registry-nonexistent`
- **AND** Authentik returns 404
- **THEN** the server returns HTTP 404 with `{"error": "token not found"}`

#### Scenario: Attempt to revoke non-registry token blocked
- **WHEN** the user sends `DELETE /api/v1/protected/registry-tokens/my-api-token`
- **AND** the identifier does not start with `metahub-registry-`
- **THEN** the server returns HTTP 400 with `{"error": "invalid registry token identifier"}`

### Requirement: Authentik API URL configuration
The system SHALL read the Authentik API base URL from `oidc.authentik_api_url` in the service configuration. This field SHALL be required when registry token management is used. The URL SHALL point to Authentik's versioned API base (e.g., `https://authentik.example.com/api/v3`).

#### Scenario: Config loaded from environment
- **WHEN** `OIDC_AUTHENTIK_API_URL` is set to `https://authentik.example.com/api/v3`
- **THEN** the registry token management endpoints use this URL for API calls

#### Scenario: Missing config gracefully handled
- **WHEN** `oidc.authentik_api_url` is not configured
- **AND** a user calls `POST /api/v1/protected/registry-token`
- **THEN** the server returns HTTP 501 with `{"error": "registry token management not configured"}`

