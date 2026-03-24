package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registryTokenTestHandler returns a Handlers instance with mock Authentik API.
func registryTokenTestHandler(t *testing.T) (Handlers, *httptest.Server) {
	t.Helper()
	_, client := newMockAuthentikAPI(t)
	return Handlers{
		oidc: &OIDCProvider{
			AuthentikAPIURL: client.BaseURL,
		},
	}, nil
}

func injectClaimsAndToken(r *http.Request, claims *OIDCClaims, rawToken string) *http.Request {
	ctx := contextWithClaims(r.Context(), claims)
	ctx = contextWithRawToken(ctx, rawToken)
	return r.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// RegistryTokenCreate tests
// ---------------------------------------------------------------------------

func TestRegistryTokenCreate_Success(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	claims := &OIDCClaims{Sub: "sub-1", Username: "alice", Email: "alice@example.com"}

	body := strings.NewReader(`{"description":"my laptop"}`)
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", body)
	req.Host = "registry.example.com"
	req = injectClaimsAndToken(req, claims, "valid-user-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp RegistryTokenCreateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, strings.HasPrefix(resp.Identifier, registryTokenPrefix))
	assert.Equal(t, "my laptop", resp.Description)
	assert.Equal(t, "alice", resp.Username)
	assert.Equal(t, "secret-app-password-key", resp.Password)
	assert.Contains(t, resp.DockerLogin, "docker login")
	assert.Contains(t, resp.DockerLogin, "alice")
}

func TestRegistryTokenCreate_NotConfigured(t *testing.T) {
	hh := Handlers{oidc: nil}
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)
	assert.Equal(t, http.StatusNotImplemented, rr.Code)
}

func TestRegistryTokenCreate_NoClaims(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	// No claims injected
	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRegistryTokenCreate_Forbidden(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	claims := &OIDCClaims{Sub: "sub-1", Username: "alice"}
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	req = injectClaimsAndToken(req, claims, "bad-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ---------------------------------------------------------------------------
// RegistryTokenList tests
// ---------------------------------------------------------------------------

func TestRegistryTokenList_Success(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/protected/registry-tokens", nil)
	req = injectClaimsAndToken(req, &OIDCClaims{Username: "alice"}, "valid-user-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenList(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var items []RegistryTokenListItem
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
	assert.Equal(t, "metahub-registry-111", items[0].Identifier)
}

func TestRegistryTokenList_NotConfigured(t *testing.T) {
	hh := Handlers{oidc: &OIDCProvider{AuthentikAPIURL: ""}}
	req := httptest.NewRequest("GET", "/api/v1/protected/registry-tokens", nil)
	rr := httptest.NewRecorder()
	hh.RegistryTokenList(rr, req)
	assert.Equal(t, http.StatusNotImplemented, rr.Code)
}

// ---------------------------------------------------------------------------
// RegistryTokenRevoke tests
// ---------------------------------------------------------------------------

func TestRegistryTokenRevoke_Success(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req = injectClaimsAndToken(req, &OIDCClaims{Username: "alice"}, "valid-user-token")
			next.ServeHTTP(w, req)
		})
	})
	r.Delete("/api/v1/protected/registry-tokens/{identifier}", hh.RegistryTokenRevoke)
	req := httptest.NewRequest("DELETE", "/api/v1/protected/registry-tokens/metahub-registry-111", nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestRegistryTokenRevoke_InvalidPrefix(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req = injectClaimsAndToken(req, &OIDCClaims{Username: "alice"}, "valid-user-token")
			next.ServeHTTP(w, req)
		})
	})
	r.Delete("/api/v1/protected/registry-tokens/{identifier}", hh.RegistryTokenRevoke)
	req := httptest.NewRequest("DELETE", "/api/v1/protected/registry-tokens/my-api-token", nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "invalid registry token identifier")
}

func TestRegistryTokenCreate_ClaimsButNoRawToken(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	claims := &OIDCClaims{Sub: "sub-1", Username: "alice"}
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	// Inject claims but NOT a raw token
	req = req.WithContext(contextWithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "missing access token")
}

func TestRegistryTokenCreate_AuthentikServerError(t *testing.T) {
	// Create a handler pointing at a mock that returns 500 for token creation
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"internal error"}`, http.StatusInternalServerError)
	}))
	defer errSrv.Close()
	hh := Handlers{oidc: &OIDCProvider{AuthentikAPIURL: errSrv.URL}}
	claims := &OIDCClaims{Sub: "sub-1", Username: "alice"}
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	req = injectClaimsAndToken(req, claims, "valid-user-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)
	assert.Equal(t, http.StatusBadGateway, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "failed to create registry token")
}

func TestRegistryTokenCreate_DefaultDescription(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	claims := &OIDCClaims{Sub: "sub-1", Username: "alice", Email: "alice@example.com"}

	// No body = empty description → should default
	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	req.Host = "registry.example.com"
	req = injectClaimsAndToken(req, claims, "valid-user-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp RegistryTokenCreateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "MetaHub Registry Access", resp.Description)
}

func TestRegistryTokenCreate_EmptyHost(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	claims := &OIDCClaims{Sub: "sub-1", Username: "alice", Email: "alice@example.com"}

	req := httptest.NewRequest("POST", "/api/v1/protected/registry-token", nil)
	req.Host = "" // empty host
	req = injectClaimsAndToken(req, claims, "valid-user-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenCreate(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp RegistryTokenCreateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Contains(t, resp.DockerLogin, "registry.example.com")
}

func TestRegistryTokenList_NoRawToken(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/protected/registry-tokens", nil)
	// no token injected
	rr := httptest.NewRecorder()
	hh.RegistryTokenList(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRegistryTokenList_Forbidden(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/protected/registry-tokens", nil)
	req = injectClaimsAndToken(req, &OIDCClaims{Username: "alice"}, "bad-token")

	rr := httptest.NewRecorder()
	hh.RegistryTokenList(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRegistryTokenRevoke_Forbidden(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req = injectClaimsAndToken(req, &OIDCClaims{Username: "alice"}, "bad-token")
			next.ServeHTTP(w, req)
		})
	})
	r.Delete("/api/v1/protected/registry-tokens/{identifier}", hh.RegistryTokenRevoke)
	req := httptest.NewRequest("DELETE", "/api/v1/protected/registry-tokens/metahub-registry-111", nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRegistryTokenRevoke_NoRawToken(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	r := chi.NewRouter()
	r.Delete("/api/v1/protected/registry-tokens/{identifier}", hh.RegistryTokenRevoke)
	req := httptest.NewRequest("DELETE", "/api/v1/protected/registry-tokens/metahub-registry-111", nil)
	// no token

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRegistryTokenRevoke_NotFound(t *testing.T) {
	hh, _ := registryTokenTestHandler(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req = injectClaimsAndToken(req, &OIDCClaims{Username: "alice"}, "valid-user-token")
			next.ServeHTTP(w, req)
		})
	})
	r.Delete("/api/v1/protected/registry-tokens/{identifier}", hh.RegistryTokenRevoke)
	req := httptest.NewRequest("DELETE", "/api/v1/protected/registry-tokens/metahub-registry-nonexistent", nil)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
