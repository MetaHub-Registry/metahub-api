package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func SetCommandValues(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them,
		// so bind them to their equivalent keys with underscores,
		// e.g. --server_url to server_url
		envVarSuffix := ""
		if strings.Contains(f.Name, "-") {
			envVarSuffix = strings.ReplaceAll(f.Name, "-", "_")
			viper.BindEnv(f.Name, envVarSuffix)
		}

		// Apply the viper config value to the flag
		// when the flag is not set and viper has a value
		if !f.Changed && viper.GetString(envVarSuffix) != "" {
			val := viper.GetString(envVarSuffix)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
