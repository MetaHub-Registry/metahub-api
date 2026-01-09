package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestUserLogin(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	ctx := context.Background()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual Tests
	adminPwd, err := br.Get("user:admin#password")
	if err != nil {
		t.Fatal(err)
	}
	// Login User
	if err := h.mag.UserManagement.LoginUser(ctx, "admin", adminPwd); err != nil {
		t.Fatal(err)
	}
	// FAILED LOGIN
	err = h.mag.UserManagement.LoginUser(ctx, "admin", "FAKE")
	if err == nil {
		t.Fatal("Should have failed")
	}
}

/* TODO: Fix test by injecting bearer token... */
func TestGetUsername(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	ctx := context.Background()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	// Actual Tests
	expUsername := "admin"
	expGHID := "123"
	url := "/api/v1/protected/session"
	tkn, err := h.CreateJWT(ctx, expUsername, expGHID)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", url, bytes.NewBuffer(nil))
	req.Header.Add("Accept", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "jwt", &tkn))
	c := req.Context()
	_, claims, err := jwtauth.FromContext(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Claims: %v", claims)
}

func TestUserGetSession(t *testing.T) {
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
	br.Add("sqfile", h.mag.UserManagement.MhFile)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs, br)
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
		r.Get("/api/v1/protected/session", h.UserGetSession)
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
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/session", hdr, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		t.Log(resp)
	}
}

func TestUserSecrets(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.SetSqlite("Desktop/user_secrets.db")
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
		r.Get("/api/v1/protected/session", h.UserGetSession)
		r.Put("/api/v1/protected/user/secret", h.UserPutSecret)
		r.Delete("/api/v1/protected/user/secret/{id}", h.UserDelSecret)
		r.Get("/api/v1/protected/user", h.UserGetOne)
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
	hdr := newAuthHeader((map[string]interface{}{"user": "admin", "id": "1234", "exp": jwtauth.ExpireIn(5 * time.Minute)}))
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", hdr, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			t.Logf("User: %v", u)
			assert.Equal(t, 1, len(u.Secrets))
		}
	}
	// Put Secret
	s1 := types.Secret{
		Name: "put1",
		Key:  "key1",
	}
	b1 := s1.ToBytes()
	r1 := bytes.NewReader(b1)
	if err != nil {
		t.Fatal(err)
	}
	if status, resp := testRequest(t, ts, "PUT", "/api/v1/protected/user/secret", hdr, r1); status != 201 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", hdr, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else if len(u.Secrets) != 2 {
			t.Fatal("Expected 2 secrets")
		}
	}
	usrSects, err := h.mag.UserManagement.GetSecrets(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range usrSects {
		if s.Name == s1.Name {
			s1.SqlID = s.SqlID
		}
	}
	// Delete Secret
	assert.Greater(t, s1.SqlID, int64(0))
	url := fmt.Sprintf("/api/v1/protected/user/secret/%d", s1.SqlID)
	if status, resp := testRequest(t, ts, http.MethodDelete, url, hdr, nil); status != 200 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", hdr, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 1, len(u.Secrets))
		}
	}
	//
	s2 := types.Secret{
		Name: "put2",
		Key:  "key2",
	}
	b2 := s2.ToBytes()
	r2 := bytes.NewReader(b2)
	if err != nil {
		t.Fatal(err)
	}
	if status, resp := testRequest(t, ts, "PUT", "/api/v1/protected/user/secret", hdr, r2); status != 201 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", hdr, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 2, len(u.Secrets))
		}
	}
	// Delete
	url = "/api/v1/protected/user/secret/put2"
	if status, resp := testRequest(t, ts, http.MethodDelete, url, hdr, nil); status != 200 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", hdr, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 1, len(u.Secrets))
		}
	}
}
