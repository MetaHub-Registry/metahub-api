# authentik-provider-config Specification

## Purpose
TBD - created by archiving change durable-registry-tokens. Update Purpose after archive.
## Requirements
### Requirement: metahub OAuth2 provider grants the API scope reproducibly
The metahub OAuth2 providers in Authentik (the prod `metahub-api` provider and the `metahub-dev` provider) SHALL include the `goauthentik.io/api` scope mapping in their selected scope mappings, and this configuration SHALL be captured as reproducible infrastructure (an Authentik blueprint / declarative config) rather than a manual database edit. Without this scope grant, a user's OIDC access token cannot call Authentik's core API, so self-service registry-token management fails with HTTP 403.

#### Scenario: Prod provider grants the API scope
- **WHEN** the Authentik configuration is applied
- **THEN** the `metahub-api` OAuth2 provider's scope mappings include `goauthentik.io/api`
- **AND** a user's access token issued by that provider can call `GET /api/v3/core/tokens/`

#### Scenario: Configuration is reproducible after a rebuild
- **WHEN** the Authentik instance is rebuilt from its declarative configuration (e.g. helm + blueprints) without the manual DB edit
- **THEN** the `goauthentik.io/api` scope mapping is present on the metahub providers
- **AND** self-service registry-token management continues to work

#### Scenario: Blueprint application is idempotent
- **WHEN** the Authentik blueprint is applied more than once
- **THEN** the scope mapping is present exactly once on each provider
- **AND** no duplicate or conflicting mappings are created

