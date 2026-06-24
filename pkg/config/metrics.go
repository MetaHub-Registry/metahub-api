package config

import "github.com/spf13/viper"

// MetricsConfig controls the dedicated Prometheus metrics endpoint.
type MetricsConfig struct {
	// Enabled toggles the /metrics endpoint and HTTP instrumentation. Default true.
	Enabled bool
	// Port is the dedicated metrics listener port. Default 9090.
	Port int
}

// MetricsConfigFromViper reads metrics configuration from Viper, applying
// defaults (enabled=true, port=9090). The metrics endpoint is served on its
// own listener, separate from the API/registry port, and is never exposed via
// the public gateway.
func MetricsConfigFromViper() MetricsConfig {
	cfg := MetricsConfig{
		Enabled: true,
		Port:    9090,
	}
	if viper.IsSet("metrics.enabled") {
		cfg.Enabled = viper.GetBool("metrics.enabled")
	}
	if p := viper.GetInt("metrics.port"); p != 0 {
		cfg.Port = p
	}
	return cfg
}
