# oidc-login Specification

## Purpose
OIDC login flow (authorize → callback → session) via Authentik with PKCE.

## Requirements

### Requirement: OIDC login initiation
The system SHALL redirect browser clients to the Authentik OIDC authorization endpoint when `GET /api/v1/login` is called, using the Authorization Code flow with PKCE (`code_challenge_method=S256`). The redirect MUST include a randomly generated, per-request `state` parameter stored in a short-lived signed cookie for CSRF protection. The requested scopes MUST include `openid`, `profile`, `email`, and `goauthentik.io/api` to enable both user identity extraction and Authentik core API access for registry token management.

#### Scenario: Browser initiates login
- **WHEN** a client sends `GET /api/v1/login`
- **THEN** the server responds with HTTP 302 and a `Location` header pointing to Authentik's `/authorize` endpoint with `response_type=code`, `client_id`, `redirect_uri`, `scope=openid profile email goauthentik.io/api`, `state`, `code_challenge`, and `code_challenge_method=S256`

#### Scenario: State cookie is set
- **WHEN** a client sends `GET /api/v1/login`
- **THEN** the server sets a `HttpOnly; SameSite=Lax` cookie containing the `state` value (or a signed HMAC of it) to be validated on callback
