package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveRef mirrors handlers.loadModelDetail's ref resolution: prefer {ref},
// else reassemble the org-qualified {org}/{name} pair.
func resolveRef(r *http.Request) string {
	ref := chi.URLParam(r, "ref")
	if ref == "" {
		if org := chi.URLParam(r, "org"); org != "" {
			ref = org + "/" + chi.URLParam(r, "name")
		}
	}
	return ref
}

// TestModelRoutesResolveOrgQualified guards the model-card routing for
// org-qualified names (models/<org>/<name>). The UI requests `qnib%2Fminicpm5`;
// chi 307-redirects that to the decoded two-segment path `/models/qnib/minicpm5`,
// which the lone `/models/{ref}` route can't match (→ 404 "Failed to load model").
// The {org}/{name} routes must catch the redirected form and reassemble the ref.
// This also asserts the {ref} and {org}/{name} routes don't conflict at
// registration and that single-segment names still work.
func TestModelRoutesResolveOrgQualified(t *testing.T) {
	var gotRef, gotHandler string
	spy := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			gotRef = resolveRef(r)
			gotHandler = name
			w.WriteHeader(http.StatusOK)
		}
	}

	r := chi.NewRouter()
	// Same registration order as pkg/api/v1/router/router.go.
	r.Get("/models/{ref}", spy("one"))
	r.Get("/models/{ref}/compositions", spy("compositions"))
	r.Get("/models/{org}/{name}", spy("one"))
	r.Get("/models/{org}/{name}/compositions", spy("compositions"))

	cases := []struct {
		path        string
		wantRef     string
		wantHandler string
	}{
		{"/models/e2e-gguf43", "e2e-gguf43", "one"},                      // single-segment still works
		{"/models/qnib/minicpm5", "qnib/minicpm5", "one"},               // the %2F 307 redirect target
		{"/models/qnib/minicpm5/compositions", "qnib/minicpm5", "compositions"},
		{"/models/e2e-gguf43/compositions", "e2e-gguf43", "compositions"},
	}
	for _, tc := range cases {
		gotRef, gotHandler = "", ""
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		require.Equalf(t, http.StatusOK, w.Code, "route %s must match a handler, not 404", tc.path)
		assert.Equalf(t, tc.wantRef, gotRef, "ref for %s", tc.path)
		assert.Equalf(t, tc.wantHandler, gotHandler, "handler for %s", tc.path)
	}
}
