## ADDED Requirements

### Requirement: Admin-CLI machine authentication
The system SHALL accept Bearer tokens issued by Authentik via the Client Credentials grant on all `/api/v1/protected` routes. These tokens MUST be validated by the same OIDC validation middleware used for frontend users (JWKS-based); no separate endpoint or code path is required for CLI callers.

#### Scenario: CLI sends valid access token
- **WHEN** the admin-cli sends `Authorization: Bearer <authentik-access-token>` on a protected endpoint
- **AND** the token was issued by Authentik via Client Credentials grant for a configured service account
- **THEN** the request is authorized and the `sub` and associated claims are available in context

#### Scenario: CLI token without required scope
- **WHEN** the token is valid but lacks the required OIDC scope (if scopes are enforced)
- **THEN** the middleware responds with HTTP 403 and `{"error": "insufficient_scope"}`

### Requirement: OIDC configuration via service config
The system SHALL read Authentik OIDC parameters from the service configuration. The following fields SHALL be required at startup: `oidc.issuer`, `oidc.client_id`, `oidc.client_secret`, `oidc.redirect_uri`. If any required field is missing, the service MUST fail to start with a descriptive error message.

#### Scenario: Missing issuer at startup
- **WHEN** `oidc.issuer` is not set in the configuration
- **THEN** the service exits with a non-zero status and logs `"oidc.issuer is required"`

#### Scenario: Configuration loaded from environment variable
- **WHEN** `OIDC_ISSUER` environment variable is set
- **THEN** the service uses its value as the OIDC issuer URL (Viper env override)

### Requirement: OIDC provider discovery at startup
The system SHALL perform OIDC provider discovery against `<oidc.issuer>/.well-known/openid-configuration` at startup and cache the provider metadata (token endpoint, JWKS URI, authorization endpoint). If discovery fails, the service MUST fail to start.

#### Scenario: Successful discovery
- **WHEN** the service starts and Authentik is reachable at `<oidc.issuer>/.well-known/openid-configuration`
- **THEN** the service caches the provider metadata and starts normally

#### Scenario: Discovery failure at startup
- **WHEN** Authentik is unreachable at startup
- **THEN** the service exits with a non-zero status and logs the discovery error
