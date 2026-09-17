package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Inventory resolution defaults (openspec change hostname-profile-resolution).
const (
	DefaultInventoryTimeout  = 2 * time.Second
	DefaultInventoryCacheTTL = 60 * time.Second
)

// InventoryResolutionConfig points metahub-api at the inventory service
// (metahub-admin-ui) that owns the ENTIRE resolution chain. metahub-api is a
// thin consumer: it asks one question over one versioned surface
// (GET /api/v1/resolve?ip=) and fail-opens to `generic`.
type InventoryResolutionConfig struct {
	URL      string        `mapstructure:"url" json:"url"` // presence enables resolution
	TokenEnv string        `mapstructure:"token_env" json:"token_env"`
	Timeout  time.Duration `mapstructure:"timeout" json:"timeout"`
	CacheTTL time.Duration `mapstructure:"cache_ttl" json:"cache_ttl"`
}

// ProfileResolutionConfig is the optional `profile_resolution` block.
// There is no separate `enabled` flag: an absent/empty block (no url)
// disables resolution; a present url enables it.
type ProfileResolutionConfig struct {
	Inventory *InventoryResolutionConfig `mapstructure:"inventory" json:"inventory"`
}

// Enabled reports whether resolution is configured.
func (p ProfileResolutionConfig) Enabled() bool {
	return p.Inventory != nil && p.Inventory.URL != ""
}

// ProfileResolutionFromViper reads the `profile_resolution` block. Semantics:
//   - absent block / empty url ⇒ disabled (Inventory == nil, nil error)
//   - url set but unparseable ⇒ startup config error
//   - timeout/cache_ttl zero ⇒ defaults applied
//
// The bearer token is NOT read here — the client resolves `token_env` from
// the environment at wiring time (tokens never live in YAML).
func ProfileResolutionFromViper() (ProfileResolutionConfig, error) {
	var cfg ProfileResolutionConfig
	rawURL := viper.GetString("profile_resolution.inventory.url")
	if rawURL == "" {
		return cfg, nil // disabled — byte-identical legacy behaviour
	}
	if _, err := url.Parse(rawURL); err != nil {
		return cfg, fmt.Errorf("profile_resolution.inventory.url %q is not a valid URL: %w", rawURL, err)
	}
	inv := &InventoryResolutionConfig{
		URL:      rawURL,
		TokenEnv: viper.GetString("profile_resolution.inventory.token_env"),
		Timeout:  viper.GetDuration("profile_resolution.inventory.timeout"),
		CacheTTL: viper.GetDuration("profile_resolution.inventory.cache_ttl"),
	}
	if inv.Timeout <= 0 {
		inv.Timeout = DefaultInventoryTimeout
	}
	if inv.CacheTTL <= 0 {
		inv.CacheTTL = DefaultInventoryCacheTTL
	}
	cfg.Inventory = inv
	return cfg, nil
}

// TokenFromEnv resolves the configured environment variable to the bearer
// token. An unset variable means "no token" (network-trust posture of the
// inventory service) — warn once so misconfigurations are visible.
func (c InventoryResolutionConfig) TokenFromEnv() string {
	if c.TokenEnv == "" {
		return ""
	}
	token := os.Getenv(c.TokenEnv)
	if token == "" {
		log.Warnf("profile_resolution.inventory.token_env=%q is set but the environment variable is empty — resolve requests will be sent without a bearer token", c.TokenEnv)
	}
	return token
}
