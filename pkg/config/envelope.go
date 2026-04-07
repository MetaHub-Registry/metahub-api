package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// EnvelopeConfig holds configuration for the profile envelope token.
type EnvelopeConfig struct {
	SigningKey string `mapstructure:"signing_key"`
}

// EnvelopeConfigFromViper reads envelope token configuration from Viper.
// Returns an empty config when the auth section is absent.
func EnvelopeConfigFromViper() EnvelopeConfig {
	return EnvelopeConfig{
		SigningKey: viper.GetString("auth.envelope_signing_key"),
	}
}

// EnvelopeSigningKey resolves the signing key for envelope tokens.
// It returns auth.envelope_signing_key if set, falling back to godmode.signing_key.
// Returns an error if neither is configured.
func EnvelopeSigningKey() ([]byte, error) {
	if k := viper.GetString("auth.envelope_signing_key"); k != "" {
		if len(k) < 32 {
			return nil, fmt.Errorf("auth.envelope_signing_key must be at least 32 characters (got %d)", len(k))
		}
		return []byte(k), nil
	}
	if k := viper.GetString("godmode.signing_key"); k != "" {
		if len(k) < 32 {
			return nil, fmt.Errorf("godmode.signing_key must be at least 32 characters (got %d)", len(k))
		}
		return []byte(k), nil
	}
	return nil, fmt.Errorf("no envelope signing key configured: set auth.envelope_signing_key or godmode.signing_key")
}
