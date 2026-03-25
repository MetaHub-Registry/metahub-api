package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// GodModeUser represents a service account that bypasses OIDC authentication.
type GodModeUser struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Role     string `mapstructure:"role"` // "user" or "admin"; defaults to "admin" if empty
}

// GodModeConfig holds the god-mode authentication bypass configuration.
type GodModeConfig struct {
	SigningKey string        `mapstructure:"signing_key"`
	Users      []GodModeUser `mapstructure:"users"`
}

// IsEnabled returns true if god-mode has at least one user configured.
func (c GodModeConfig) IsEnabled() bool {
	return len(c.Users) > 0
}

// GodModeConfigFromViper reads god-mode configuration from Viper.
// Returns an empty config (disabled) when the godmode section is absent.
// Fatals if users are configured but the signing key is missing or too short.
func GodModeConfigFromViper() (GodModeConfig, error) {
	var cfg GodModeConfig

	// Viper doesn't auto-unmarshal nested slices of structs well with mapstructure,
	// so we read the raw slice and parse manually.
	cfg.SigningKey = viper.GetString("godmode.signing_key")

	var users []GodModeUser
	if err := viper.UnmarshalKey("godmode.users", &users); err != nil {
		// If unmarshal fails, try to read as empty
		users = nil
	}
	cfg.Users = users

	if !cfg.IsEnabled() {
		return cfg, nil
	}

	// Validate signing key when users are present
	if cfg.SigningKey == "" {
		return cfg, fmt.Errorf("godmode.signing_key is required when godmode.users are configured")
	}
	if len(cfg.SigningKey) < 32 {
		return cfg, fmt.Errorf("godmode.signing_key must be at least 32 characters (got %d)", len(cfg.SigningKey))
	}

	return cfg, nil
}
