package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGodModeConfigFromViper_EmptyConfig(t *testing.T) {
	viper.Reset()
	cfg, err := GodModeConfigFromViper()
	require.NoError(t, err)
	assert.False(t, cfg.IsEnabled())
	assert.Empty(t, cfg.Users)
}

func TestGodModeConfigFromViper_ValidConfig(t *testing.T) {
	viper.Reset()
	viper.Set("godmode.signing_key", "this-is-a-secret-key-that-is-at-least-32-chars")
	viper.Set("godmode.users", []map[string]string{
		{"username": "oci", "password": "oci-pass"},
		{"username": "test", "password": "test-pass"},
	})

	cfg, err := GodModeConfigFromViper()
	require.NoError(t, err)
	assert.True(t, cfg.IsEnabled())
	assert.Len(t, cfg.Users, 2)
	assert.Equal(t, "oci", cfg.Users[0].Username)
	assert.Equal(t, "oci-pass", cfg.Users[0].Password)
}

func TestGodModeConfigFromViper_MissingSigningKey(t *testing.T) {
	viper.Reset()
	viper.Set("godmode.users", []map[string]string{
		{"username": "oci", "password": "oci-pass"},
	})

	_, err := GodModeConfigFromViper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "godmode.signing_key is required")
}

func TestGodModeConfigFromViper_ShortSigningKey(t *testing.T) {
	viper.Reset()
	viper.Set("godmode.signing_key", "too-short")
	viper.Set("godmode.users", []map[string]string{
		{"username": "oci", "password": "oci-pass"},
	})

	_, err := GodModeConfigFromViper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")
}
