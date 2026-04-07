## ADDED Requirements

### Requirement: Envelope token minting after Authentik exchange
When a Docker client logs in with a username that includes a `/profile` suffix (e.g., `alice/zen2`) and Authentik successfully authenticates the base username (`alice`), the `/auth` endpoint SHALL mint an HMAC-SHA256 signed envelope JWT instead of returning the Authentik JWT directly.

The envelope token SHALL contain:
- `iss`: `"metahub-profile-token"`
- `preferred_username`: the raw username including the profile suffix (e.g., `"alice/zen2"`)
- `inner_token`: the full Authentik JWT string
- `exp`: copied from the inner Authentik JWT's `exp` claim, capped at 1 hour from issuance
- `iat`: current time

#### Scenario: Profile suffix preserved in envelope token
- **WHEN** a Docker client sends `GET /auth` with `account=alice%2Fzen2` and valid credentials for `alice`
- **AND** Authentik successfully authenticates `alice`
- **THEN** the server returns HTTP 200 with `{"token": "<envelope-jwt>"}`
- **AND** the envelope JWT contains `preferred_username: "alice/zen2"`
- **AND** the envelope JWT contains `inner_token: <authentik-jwt>`
- **AND** the envelope JWT is signed with HMAC-SHA256 using the configured signing key

#### Scenario: No profile suffix — Authentik JWT returned unchanged
- **WHEN** a Docker client sends `GET /auth` with `account=alice` (no slash) and valid credentials
- **AND** Authentik successfully authenticates `alice`
- **THEN** the server returns HTTP 200 with `{"token": "<authentik-jwt>"}` unchanged (no envelope)

#### Scenario: Envelope TTL capped at 1 hour
- **WHEN** Authentik returns a JWT with `exp` more than 1 hour in the future
- **THEN** the envelope token's `exp` SHALL be set to `now + 1h` (not the Authentik JWT's `exp`)

#### Scenario: Envelope TTL inherits short Authentik TTL
- **WHEN** Authentik returns a JWT with `exp` less than 1 hour in the future
- **THEN** the envelope token's `exp` SHALL match the Authentik JWT's `exp`

### Requirement: Signing key for envelope tokens
The server SHALL use the `godmode.signing_key` to sign envelope tokens when god-mode is configured. When god-mode is not configured, a dedicated `auth.envelope_signing_key` (minimum 32 characters) SHALL be required. The server SHALL refuse to start if neither key is available and OIDC is configured.

#### Scenario: Signing key from god-mode config
- **WHEN** `godmode.signing_key` is set and `auth.envelope_signing_key` is not set
- **THEN** envelope tokens SHALL be signed with `godmode.signing_key`

#### Scenario: Dedicated signing key
- **WHEN** `auth.envelope_signing_key` is set (regardless of god-mode config)
- **THEN** envelope tokens SHALL be signed with `auth.envelope_signing_key`

#### Scenario: No signing key available with OIDC enabled
- **WHEN** neither `godmode.signing_key` nor `auth.envelope_signing_key` is configured
- **AND** `oidc.issuer` is set (OIDC mode)
- **THEN** the server SHALL log a fatal error at startup and exit
