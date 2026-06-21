package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

// OIDCConfig extends the base OIDCCfg with metahub-api-specific fields.
type OIDCConfig struct {
	magazine.OIDCCfg
	// AuthentikAPIURL is the base URL for Authentik's core REST API (e.g., "https://authentik.example.com/api/v3").
	// Required for registry token management (create/list/revoke app passwords).
	AuthentikAPIURL string
	// RegistryTokenTTL is the lifetime applied to registry app passwords created
	// via the WebUI. A zero value means non-expiring (the default), so registry
	// credentials do not inherit Authentik's short tenant default_token_duration.
	// A positive value opts into rotation: tokens are created with expires = now + TTL.
	RegistryTokenTTL time.Duration
}

// OIDCConfigFromViper reads OIDC configuration from Viper and validates required fields.
func OIDCConfigFromViper() (OIDCConfig, error) {
	cfg := OIDCConfig{
		OIDCCfg: magazine.OIDCCfg{
			Issuer:       viper.GetString("oidc.issuer"),
			ClientID:     viper.GetString("oidc.clientid"),
			ClientSecret: viper.GetString("oidc.secret"),
			RedirectURI:  viper.GetString("oidc.redirect_uri"),
		},
		AuthentikAPIURL:  viper.GetString("oidc.authentik_api_url"),
		RegistryTokenTTL: viper.GetDuration("oidc.registry_token_ttl"),
	}
	if cfg.Issuer == "" {
		return cfg, fmt.Errorf("oidc.issuer is required but not set")
	}
	if cfg.ClientID == "" {
		return cfg, fmt.Errorf("oidc.clientid is required but not set")
	}
	if cfg.RedirectURI == "" {
		return cfg, fmt.Errorf("oidc.redirect_uri is required but not set")
	}
	return cfg, nil
}
