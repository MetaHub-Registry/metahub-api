package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestGithubLogout(t *testing.T) {
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs)
	r := chi.NewRouter()
	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(TokenAuthHS256))
		authenticator := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token, _, err := jwtauth.FromContext(r.Context())
				if err != nil {
					http.Error(w, jwtauth.ErrorReason(err).Error(), http.StatusUnauthorized)
					return
				}
				if err := jwt.Validate(token); err != nil {
					http.Error(w, jwtauth.ErrorReason(err).Error(), http.StatusUnauthorized)
					return
				}
				// Token is authenticated, pass it through
				next.ServeHTTP(w, r)
			})
		}
		r.Use(authenticator)
		r.Get("/api/v1/protected/logout", h.GitHubLogout)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()
	// Actual tests
	userName := "admin"
	adminID, err := br.GetInt("user:" + userName + "#id")
	if err != nil {
		t.Fatal(err)
	}
	h.mag.UserManagement.SetLoggedIn(ctx, adminID)
	hdr := newAuthHeader((map[string]interface{}{"user": userName, "id": "1234", "exp": jwtauth.ExpireIn(5 * time.Minute)}))
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/logout", hdr, nil); status != 200 {
		t.Fatalf(resp)
	}
	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, userName)
	if err != nil {
		t.Fatal(err)
	}
	if sqlUsr.IsLoggedIn == 1 {
		t.Fatal("User is still logged in")
	}
}

func TestGithubCallback(t *testing.T) {
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
		TeardownTest(bs)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs)
	// Prepare
	h.mag.Config.OAuth.Enabled = false

	r := chi.NewRouter()
	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(TokenAuthHS256))
		authenticator := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token, _, err := jwtauth.FromContext(r.Context())
				if err != nil {
					http.Error(w, jwtauth.ErrorReason(err).Error(), http.StatusUnauthorized)
					return
				}
				if err := jwt.Validate(token); err != nil {
					http.Error(w, jwtauth.ErrorReason(err).Error(), http.StatusUnauthorized)
					return
				}
				// Token is authenticated, pass it through
				next.ServeHTTP(w, r)
			})
		}
		r.Use(authenticator)
		r.Get("/github_oauth_cb", h.GitHubCallback)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()
	// Actual tests
	userName := "admin"
	adminID, err := br.GetInt("user:" + userName + "#id")
	if err != nil {
		t.Fatal(err)
	}
	h.mag.UserManagement.SetLoggedIn(ctx, adminID)
	hdr := http.Header{}
	hdr.Set("User", userName)

	if status, resp := testRequest(t, ts, "GET", "/github_oauth_cb", hdr, nil); status != 401 || resp != "token is unauthorized\n" {
		t.Fatalf(resp)
	}
}
