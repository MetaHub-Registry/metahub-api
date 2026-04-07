## MODIFIED Requirements

### Requirement: OCI auth middleware token verification (MODIFIED)
The OCI auth middleware SHALL accept Bearer tokens of three types, checked in order:

1. **God-mode tokens** (`iss: "metahub-godmode"`): verified via HMAC-SHA256 with `godmode.signing_key`
2. **Profile envelope tokens** (`iss: "metahub-profile-token"`): the envelope signature is verified via HMAC-SHA256, then the `inner_token` claim is extracted and re-verified via OIDC JWKS. The `preferred_username` from the envelope (containing the profile suffix) is used for identity resolution.
3. **OIDC tokens** (any other `iss`): verified via OIDC JWKS as before

#### Scenario: God-mode token on /v2/ routes
- **WHEN** a request to an authenticated `/v2/*` route includes a Bearer token with `iss` equal to `"metahub-godmode"`
- **THEN** the middleware SHALL verify the HMAC-SHA256 signature using `godmode.signing_key`
- **AND** on success populate `OCIAuthContext` with the god-mode user's claims
- **AND** the request SHALL proceed to the handler

#### Scenario: Profile envelope token accepted and inner token validated
- **WHEN** a request includes a Bearer token with `iss` equal to `"metahub-profile-token"`
- **THEN** the middleware SHALL verify the envelope HMAC-SHA256 signature
- **AND** extract the `inner_token` claim and verify it via OIDC JWKS
- **AND** on success extract `preferred_username` from the envelope (e.g., `"alice/zen2"`)
- **AND** parse username and profile via `parseUsernameProfile` to yield `username=alice`, `profile=zen2`
- **AND** populate `OCIAuthContext` and gorilla context with the extracted identity

#### Scenario: Profile envelope token with expired inner token rejected
- **WHEN** a request includes a valid envelope token (HMAC signature OK)
- **AND** the `inner_token` claim is an expired Authentik JWT
- **THEN** the middleware SHALL reject the request with HTTP 401
- **AND** the Docker client SHALL re-authenticate via `/auth`

#### Scenario: Profile envelope token with invalid inner token signature rejected
- **WHEN** a request includes a valid envelope token
- **AND** the `inner_token` claim is a JWT with an invalid or unknown signature (e.g., Authentik rotated its key)
- **THEN** the middleware SHALL reject the request with HTTP 401

#### Scenario: OIDC token on /v2/ routes (unchanged)
- **WHEN** a request to an authenticated `/v2/*` route includes a Bearer token with a non-god-mode, non-envelope `iss`
- **THEN** the middleware SHALL verify via OIDC JWKS as before (no behavior change)
- **AND** `preferred_username` is used directly (no profile suffix expected in this path)

### Requirement: Docker token service proxies to Authentik ROPC (MODIFIED)
The `/auth` endpoint SHALL accept `Authorization: Basic <base64(username:password)>` credentials from Docker clients and exchange them with Authentik's token endpoint. When the `account` query parameter contains a profile suffix (`user/profile`), the endpoint SHALL return a profile envelope token. Otherwise, it SHALL return the Authentik JWT directly.

#### Scenario: Valid credentials exchange without profile
- **WHEN** a Docker client sends `GET /auth` with `Authorization: Basic <base64(alice:app-password-123)>` and `account=alice`
- **THEN** the server POSTs to Authentik with `username=alice`
- **AND** returns HTTP 200 with `{"token": "<authentik-jwt>"}` unchanged

#### Scenario: Valid credentials exchange with profile suffix
- **WHEN** a Docker client sends `GET /auth` with `Authorization: Basic <base64(alice%2Fzen2:app-password-123)>` and `account=alice%2Fzen2`
- **THEN** the server POSTs to Authentik with `username=alice` (base username only)
- **AND** on success mints an envelope token with `preferred_username: "alice/zen2"` and `inner_token: <authentik-jwt>`
- **AND** returns HTTP 200 with `{"token": "<envelope-jwt>"}`

#### Scenario: Invalid credentials rejected
- **WHEN** a Docker client sends `GET /auth` with invalid credentials
- **AND** Authentik returns an error response
- **THEN** the server responds with HTTP 401 and a JSON error body

#### Scenario: Anonymous access returns anonymous token
- **WHEN** a Docker client sends `GET /auth` without credentials or `account` parameter
- **THEN** the server returns HTTP 200 with an anonymous/generic token
