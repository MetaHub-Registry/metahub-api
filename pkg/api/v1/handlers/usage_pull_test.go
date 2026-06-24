package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// pullStatusWriter exposes Status() so metrics.StatusOf reads a concrete code.
type pullStatusWriter struct {
	http.ResponseWriter
	status int
}

func (p pullStatusWriter) Status() int { return p.status }

func TestResolvePullIdentity(t *testing.T) {
	t.Run("no auth context, request profile used", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/v2/x/manifests/latest", nil)
		profile, user := resolvePullIdentity(r, "zen2")
		if profile != "zen2" || user != "" {
			t.Fatalf("got (%q,%q), want (zen2,\"\")", profile, user)
		}
	})

	t.Run("auth context, request profile wins", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/v2/x/manifests/latest", nil)
		ctx := contextWithOCIAuth(r.Context(), &OCIAuthContext{Username: "alice", Profile: "zen3", Role: roleUser})
		r = r.WithContext(ctx)
		profile, user := resolvePullIdentity(r, "zen2")
		if profile != "zen2" || user != "alice" {
			t.Fatalf("got (%q,%q), want (zen2,alice)", profile, user)
		}
	})

	t.Run("auth context, empty request profile falls back to auth profile", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/v2/x/manifests/latest", nil)
		ctx := contextWithOCIAuth(r.Context(), &OCIAuthContext{Username: "bob", Profile: "gpu", Role: roleUser})
		r = r.WithContext(ctx)
		profile, user := resolvePullIdentity(r, "")
		if profile != "gpu" || user != "bob" {
			t.Fatalf("got (%q,%q), want (gpu,bob)", profile, user)
		}
	})
}

func TestRecordPull(t *testing.T) {
	metrics.SetEnabled(true)

	r := httptest.NewRequest(http.MethodGet, "/v2/qnib/ftest/manifests/latest", nil)
	ctx := contextWithOCIAuth(r.Context(), &OCIAuthContext{Username: "alice", Profile: "", Role: roleUser})
	r = r.WithContext(ctx)
	w := pullStatusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	req := types.Request{Repo: "qnib/ftest", Tag: "latest", Digest: "sha256:abc", Profile: "zen2"}

	// Manifest pull records both the pull and the resolution metric.
	recordPull(w, r, req, metrics.KindManifest)
	// Blob pull records only the pull metric.
	recordPull(w, r, types.Request{Repo: "qnib/ftest", Tag: "latest", Profile: "zen2"}, metrics.KindBlob)

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{"metahub_pulls_total", "metahub_manifest_resolution_total"} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics body missing %q after recordPull", want)
		}
	}
}
