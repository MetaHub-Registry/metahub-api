package metrics

import (
	"net/http"
	"strconv"
)

// Bounded label values. Callers SHOULD use these constants so the label sets
// stay closed (and therefore cardinality stays bounded).
const (
	KindManifest = "manifest"
	KindBlob     = "blob"

	ResultHit   = "hit"
	ResultMiss  = "miss"
	ResultError = "error"
	ResultAllow = "allow"
	ResultDeny  = "deny"

	AuthOIDC      = "oidc"
	AuthGodmode   = "godmode"
	AuthAnonymous = "anonymous"
	AuthUnknown   = "unknown"

	profileNone = "none"
)

// StatusOf extracts the HTTP status written so far when w records it (as the
// metrics Middleware's wrapped writer does). Returns 200 when unknown, matching
// net/http's implicit success status. Intended for use in a deferred call after
// the handler has written its response.
func StatusOf(w http.ResponseWriter) int {
	if sw, ok := w.(interface{ Status() int }); ok {
		if s := sw.Status(); s != 0 {
			return s
		}
	}
	return http.StatusOK
}

// ResultFromStatus maps an HTTP status to a bounded resolution result label:
// 2xx/3xx -> hit, 404 -> miss, everything else -> error.
func ResultFromStatus(status int) string {
	switch {
	case status >= 200 && status < 400:
		return ResultHit
	case status == http.StatusNotFound:
		return ResultMiss
	default:
		return ResultError
	}
}

// statusLabel renders an HTTP status code as a label value, treating an unset
// (0) status as 200 — matching net/http's implicit WriteHeader(200).
func statusLabel(status int) string {
	if status == 0 {
		status = http.StatusOK
	}
	return strconv.Itoa(status)
}

// normProfile normalises an empty profile to "none" so the label stays bounded.
func normProfile(profile string) string {
	if profile == "" {
		return profileNone
	}
	return profile
}

// ObservePull records a registry pull of the given kind (KindManifest|KindBlob),
// labelled by resolved profile and status code.
func ObservePull(kind, profile string, status int) {
	pullsTotal.WithLabelValues(kind, normProfile(profile), statusLabel(status)).Inc()
}

// ObserveManifestResolution records a profile-aware manifest resolution.
func ObserveManifestResolution(profile, result string) {
	manifestResolutionTotal.WithLabelValues(normProfile(profile), result).Inc()
}

// ObserveTagsList records a tag-list request by status code.
func ObserveTagsList(status int) {
	tagsListTotal.WithLabelValues(statusLabel(status)).Inc()
}

// ObserveCatalogList records a catalog-list request by status code.
func ObserveCatalogList(status int) {
	catalogListTotal.WithLabelValues(statusLabel(status)).Inc()
}

// ObserveTokenIssued records a token issuance by auth method and profile.
func ObserveTokenIssued(authMethod, profile string) {
	tokensIssuedTotal.WithLabelValues(authMethod, normProfile(profile)).Inc()
}

// ObserveAuthRequest records an OCI auth decision by method, profile and result.
func ObserveAuthRequest(authMethod, profile, result string) {
	authRequestsTotal.WithLabelValues(authMethod, normProfile(profile), result).Inc()
}
