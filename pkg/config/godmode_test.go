package config

import (
	"strings"
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

// TestGodModeConfigFromViper_FromYAML exercises the real production path:
// Viper reading the godmode.users list from a parsed YAML document rather than
// from a hand-built []map[string]string via viper.Set. Nested list-of-maps in
// YAML decode differently from Go-native maps, so this guards against the field
// values being silently dropped during unmarshal (which manifests as
// CheckCredentials failing for a correctly-configured user).
func TestGodModeConfigFromViper_FromYAML(t *testing.T) {
	viper.Reset()
	viper.SetConfigType("yaml")
	doc := `
godmode:
  signing_key: "this-is-a-secret-key-that-is-at-least-32-chars"
  users:
    - username: oci
      password: oci-pass
      role: admin
    - username: meta
      password: hub
      role: user
`
	require.NoError(t, viper.ReadConfig(strings.NewReader(doc)))

	cfg, err := GodModeConfigFromViper()
	require.NoError(t, err)
	assert.True(t, cfg.IsEnabled())
	require.Len(t, cfg.Users, 2)
	assert.Equal(t, "oci", cfg.Users[0].Username)
	assert.Equal(t, "oci-pass", cfg.Users[0].Password)
	assert.Equal(t, "admin", cfg.Users[0].Role)
	assert.Equal(t, "meta", cfg.Users[1].Username)
	assert.Equal(t, "hub", cfg.Users[1].Password)
	assert.Equal(t, "user", cfg.Users[1].Role)
}
