## ADDED Requirements

### Requirement: OIDC login initiation
The system SHALL redirect browser clients to the Authentik OIDC authorization endpoint when `GET /api/v1/login` is called, using the Authorization Code flow with PKCE (`code_challenge_method=S256`). The redirect MUST include a randomly generated, per-request `state` parameter stored in a short-lived signed cookie for CSRF protection.

#### Scenario: Browser initiates login
- **WHEN** a client sends `GET /api/v1/login`
- **THEN** the server responds with HTTP 302 and a `Location` header pointing to Authentik's `/authorize` endpoint with `response_type=code`, `client_id`, `redirect_uri`, `scope=openid profile email`, `state`, `code_challenge`, and `code_challenge_method=S256`

#### Scenario: State cookie is set
- **WHEN** a client sends `GET /api/v1/login`
- **THEN** the server sets a `HttpOnly; SameSite=Lax` cookie containing the `state` value (or a signed HMAC of it) to be validated on callback

### Requirement: OIDC callback handling
The system SHALL handle the OIDC callback at `GET /api/v1/callback`. It MUST validate the `state` parameter against the cookie, exchange the authorization `code` for tokens using Authentik's token endpoint (including the `code_verifier`), verify the ID token signature via JWKS, and redirect the user to the configured frontend URL.

#### Scenario: Successful callback
- **WHEN** Authentik redirects to `GET /api/v1/callback?code=<code>&state=<state>`
- **AND** the `state` matches the cookie value
- **THEN** the server exchanges the code for tokens, verifies the ID token, and redirects to `<frontend_redirect_url>#access_token=<access_token>`

#### Scenario: State mismatch on callback
- **WHEN** the `state` parameter does not match the stored cookie value
- **THEN** the server responds with HTTP 400 and an error message; no token is issued

#### Scenario: Invalid code on callback
- **WHEN** Authentik returns an error (e.g., `error=access_denied`) on the callback
- **THEN** the server responds with HTTP 400 containing the OIDC error description

### Requirement: OIDC logout
The system SHALL provide `GET /api/v1/protected/logout` that clears the user's session state server-side and optionally redirects to Authentik's end-session endpoint. This endpoint MUST require a valid Bearer token.

#### Scenario: Authenticated logout
- **WHEN** a client sends `GET /api/v1/protected/logout` with a valid `Authorization: Bearer <token>` header
- **THEN** the server responds with HTTP 200 or HTTP 302 to Authentik's logout URL

#### Scenario: Unauthenticated logout attempt
- **WHEN** a client sends `GET /api/v1/protected/logout` without a valid token
- **THEN** the server responds with HTTP 401
