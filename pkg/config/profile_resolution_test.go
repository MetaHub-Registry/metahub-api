package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileResolutionAbsentBlockDisabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	cfg, err := ProfileResolutionFromViper()
	require.NoError(t, err)
	assert.False(t, cfg.Enabled())
	assert.Nil(t, cfg.Inventory, "absent block must leave Inventory nil (byte-identical legacy behaviour)")
}

func TestProfileResolutionDefaultsApplied(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("profile_resolution.inventory.url", "http://metahub-admin-ui:8080")

	cfg, err := ProfileResolutionFromViper()
	require.NoError(t, err)
	require.True(t, cfg.Enabled())
	assert.Equal(t, "http://metahub-admin-ui:8080", cfg.Inventory.URL)
	assert.Equal(t, DefaultInventoryTimeout, cfg.Inventory.Timeout, "timeout default 2s")
	assert.Equal(t, DefaultInventoryCacheTTL, cfg.Inventory.CacheTTL, "cache_ttl default 60s")
	assert.Empty(t, cfg.Inventory.TokenEnv)
}

func TestProfileResolutionRoundTrip(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("profile_resolution.inventory.url", "http://metahub-admin-ui.metahub-dev.svc.cluster.local")
	viper.Set("profile_resolution.inventory.token_env", "INVENTORY_TOKEN")
	viper.Set("profile_resolution.inventory.timeout", "3s")
	viper.Set("profile_resolution.inventory.cache_ttl", "2m")

	cfg, err := ProfileResolutionFromViper()
	require.NoError(t, err)
	require.True(t, cfg.Enabled())
	assert.Equal(t, "INVENTORY_TOKEN", cfg.Inventory.TokenEnv)
	assert.Equal(t, 3*time.Second, cfg.Inventory.Timeout)
	assert.Equal(t, 2*time.Minute, cfg.Inventory.CacheTTL)
}

func TestProfileResolutionEmptyURLDisabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("profile_resolution.inventory.token_env", "INVENTORY_TOKEN")

	cfg, err := ProfileResolutionFromViper()
	require.NoError(t, err)
	assert.False(t, cfg.Enabled(), "block present but url empty ⇒ disabled")
}

func TestProfileResolutionInvalidURLFails(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	// A control character in the host makes url.Parse fail.
	viper.Set("profile_resolution.inventory.url", "http://metahub-admin-ui\x7f:8080")

	_, err := ProfileResolutionFromViper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid URL")
}

func TestInventoryTokenFromEnv(t *testing.T) {
	c := InventoryResolutionConfig{TokenEnv: "TEST_INVENTORY_TOKEN_XYZ"}

	t.Setenv("TEST_INVENTORY_TOKEN_XYZ", "sekrit")
	assert.Equal(t, "sekrit", c.TokenFromEnv())

	t.Setenv("TEST_INVENTORY_TOKEN_XYZ", "")
	assert.Empty(t, c.TokenFromEnv(), "unset env var ⇒ empty token (warns once)")

	c2 := InventoryResolutionConfig{}
	assert.Empty(t, c2.TokenFromEnv(), "no token_env configured ⇒ no token")
}
