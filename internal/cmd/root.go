/*
Copyright © 2022 Christian Kniep <christian@qnib.org>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/logging"
)

// rootCmd represents the base command when called without any subcommands
var (
	rootCmd = &cobra.Command{
		Use:   "metahub-api",
		Short: "API server of MetaHub",
		Long:  `API server of MetaHub`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
	}
	cfgFile = ""
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(logging.InitLog)
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.metahub-api.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().String("minio-url", "minio:9000", "URL to MinIO/S3")
	rootCmd.PersistentFlags().String("minio-user", "minio", "User/AccessKey for MinIO/S3")
	rootCmd.PersistentFlags().String("minio-pass", "", "Password/SecretKey for MinIO/S3")
	rootCmd.PersistentFlags().Bool("minio-insecure", false, "Disable SSL for MinIO/S3")
	rootCmd.PersistentFlags().String("minio-region", "us-east-1", "Region for MinIO/S3")
	// OIDC flags with env var overrides
	rootCmd.PersistentFlags().String("oidc-issuer", "", "OIDC issuer URL (env: OIDC_ISSUER)")
	rootCmd.PersistentFlags().String("oidc-client-id", "", "OIDC client ID (env: OIDC_CLIENTID)")
	rootCmd.PersistentFlags().String("oidc-client-secret", "", "OIDC client secret (env: OIDC_SECRET)")
	rootCmd.PersistentFlags().String("oidc-redirect-uri", "", "OIDC redirect URI (env: OIDC_REDIRECT_URI)")
	rootCmd.PersistentFlags().String("oidc-authentik-api-url", "", "Authentik API base URL for token management (env: OIDC_AUTHENTIK_API_URL)")
	viper.BindEnv("oidc.issuer", "OIDC_ISSUER")
	viper.BindEnv("oidc.clientid", "OIDC_CLIENTID")
	viper.BindEnv("oidc.secret", "OIDC_SECRET")
	viper.BindEnv("oidc.redirect_uri", "OIDC_REDIRECT_URI")
	viper.BindEnv("oidc.authentik_api_url", "OIDC_AUTHENTIK_API_URL")
	// God-mode flags with env var overrides
	viper.BindEnv("godmode.signing_key", "GODMODE_SIGNING_KEY")
	// Envelope token signing key (falls back to godmode.signing_key if not set)
	viper.BindEnv("auth.envelope_signing_key", "AUTH_ENVELOPE_SIGNING_KEY")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".metahub-api")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
