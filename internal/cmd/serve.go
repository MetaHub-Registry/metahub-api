/*
Copyright © 2022 Christian Kniep <christian@qnib.org>
*/
package cmd

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/handlers"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/router"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-api/pkg/pullthrough"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	mlver "github.com/metahub-registry/metahub-core/pkg/version"
)

// flagMetricsPort is the name of the metrics-port flag, referenced both when
// the flag is defined and when its overridden value is read.
const flagMetricsPort = "metrics-port"

// serveCmd represents the parse command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the API server",
	Run:   sCmd,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().Int("port", 80, "The API port")
	serveCmd.Flags().Int(flagMetricsPort, 9090, "The Prometheus metrics port (served on a dedicated listener)")
	serveCmd.Flags().String("user-db", "", "The user database file to use - will be overwritten")
	serveCmd.Flags().String("registries", "", "Backend registries to add (comma-separated)")
}

// startMetricsServer records build info and, when metrics are enabled, serves
// Prometheus metrics on a dedicated listener separate from live API/registry
// traffic. It runs in its own goroutine and never fatals the API — a metrics
// bind failure must not take down the registry.
func startMetricsServer(cmd *cobra.Command) {
	metrics.SetBuildInfo(version, mlver.Version)
	mCfg := config.MetricsConfigFromViper()
	metrics.SetEnabled(mCfg.Enabled)
	if !mCfg.Enabled {
		logrus.Info("Metrics endpoint disabled (metrics.enabled=false)")
		return
	}
	metricsPort := mCfg.Port
	if f := cmd.Flags().Lookup(flagMetricsPort); f != nil && f.Changed {
		metricsPort, _ = cmd.Flags().GetInt(flagMetricsPort)
	}
	go func() {
		mAddr := fmt.Sprintf(":%d", metricsPort)
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		logrus.Infof("Start serving metrics on %s/metrics", mAddr)
		if mErr := http.ListenAndServe(mAddr, mux); mErr != nil {
			logrus.Errorf("metrics server stopped: %v", mErr)
		}
	}()
}

func sCmd(cmd *cobra.Command, args []string) {
	// Banner — logged at WARN level so it stands out in aggregated
	// logs and is easy to grep in `kubectl logs` / Kibana. Answers the
	// frequent "is this pod running the metalib/metahub-api version I
	// expect after a deploy?" question without needing to exec into
	// the container and run `metahub-api version`.
	logrus.Warnf("METAHUB STARTUP  api=v%s  metalib=v%s", version, mlver.Version)
	// Inject into the handler layer so `GET /api/v1/version` can
	// report the api version. metalib is read directly by the handler
	// from its own version package; the api version only lives in the
	// cmd package so has to be passed in.
	handlers.SetAPIVersion(version)
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
		users := []string{}
		for _, u := range godmodeCfg.Users {
			users = append(users, u.Username)
		}
		logrus.Warnf("God-mode authentication enabled for %d user(s) ('%s') — this bypasses OIDC validation", len(godmodeCfg.Users), strings.Join(users, "', '"))
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

	// Metrics live on a dedicated listener; see startMetricsServer.
	startMetricsServer(cmd)

	// Async pull-through cache: streams upstream base layers into MetaHub storage
	// off the client request path (add-pull-through-manifest-intercept). Inert
	// unless a registry opts in via pull_through.enabled.
	policies := pullthrough.MagazinePolicies{Mag: mag}
	pullCache := pullthrough.New(
		pullthrough.MagazineStore{Mag: mag},
		policies,
		nil,
		pullthrough.Options{Workers: policies.MaxWorkers()},
	)
	pullCache.Start(cmd.Context())
	defer pullCache.Stop()

	r := router.Initialize(mag, oidcProvider, godmodeAuth, pullCache)
	addr := fmt.Sprintf(":%d", port)
	logrus.Infof("Start serving on %s", addr)
	err = http.ListenAndServe(addr, r)
	if err != nil {
		logrus.Fatal("Error serving router: ", err)
	}
}
