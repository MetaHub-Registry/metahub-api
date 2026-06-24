package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSetBuildInfo(t *testing.T) {
	SetBuildInfo("9.9.9-test", "8.8.8-test")
	if got := testutil.ToFloat64(buildInfo.WithLabelValues("9.9.9-test", "8.8.8-test")); got != 1 {
		t.Fatalf("build_info = %v, want 1", got)
	}
}

func TestMiddlewareRecordsRoutePatternNotPath(t *testing.T) {
	SetEnabled(true)
	const pattern = "/test/profiles/{label}"
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, pattern, "200"))

	r := chi.NewRouter()
	r.Use(Middleware)
	r.Get(pattern, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	// Two different concrete paths must collapse onto the same route series.
	for _, p := range []string{"/test/profiles/cpu", "/test/profiles/gpu"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, pattern, "200"))
	if after-before != 2 {
		t.Fatalf("requests_total delta = %v, want 2 (both concrete paths on one route series)", after-before)
	}
	// The concrete path must NOT exist as its own series.
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, "/test/profiles/cpu", "200")); got != 0 {
		t.Fatalf("concrete path series = %v, want 0 (only route pattern is labelled)", got)
	}
}

func TestMiddlewareDisabledIsPassthrough(t *testing.T) {
	const pattern = "/test/disabled/{id}"
	SetEnabled(false)
	t.Cleanup(func() { SetEnabled(true) })

	r := chi.NewRouter()
	r.Use(Middleware)
	r.Get(pattern, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test/disabled/1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (handler still runs when metrics disabled)", rec.Code)
	}
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(http.MethodGet, pattern, "200")); got != 0 {
		t.Fatalf("requests_total = %v, want 0 when disabled", got)
	}
}

func TestUsageObservers(t *testing.T) {
	pullBefore := testutil.ToFloat64(pullsTotal.WithLabelValues(KindManifest, "none", "200"))
	ObservePull(KindManifest, "", http.StatusOK) // empty profile -> "none"
	if got := testutil.ToFloat64(pullsTotal.WithLabelValues(KindManifest, "none", "200")) - pullBefore; got != 1 {
		t.Fatalf("pulls_total delta = %v, want 1", got)
	}

	resBefore := testutil.ToFloat64(manifestResolutionTotal.WithLabelValues(profileNone, ResultHit))
	ObserveManifestResolution("", ResultHit) // empty profile normalised to "none"
	if got := testutil.ToFloat64(manifestResolutionTotal.WithLabelValues(profileNone, ResultHit)) - resBefore; got != 1 {
		t.Fatalf("manifest_resolution none/hit delta = %v, want 1", got)
	}

	authBefore := testutil.ToFloat64(authRequestsTotal.WithLabelValues(AuthOIDC, "zen2", ResultAllow))
	ObserveAuthRequest(AuthOIDC, "zen2", ResultAllow)
	if got := testutil.ToFloat64(authRequestsTotal.WithLabelValues(AuthOIDC, "zen2", ResultAllow)) - authBefore; got != 1 {
		t.Fatalf("auth_requests oidc/zen2/allow delta = %v, want 1", got)
	}

	tokBefore := testutil.ToFloat64(tokensIssuedTotal.WithLabelValues(AuthGodmode, "none"))
	ObserveTokenIssued(AuthGodmode, "") // empty profile -> "none"
	if got := testutil.ToFloat64(tokensIssuedTotal.WithLabelValues(AuthGodmode, "none")) - tokBefore; got != 1 {
		t.Fatalf("tokens_issued godmode delta = %v, want 1", got)
	}
}

func TestStatusOf(t *testing.T) {
	// Plain recorder does not expose Status() -> defaults to 200.
	if got := StatusOf(httptest.NewRecorder()); got != http.StatusOK {
		t.Fatalf("StatusOf(plain) = %d, want 200", got)
	}
}

func TestResultFromStatus(t *testing.T) {
	cases := map[int]string{200: ResultHit, 301: ResultHit, 404: ResultMiss, 500: ResultError, 401: ResultError}
	for status, want := range cases {
		if got := ResultFromStatus(status); got != want {
			t.Errorf("ResultFromStatus(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestHandlerExposesFamilies(t *testing.T) {
	SetBuildInfo("v-handler-test", "ml-handler-test")
	ObservePull(KindBlob, "generic", http.StatusOK)

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"metahub_build_info", "metahub_pulls_total", "go_goroutines"} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics body missing %q", want)
		}
	}
}
