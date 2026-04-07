/*
Copyright © 2022 Christian Kniep <christian@qnib.org>
*/
package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/handlers"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/router"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

// serveCmd represents the parse command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the API server",
	Run:   sCmd,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().Int("port", 80, "The API port")
	serveCmd.Flags().String("user-db", "", "The user database file to use - will be overwritten")
	serveCmd.Flags().String("registries", "", "Backend registries to add (comma-separated)")
}

func sCmd(cmd *cobra.Command, args []string) {
	config.SetCommandValues(cmd)
	port, _ := cmd.Flags().GetInt("port")
	mag, _, err := magazine.NewMagazine(cmd, "")
	if err != nil {
		err = errors.Wrap(err, "Cannot create magazine")
		log.Fatal(err)
	}

	// Load god-mode config first (it determines whether OIDC is required)
	godmodeCfg, err := config.GodModeConfigFromViper()
	if err != nil {
		log.Fatalf("God-mode configuration error: %v", err)
	}

	var godmodeAuth *handlers.GodModeAuth
	if godmodeCfg.IsEnabled() {
		logrus.Warnf("God-mode authentication enabled for %d user(s) — this bypasses OIDC validation", len(godmodeCfg.Users))
		godmodeAuth = handlers.NewGodModeAuth(godmodeCfg)
	}

	// Conditionally initialize OIDC
	var oidcProvider *handlers.OIDCProvider
	oidcCfg, oidcErr := config.OIDCConfigFromViper()
	if oidcErr == nil && oidcCfg.Issuer != "" {
		// OIDC is configured — initialize the provider
		oidcProvider, err = handlers.NewOIDCProvider(cmd.Context(), oidcCfg)
		if err != nil {
			log.Fatalf("Failed to initialize OIDC provider: %v", err)
		}
		logrus.Info("OIDC authentication initialized")
	} else if godmodeCfg.IsEnabled() {
		// OIDC is not configured, but god-mode is — run without OIDC
		logrus.Warn("OIDC disabled — god-mode is the only authentication method")
	} else {
		// Neither OIDC nor god-mode — cannot serve
		log.Fatalf("No authentication method configured: OIDC error: %v, god-mode: disabled", oidcErr)
	}

	// Validate envelope signing key when OIDC is active (envelope tokens require it)
	if oidcProvider != nil {
		if _, err := config.EnvelopeSigningKey(); err != nil {
			log.Fatalf("Envelope token signing key not configured: %v", err)
		}
	}

	r := router.Initialize(mag, oidcProvider, godmodeAuth)
	addr := fmt.Sprintf(":%d", port)
	logrus.Infof("Start serving on %s", addr)
	err = http.ListenAndServe(addr, r)
	if err != nil {
		logrus.Fatal("Error serving router: ", err)
	}
}
