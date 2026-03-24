package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOIDCConfigFromViper_MissingIssuer(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	_, err := OIDCConfigFromViper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.issuer")
}

func TestOIDCConfigFromViper_MissingClientID(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("oidc.issuer", "https://auth.example.com")
	_, err := OIDCConfigFromViper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.clientid")
}

func TestOIDCConfigFromViper_MissingRedirectURI(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("oidc.issuer", "https://auth.example.com")
	viper.Set("oidc.clientid", "my-client")
	_, err := OIDCConfigFromViper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.redirect_uri")
}

func TestOIDCConfigFromViper_AllSet(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("oidc.issuer", "https://auth.example.com")
	viper.Set("oidc.clientid", "my-client")
	viper.Set("oidc.secret", "my-secret")
	viper.Set("oidc.redirect_uri", "https://app.example.com/callback")

	cfg, err := OIDCConfigFromViper()
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", cfg.Issuer)
	assert.Equal(t, "my-client", cfg.ClientID)
	assert.Equal(t, "my-secret", cfg.ClientSecret)
	assert.Equal(t, "https://app.example.com/callback", cfg.RedirectURI)
}
