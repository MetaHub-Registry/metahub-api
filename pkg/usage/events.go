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

// RegistryPull records a single registry pull with full detail (which image, by
// which profile, by which user). Bounded aggregate counts are recorded
// separately via pkg/metrics.ObservePull.
func RegistryPull(kind, repo, tag, digest, profile, user string, status int) {
	logrus.WithFields(logrus.Fields{
		eventField: EventPull,
		"kind":     kind,
		"repo":     repo,
		"tag":      tag,
		"digest":   digest,
		"profile":  orNone(profile),
		"user":     orAnon(user),
		"status":   status,
	}).Info("registry pull")
}

// Auth records an authentication attempt (login / token issuance): which user
// and profile authenticated, by which method, and whether it succeeded.
func Auth(user, profile, method, result string) {
	logrus.WithFields(logrus.Fields{
		eventField: EventAuth,
		"user":     orAnon(user),
		"profile":  orNone(profile),
		"method":   method,
		"result":   result,
	}).Info("auth")
}
