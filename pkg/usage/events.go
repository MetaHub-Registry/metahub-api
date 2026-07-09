// Package usage emits structured registry usage events as JSON log lines.
//
// These events are shipped to VictoriaLogs (container stdout -> fluent-bit ->
// VictoriaLogs) and are the intended home for HIGH-CARDINALITY dimensions —
// repository, tag, digest, user — that must NOT become Prometheus metric labels
// (those would create unbounded time series). Aggregate counts live in
// pkg/metrics with bounded labels; the per-user/per-image detail lives here and
// is queried in VictoriaLogs / Grafana by the `event` field.
package usage

import "github.com/sirupsen/logrus"

const (
	eventField = "event"

	// EventPull marks a registry pull event line.
	EventPull = "registry_pull"
	// EventAuth marks an authentication event line.
	EventAuth = "auth"

	// ResultSuccess / ResultFailure label auth outcomes.
	ResultSuccess = "success"
	ResultFailure = "failure"
)

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func orAnon(s string) string {
	if s == "" {
		return "anonymous"
	}
	return s
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// PullEvent is the full detail of a single registry pull (which image, by which
// profile/user, from which client IP, with what HTTP status).
type PullEvent struct {
	Kind     string
	Repo     string
	Tag      string
	Digest   string
	Profile  string
	User     string
	ClientIP string
	Status   int
}

// RegistryPull records a single registry pull with full detail. Bounded
// aggregate counts are recorded separately via pkg/metrics.ObservePull. The
// client_ip dimension is left raw here; reverse-DNS / GeoIP enrichment happens
// downstream at query time in VictoriaLogs / Grafana.
func RegistryPull(e PullEvent) {
	logrus.WithFields(logrus.Fields{
		eventField:  EventPull,
		"kind":      e.Kind,
		"repo":      e.Repo,
		"tag":       e.Tag,
		"digest":    e.Digest,
		"profile":   orNone(e.Profile),
		"user":      orAnon(e.User),
		"client_ip": orUnknown(e.ClientIP),
		"status":    e.Status,
	}).Info("registry pull")
}

// Auth records an authentication attempt (login / token issuance): which user
// and profile authenticated, by which method, from which client IP, and whether
// it succeeded.
func Auth(user, profile, method, result, clientIP string) {
	logrus.WithFields(logrus.Fields{
		eventField:  EventAuth,
		"user":      orAnon(user),
		"profile":   orNone(profile),
		"method":    method,
		"result":    result,
		"client_ip": orUnknown(clientIP),
	}).Info("auth")
}
