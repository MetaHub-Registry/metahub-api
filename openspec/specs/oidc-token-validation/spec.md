## ADDED Requirements

### Requirement: JWKS-based token validation middleware
The system SHALL provide an HTTP middleware that validates `Authorization: Bearer <token>` JWTs on all `/api/v1/protected` routes. Validation MUST use Authentik's JWKS endpoint (discovered via `<issuer>/.well-known/openid-configuration`). The middleware MUST reject tokens with an invalid signature, expired `exp` claim, or mismatched `iss` (issuer) claim.

#### Scenario: Valid Bearer token accepted
- **WHEN** a request to `/api/v1/protected/*` includes `Authorization: Bearer <valid-jwt>`
- **AND** the JWT signature verifies against the JWKS
- **AND** the token is not expired and the issuer matches
- **THEN** the middleware passes the request to the next handler and sets user identity claims in the request context

#### Scenario: Missing Authorization header rejected
- **WHEN** a request to `/api/v1/protected/*` has no `Authorization` header
- **THEN** the middleware responds with HTTP 401 and a JSON error body `{"error": "unauthorized"}`

#### Scenario: Expired token rejected
- **WHEN** the JWT's `exp` claim is in the past
- **THEN** the middleware responds with HTTP 401 with `{"error": "token expired"}`

#### Scenario: Invalid signature rejected
- **WHEN** the JWT signature does not verify against any key in the JWKS
- **THEN** the middleware responds with HTTP 401 with `{"error": "invalid token"}`

#### Scenario: Wrong issuer rejected
- **WHEN** the JWT `iss` claim does not match the configured Authentik issuer URL
- **THEN** the middleware responds with HTTP 401 with `{"error": "invalid token"}`

### Requirement: OIDC claims propagation
The middleware SHALL extract the following claims from the validated JWT and store them in the request context for use by downstream handlers: `sub` (subject/user ID), `preferred_username`, `email`, `groups` (if present). Handlers MUST retrieve these claims using a typed context key (not `jwtauth.FromContext`).

#### Scenario: Claims available in handler
- **WHEN** a valid token is accepted by the middleware
- **THEN** downstream handlers can retrieve `sub`, `preferred_username`, and `email` from `r.Context()`

### Requirement: JWKS key rotation resilience
The system SHALL refresh JWKS keys automatically. If token validation fails due to an unknown `kid` (key ID), the system MUST attempt a single JWKS re-fetch before returning 401.

#### Scenario: Key rotation refresh
- **WHEN** a JWT's `kid` is not found in the cached JWKS
- **THEN** the system fetches the JWKS again and retries validation before rejecting the token
