package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// OCIAuthMiddleware tests
// ---------------------------------------------------------------------------

func TestOCIAuthMiddleware_ValidToken(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.validToken(t, "sub-1", "alice")

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := OCIUserFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "alice", auth.Username)
		assert.Equal(t, "generic", auth.Profile)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOCIAuthMiddleware_ValidTokenWithProfile(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	// Create a token with username/profile
	token := m.signJWT(t, m.srv.URL, "sub-2", time.Now().Add(5*time.Minute),
		map[string]interface{}{
			"preferred_username": "alice/staging",
			"email":              "alice@example.com",
			"aud":                []string{"test-client"},
		})

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := OCIUserFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "alice", auth.Username)
		assert.Equal(t, "staging", auth.Profile)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOCIAuthMiddleware_MissingHeader(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	// Should have WWW-Authenticate challenge header
	assert.Contains(t, rr.Header().Get("WWW-Authenticate"), "Bearer realm=")
}

func TestOCIAuthMiddleware_ExpiredToken(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.expiredToken(t, "sub-expired")

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Header().Get("WWW-Authenticate"), "Bearer realm=")
}

func TestOCIAuthMiddleware_InvalidSignature(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.wrongKeyToken(t, "sub-wrong")

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ---------------------------------------------------------------------------
// OCIHeartbeatHandler test
// ---------------------------------------------------------------------------

func TestOCIHeartbeatHandler(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	req := httptest.NewRequest("GET", "/v2/", nil)
	rr := httptest.NewRecorder()
	hh.OCIHeartbeatHandler(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Equal(t, "{}", string(body))
}

// ---------------------------------------------------------------------------
// OCIUserFromContext test
// ---------------------------------------------------------------------------

func TestOCIUserFromContext_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	_, ok := OCIUserFromContext(req.Context())
	assert.False(t, ok)
}
