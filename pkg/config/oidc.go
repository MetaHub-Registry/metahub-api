package config

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

// OIDCConfigFromViper reads OIDC configuration from Viper and validates required fields.
func OIDCConfigFromViper() (magazine.OIDCCfg, error) {
	cfg := magazine.OIDCCfg{
		Issuer:       viper.GetString("oidc.issuer"),
		ClientID:     viper.GetString("oidc.clientid"),
		ClientSecret: viper.GetString("oidc.secret"),
		RedirectURI:  viper.GetString("oidc.redirect_uri"),
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
