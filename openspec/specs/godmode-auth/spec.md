# godmode-auth Specification

## Purpose
God-mode service accounts that bypass OIDC for administrative access.

## Requirements

### Requirement: God-mode user configuration
The system SHALL support an optional `godmode` configuration section defining service accounts that bypass OIDC authentication.

#### Scenario: God-mode users configured
- **WHEN** the config contains a `godmode.users` list with at least one entry
- **THEN** the system SHALL log a warning at startup: "God-mode authentication enabled for N user(s) — this bypasses OIDC validation"
- **THEN** the system SHALL accept Basic auth credentials matching god-mode users at the `/auth` endpoint

#### Scenario: God-mode not configured
- **WHEN** the config does not contain a `godmode` section or `godmode.users` is empty
- **THEN** the system SHALL NOT log any god-mode warning
- **THEN** the `/auth` endpoint SHALL behave exactly as before (Authentik credential exchange only)

### Requirement: God-mode token issuance
The `/auth` endpoint SHALL issue HMAC-signed JWTs for authenticated god-mode users.

#### Scenario: Valid god-mode credentials at /auth
- **WHEN** a client sends `GET /auth` with Basic auth matching a god-mode user
- **THEN** the system SHALL return HTTP 200 with a JSON body containing a `token` field
- **THEN** the token SHALL be an HMAC-SHA256 signed JWT with claims: `sub` (username), `preferred_username` (username), `aud` (configured client ID), `iss` ("metahub-godmode"), `exp` (current time + 1 hour)

#### Scenario: Invalid god-mode password at /auth
- **WHEN** a client sends `GET /auth` with a username matching a god-mode user but incorrect password
- **THEN** the system SHALL NOT issue a god-mode token
- **THEN** the system SHALL fall through to the Authentik credential exchange (existing behavior)

#### Scenario: Non-god-mode user at /auth
- **WHEN** a client sends `GET /auth` with credentials not matching any god-mode user
- **THEN** the system SHALL proceed with the Authentik credential exchange (existing behavior)

### Requirement: God-mode token verification
Both `OCIAuthMiddleware` and `OIDCAuthMiddleware` SHALL accept god-mode tokens.

#### Scenario: Bearer token with god-mode issuer
- **WHEN** a request includes a Bearer token with `iss` claim equal to "metahub-godmode"
- **THEN** the middleware SHALL verify the token signature using HMAC-SHA256 with the configured `godmode.signing_key`
- **THEN** on successful verification, the middleware SHALL extract claims and proceed as if it were a normal OIDC token

#### Scenario: Bearer token with OIDC issuer
- **WHEN** a request includes a Bearer token with an `iss` claim that is NOT "metahub-godmode"
- **THEN** the middleware SHALL verify the token using the existing OIDC JWKS verifier (unchanged behavior)

#### Scenario: Expired god-mode token
- **WHEN** a request includes a god-mode Bearer token whose `exp` claim is in the past
- **THEN** the middleware SHALL reject the token with HTTP 401

### Requirement: OIDC-less startup
The system SHALL support starting without an OIDC provider when god-mode is the only configured auth method.

#### Scenario: God-mode enabled, OIDC issuer empty
- **WHEN** `godmode.users` is configured AND `oidc.issuer` is empty or unset
- **THEN** the system SHALL start without initializing the OIDC provider
- **THEN** the system SHALL log: "OIDC disabled — god-mode is the only authentication method"
- **THEN** OIDC-dependent routes (`/api/v1/login`, `/api/v1/callback`, `/api/v1/protected/*`) SHALL return HTTP 501 with `{"error": "OIDC authentication is not configured"}`
- **THEN** the `/auth` endpoint SHALL only accept god-mode credentials (no Authentik exchange)
- **THEN** the `/v2/*` middleware SHALL only accept god-mode tokens

#### Scenario: Neither OIDC nor god-mode configured
- **WHEN** `oidc.issuer` is empty AND `godmode.users` is empty or absent
- **THEN** the system SHALL log a fatal error: "no authentication method configured" and exit

#### Scenario: Both OIDC and god-mode configured
- **WHEN** `oidc.issuer` is set AND `godmode.users` is configured
- **THEN** the system SHALL initialize OIDC normally AND enable god-mode
- **THEN** both auth methods SHALL work in parallel (god-mode checked first, then OIDC)

### Requirement: God-mode signing key
The system SHALL require a `godmode.signing_key` when god-mode users are configured.

#### Scenario: Users configured without signing key
- **WHEN** the config has `godmode.users` but no `godmode.signing_key`
- **THEN** the system SHALL log a fatal error at startup and exit

#### Scenario: Signing key too short
- **WHEN** the `godmode.signing_key` is shorter than 32 characters
- **THEN** the system SHALL log a fatal error at startup and exit
