// Package metrics owns the metahub-api Prometheus registry and the operational
// and usage metrics exposed on the dedicated /metrics endpoint.
//
// The registry is package-owned (not the global default) so exposition is
// explicit and unit tests stay hermetic. Instrumentation points call the
// exported Observe* helpers; the HTTP middleware (see middleware.go) records
// the generic operational request metrics.
//
// Cardinality policy: usage metrics use bounded labels ONLY. Do NOT add
// per-repository, per-image, per-tag or per-user labels — on a public registry
// those are unbounded and would overwhelm the VictoriaMetrics backend.
package metrics

import (
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace     = "metahub"
	httpSubsystem = "http"
)

// Registry is the package-owned Prometheus registry serving /metrics.
var Registry = prometheus.NewRegistry()

var factory = promauto.With(Registry)

var (
	buildInfo = factory.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "build_info",
		Help:      "Build information; constant 1 labelled by component versions.",
	}, []string{"api", "metalib"})

	// Operational HTTP metrics (recorded by the middleware).
	httpRequestsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: httpSubsystem,
		Name:      "requests_total",
		Help:      "Total HTTP requests by method, route pattern and status code.",
	}, []string{"method", "route", "code"})

	httpRequestDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: httpSubsystem,
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency in seconds by method, route pattern and status code.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "code"})

	httpInFlight = factory.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: httpSubsystem,
		Name:      "requests_in_flight",
		Help:      "Number of HTTP requests currently being served.",
	})

	// Usage metrics (recorded explicitly at domain decision points).
	pullsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pulls_total",
		Help:      "Registry pulls by artifact kind (manifest|blob), resolved profile and status code.",
	}, []string{"kind", "profile", "code"})

	manifestResolutionTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "manifest_resolution_total",
		Help:      "Profile-aware manifest resolutions by resolved profile and result.",
	}, []string{"profile", "result"})

	tagsListTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "tags_list_total",
		Help:      "Tag-list requests by status code.",
	}, []string{"code"})

	catalogListTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "catalog_list_total",
		Help:      "Catalog-list requests by status code.",
	}, []string{"code"})

	tokensIssuedTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "tokens_issued_total",
		Help:      "Tokens issued by auth method (oidc|godmode|anonymous) and profile.",
	}, []string{"auth_method", "profile"})

	authRequestsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "auth_requests_total",
		Help:      "OCI auth decisions by auth method, profile and result (allow|deny).",
	}, []string{"auth_method", "profile", "result"})
)

// enabled gates HTTP instrumentation. It defaults to true and is set once at
// startup from config before any request is served.
var enabled atomic.Bool

func init() {
	enabled.Store(true)
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// SetEnabled toggles HTTP instrumentation (the middleware becomes a pass-through
// when disabled). Call at startup before serving.
func SetEnabled(b bool) { enabled.Store(b) }

// Enabled reports whether HTTP instrumentation is active.
func Enabled() bool { return enabled.Load() }

// SetBuildInfo records the running api and metalib versions as a build_info
// gauge. Call once at startup.
func SetBuildInfo(apiVersion, metalibVersion string) {
	buildInfo.WithLabelValues(apiVersion, metalibVersion).Set(1)
}

// Handler returns an http.Handler that exposes the package registry in
// Prometheus text format.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
}
