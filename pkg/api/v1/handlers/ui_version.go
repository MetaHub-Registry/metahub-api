package handlers

import (
	"net/http"

	mlver "github.com/metahub-registry/metahub-core/pkg/version"
)

// apiVersion is the metahub-api semver including the -devN suffix. The
// value is injected by internal/cmd to avoid a cyclic import; callers
// outside tests should use VersionGetOne which reads from the injected
// field.
var apiVersion = "unknown"

// SetAPIVersion is called once at startup to wire the api version
// string from internal/cmd into the handler layer. Kept separate from
// the handler struct so the package initialises cleanly without a
// constructor argument.
func SetAPIVersion(v string) {
	apiVersion = v
}

// VersionGetOne handles `GET /api/v1/version` and returns the
// metahub-api + metalib versions the running pod was built with.
// Useful for:
//   - verifying rolling deploys picked up the expected image,
//   - regression guards in e2e pipelines that want to assert a
//     minimum version is live before running behaviour-dependent
//     tests,
//   - the UI's "About" panel.
//
// Public endpoint (no authentication required). Response shape:
//
//	{ "api": "3.1.0-dev2", "metalib": "3.2.0-dev2" }
func (h Handlers) VersionGetOne(w http.ResponseWriter, r *http.Request) {
	_ = h.writeJSON(w, http.StatusOK, envelope{
		"api":     apiVersion,
		"metalib": mlver.Version,
	})
}
