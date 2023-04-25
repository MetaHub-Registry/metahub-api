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
	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// parseCmd.PersistentFlags().String("foo", "", "A help for foo")
	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// parseCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
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
	r := router.Initialize(mag)
	addr := fmt.Sprintf(":%d", port)
	logrus.Infof("Start serving on %s", addr)
	err = http.ListenAndServe(addr, r)
	if err != nil {
		logrus.Fatal("Error serving router: ", err)
	}
}
