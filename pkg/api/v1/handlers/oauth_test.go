package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// testRequest is a shared test helper that sends an HTTP request to a test server.
func testRequest(t *testing.T, ts *httptest.Server, method, path string, header http.Header, body io.Reader) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
		return 0, ""
	}
	for k, v := range header {
		req.Header.Set(k, v[0])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return 0, ""
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return 0, ""
	}
	defer resp.Body.Close()
	return resp.StatusCode, string(respBody)
}

// oidcMiddlewareInject returns a chi middleware that injects the given OIDCClaims
// into the request context, bypassing actual token validation.
// Use this in tests to exercise handlers that require authenticated context.
func oidcMiddlewareInject(claims *OIDCClaims) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := contextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TestOIDCLogout(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs, br)

	userName := "admin"
	adminID, err := br.GetInt("user:" + userName + "#id")
	if err != nil {
		t.Fatal(err)
	}
	h.mag.UserManagement.SetLoggedIn(ctx, adminID)

	claims := &OIDCClaims{
		Sub:      "sub-admin",
		Username: userName,
		Email:    "admin@example.com",
	}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(oidcMiddlewareInject(claims))
		r.Get("/api/v1/protected/logout", h.OIDCLogout)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/logout", http.Header{}, nil); status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, resp)
	}

	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, userName)
	if err != nil {
		t.Fatal(err)
	}
	if sqlUsr.IsLoggedIn == 1 {
		t.Fatal("User is still logged in after logout")
	}
}
