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

	oidcCfg, err := config.OIDCConfigFromViper()
	if err != nil {
		log.Fatalf("OIDC configuration error: %v", err)
	}
	oidcProvider, err := handlers.NewOIDCProvider(cmd.Context(), oidcCfg)
	if err != nil {
		log.Fatalf("Failed to initialize OIDC provider: %v", err)
	}

	r := router.Initialize(mag, oidcProvider)
	addr := fmt.Sprintf(":%d", port)
	logrus.Infof("Start serving on %s", addr)
	err = http.ListenAndServe(addr, r)
	if err != nil {
		logrus.Fatal("Error serving router: ", err)
	}
}
