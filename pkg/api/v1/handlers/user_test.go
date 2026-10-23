package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Unit tests for getUserClaim and getUsername — no containers needed
// ---------------------------------------------------------------------------

func TestGetUserClaim_NoClaims(t *testing.T) {
	ctx := context.Background()
	req := httptest.NewRequest("GET", "/", nil)
	// context has no OIDCClaims → should error
	_, _, err := getUserClaim(ctx, req)
	assert.Error(t, err)
}

func TestGetUsername_NoClaims(t *testing.T) {
	ctx := context.Background()
	req := httptest.NewRequest("GET", "/", nil)
	_, err := h.getUsername(ctx, req)
	assert.Error(t, err)
}

func TestGetUsername_WithClaims(t *testing.T) {
	ctx := context.Background()
	claims := &OIDCClaims{Sub: "sub-xyz", Username: "alice"}
	ctx = contextWithClaims(ctx, claims)
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ctx)

	uname, err := h.getUsername(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "alice", uname)
}

func TestUserGetSession_NoClaims(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/protected/session", nil)
	// no claims in context → handler should return error response
	rr := httptest.NewRecorder()
	h.UserGetSession(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUserGetOne_NoClaims(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/protected/user", nil)
	rr := httptest.NewRecorder()
	h.UserGetOne(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUserPutSecret_NoClaims(t *testing.T) {
	req := httptest.NewRequest("PUT", "/api/v1/protected/user/secret", nil)
	rr := httptest.NewRecorder()
	h.UserPutSecret(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUserDelSecret_NoClaims(t *testing.T) {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "secret1")
	req := httptest.NewRequest("DELETE", "/api/v1/protected/user/secret/secret1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.UserDelSecret(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

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

// TestGetUsername verifies that getUserClaim correctly reads claims from context.
func TestGetUsername(t *testing.T) {
	ctx := context.Background()
	expUsername := "admin"
	expSub := "sub-admin-123"

	claims := &OIDCClaims{
		Sub:      expSub,
		Username: expUsername,
		Email:    "admin@example.com",
	}
	ctx = contextWithClaims(ctx, claims)

	req := httptest.NewRequest("GET", "/api/v1/protected/session", bytes.NewBuffer(nil))
	req = req.WithContext(ctx)

	uname, sub, err := getUserClaim(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expUsername, uname)
	assert.Equal(t, expSub, sub)
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

	userName := "admin"
	adminID, err := br.GetInt("user:" + userName + "#id")
	if err != nil {
		t.Fatal(err)
	}
	h.mag.UserManagement.SetLoggedIn(ctx, adminID)

	claims := &OIDCClaims{Sub: "sub-admin", Username: userName, Email: "admin@example.com"}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(oidcMiddlewareInject(claims))
		r.Get("/api/v1/protected/session", h.UserGetSession)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/session", http.Header{}, nil); status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, resp)
	} else {
		t.Log(resp)
	}
}

func TestUserSecrets(t *testing.T) {
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

	claims := &OIDCClaims{Sub: "sub-admin", Username: userName, Email: "admin@example.com"}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(oidcMiddlewareInject(claims))
		r.Get("/api/v1/protected/session", h.UserGetSession)
		r.Put("/api/v1/protected/user/secret", h.UserPutSecret)
		r.Delete("/api/v1/protected/user/secret/{name}", h.UserDelSecret)
		r.Get("/api/v1/protected/user", h.UserGetOne)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", http.Header{}, nil); status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, resp)
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
	if status, resp := testRequest(t, ts, "PUT", "/api/v1/protected/user/secret", http.Header{}, r1); status != 201 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", http.Header{}, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else if len(u.Secrets) != 2 {
			t.Fatal("Expected 2 secrets")
		}
	}
	// Delete by name
	url := "/api/v1/protected/user/secret/" + s1.Name
	if status, resp := testRequest(t, ts, http.MethodDelete, url, http.Header{}, nil); status != 200 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", http.Header{}, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 1, len(u.Secrets))
		}
	}
	// Put second secret
	s2 := types.Secret{
		Name: "put2",
		Key:  "key2",
	}
	b2 := s2.ToBytes()
	r2 := bytes.NewReader(b2)
	if status, resp := testRequest(t, ts, "PUT", "/api/v1/protected/user/secret", http.Header{}, r2); status != 201 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", http.Header{}, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 2, len(u.Secrets))
		}
	}
	// Delete by name
	url = "/api/v1/protected/user/secret/put2"
	if status, resp := testRequest(t, ts, http.MethodDelete, url, http.Header{}, nil); status != 200 {
		t.Log(status)
		t.Fatalf("%s", resp)
	}
	if status, resp := testRequest(t, ts, "GET", "/api/v1/protected/user", http.Header{}, nil); status != 200 {
		t.Fatalf("%s", resp)
	} else {
		if u, err := sqlite.NewUserFromJSON(resp); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, 1, len(u.Secrets))
		}
	}
}
