## ADDED Requirements

### Requirement: Docker token service proxies to Authentik ROPC
The `/auth` endpoint SHALL accept `Authorization: Basic <base64(username:password)>` credentials from Docker clients and exchange them with Authentik's token endpoint (`POST /application/o/token/`) using `grant_type=client_credentials` with the provided username and password (app password). The endpoint SHALL return the Authentik-issued JWT as the Docker registry bearer token in the response body as `{"token": "<jwt>"}`.

#### Scenario: Valid credentials exchange
- **WHEN** a Docker client sends `GET /auth` with `Authorization: Basic <base64(alice:app-password-123)>` and `account=alice`
- **THEN** the server POSTs to Authentik's token endpoint with `grant_type=client_credentials`, `client_id=<configured>`, `username=alice`, `password=app-password-123`
- **AND** returns HTTP 200 with `{"token": "<authentik-jwt>"}`

#### Scenario: Invalid credentials rejected
- **WHEN** a Docker client sends `GET /auth` with invalid credentials (wrong password or unknown user)
- **AND** Authentik returns an error response
- **THEN** the server responds with HTTP 401 and a JSON error body

#### Scenario: Anonymous access returns anonymous token
- **WHEN** a Docker client sends `GET /auth` without an `Authorization` header or without an `account` parameter
- **THEN** the server returns HTTP 200 with an anonymous/generic token that allows unauthenticated pull operations (if configured)

#### Scenario: Username with profile parsed
- **WHEN** a Docker client authenticates with username `alice/myprofile`
- **THEN** the server extracts `alice` as the username and `myprofile` as the profile
- **AND** sends `username=alice` to Authentik's token endpoint
- **AND** the returned token is associated with the profile for subsequent requests

### Requirement: OCI registry JWKS-based authentication middleware
All `/v2/` OCI endpoints (except `GET /v2/` heartbeat) SHALL be protected by a middleware that validates `Authorization: Bearer <jwt>` tokens using the same JWKS endpoint used for `/api/v1/protected` routes. The middleware SHALL reject requests with missing, expired, invalid-signature, or wrong-issuer tokens.

#### Scenario: Valid JWT accepted on manifest HEAD
- **WHEN** a Docker client sends `HEAD /v2/{repo}/manifests/{ref}` with `Authorization: Bearer <valid-authentik-jwt>`
- **AND** the JWT signature verifies against the JWKS and the token is not expired
- **THEN** the request is passed to the manifest handler with user identity available in context

#### Scenario: Missing Authorization header returns 401 with challenge
- **WHEN** a Docker client sends `GET /v2/{repo}/manifests/{ref}` without an `Authorization` header
- **THEN** the server responds with HTTP 401
- **AND** the response includes `WWW-Authenticate: Bearer realm="<host>/auth",service="metahub-api"` header

#### Scenario: Expired JWT returns 401
- **WHEN** a Docker client sends a request with an expired JWT
- **THEN** the server responds with HTTP 401 with the `WWW-Authenticate` challenge header
- **AND** the Docker client re-authenticates via `/auth`

#### Scenario: Invalid signature returns 401
- **WHEN** a Docker client sends a JWT signed by an unknown key
- **THEN** the server responds with HTTP 401

### Requirement: User identity and profile propagation from JWT claims
The `/v2/` middleware SHALL extract user identity from the validated JWT's claims and make it available to downstream OCI handlers. The middleware SHALL extract `preferred_username` (or `email` as fallback) as the username, and parse the `username/profile` pattern to determine the registry profile. If no profile is specified, the default profile `generic` SHALL be used.

#### Scenario: Username and profile extracted from claims
- **WHEN** a validated JWT contains `preferred_username: "alice/staging"`
- **THEN** the OCI handler context contains `userName=alice`, `profileName=staging`

#### Scenario: Default profile when no profile specified
- **WHEN** a validated JWT contains `preferred_username: "alice"` (no slash)
- **THEN** the OCI handler context contains `userName=alice`, `profileName=generic`

#### Scenario: Email fallback for username
- **WHEN** a validated JWT has empty `preferred_username` but contains `email: "alice@example.com"`
- **THEN** the OCI handler context contains `userName=alice@example.com`, `profileName=generic`

### Requirement: OCI heartbeat remains accessible
The `GET /v2/` endpoint SHALL respond with HTTP 200 and an empty JSON object without requiring authentication, per the OCI Distribution Spec.

#### Scenario: Unauthenticated heartbeat
- **WHEN** a client sends `GET /v2/` without an `Authorization` header
- **THEN** the server responds with HTTP 200

### Requirement: Legacy SQLite auth removed from /v2/ routes
The legacy `AuthMiddleware`, `AuthMiddlewareHandler`, and `AuthMiddlewareFunc` SHALL be removed. The gorilla context keys (`accountName`, `userName`, `userRole`, `profileName`) SHALL no longer be set by the auth middleware. OCI handlers SHALL read user identity from the new typed context.

#### Scenario: Old SQLite tokens rejected
- **WHEN** a Docker client sends a Bearer token that was issued by the old SQLite-based `/auth` endpoint
- **THEN** the JWKS middleware rejects the token with HTTP 401 (since it's not a valid Authentik JWT)
