package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestMetricsConfigFromViperDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	cfg := MetricsConfigFromViper()
	if !cfg.Enabled {
		t.Errorf("default Enabled = false, want true")
	}
	if cfg.Port != 9090 {
		t.Errorf("default Port = %d, want 9090", cfg.Port)
	}
}

func TestMetricsConfigFromViperOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("metrics.enabled", false)
	viper.Set("metrics.port", 19191)

	cfg := MetricsConfigFromViper()
	if cfg.Enabled {
		t.Errorf("Enabled = true, want false (overridden)")
	}
	if cfg.Port != 19191 {
		t.Errorf("Port = %d, want 19191 (overridden)", cfg.Port)
	}
}

func TestMetricsConfigEnabledExplicitTrue(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Explicitly setting enabled=true must be honoured (not just the default).
	viper.Set("metrics.enabled", true)
	if cfg := MetricsConfigFromViper(); !cfg.Enabled {
		t.Errorf("Enabled = false, want true (explicit)")
	}
}
