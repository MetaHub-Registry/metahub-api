package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
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

func TestOCIAuthMiddleware_AdminGroup(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.signJWT(t, m.srv.URL, "sub-admin", time.Now().Add(5*time.Minute),
		map[string]interface{}{
			"preferred_username": "akadmin",
			"email":              "akadmin@example.com",
			"aud":                []string{"test-client"},
			"groups":             []string{"metahub-admin", "other-group"},
		})

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := OCIUserFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "akadmin", auth.Username)
		assert.Equal(t, "admin", auth.Role)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOCIAuthMiddleware_NonAdminGroup(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.signJWT(t, m.srv.URL, "sub-user", time.Now().Add(5*time.Minute),
		map[string]interface{}{
			"preferred_username": "bob",
			"email":              "bob@example.com",
			"aud":                []string{"test-client"},
			"groups":             []string{"some-other-group"},
		})

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := OCIUserFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "bob", auth.Username)
		assert.Equal(t, "user", auth.Role)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOCIAuthMiddleware_NoGroups(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	// validToken has no groups claim at all
	token := m.validToken(t, "sub-nogroups", "charlie")

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := OCIUserFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "user", auth.Role)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---------------------------------------------------------------------------
// roleFromGroups unit tests
// ---------------------------------------------------------------------------

func TestRoleFromGroups(t *testing.T) {
	assert.Equal(t, "admin", roleFromGroups([]string{"metahub-admin"}))
	assert.Equal(t, "admin", roleFromGroups([]string{"other", "metahub-admin"}))
	assert.Equal(t, "user", roleFromGroups([]string{"other-group"}))
	assert.Equal(t, "user", roleFromGroups([]string{}))
	assert.Equal(t, "user", roleFromGroups(nil))
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

// ---------------------------------------------------------------------------
// Envelope token middleware tests
// ---------------------------------------------------------------------------

// mintEnvelopeForTest mints an envelope token whose inner JWT is signed by
// the mock OIDC server's key, so OCIAuthMiddleware can validate it end-to-end.
func mintEnvelopeForTest(t *testing.T, m *mockOIDC, rawUsername string, signingKey []byte) string {
	t.Helper()
	// The inner JWT must be a valid token verifiable by the mock OIDC JWKS.
	innerJWT := m.signJWT(t, m.srv.URL, "sub-env", time.Now().Add(10*time.Minute),
		map[string]interface{}{
			"preferred_username": rawUsername,
			"email":              rawUsername + "@example.com",
			"aud":                []string{"test-client"},
		})
	env, err := MintEnvelopeToken(rawUsername, innerJWT, signingKey)
	require.NoError(t, err)
	return env
}

func TestOCIAuthMiddleware_EnvelopeToken_CorrectProfile(t *testing.T) {
	const signingKey = "test-envelope-signing-key-at-least-32-chars-long"
	m := newMockOIDC(t)

	// Set envelope signing key in viper so the middleware can resolve it.
	viper.Set("auth.envelope_signing_key", signingKey)
	defer viper.Set("auth.envelope_signing_key", "")

	hh := m.handlers(t)
	token := mintEnvelopeForTest(t, m, "alice/zen2", []byte(signingKey))

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := OCIUserFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "alice", auth.Username)
		assert.Equal(t, "zen2", auth.Profile)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOCIAuthMiddleware_EnvelopeToken_ExpiredInner(t *testing.T) {
	const signingKey = "test-envelope-signing-key-at-least-32-chars-long"
	m := newMockOIDC(t)

	viper.Set("auth.envelope_signing_key", signingKey)
	defer viper.Set("auth.envelope_signing_key", "")

	hh := m.handlers(t)

	// Inner JWT is expired.
	expiredInner := m.signJWT(t, m.srv.URL, "sub-env", time.Now().Add(-10*time.Minute),
		map[string]interface{}{
			"preferred_username": "alice/zen2",
			"email":              "alice@example.com",
			"aud":                []string{"test-client"},
		})

	// Mint an envelope with a fresh exp but expired inner token.
	claims := jwt.MapClaims{
		"iss":                envelopeIssuer,
		"preferred_username": "alice/zen2",
		claimInnerToken:      expiredInner,
		"iat":                time.Now().Unix(),
		"exp":                time.Now().Add(1 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	env, err := tok.SignedString([]byte(signingKey))
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(hh.OCIAuthMiddleware())
	r.Get("/v2/test/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v2/test/manifests/latest", nil)
	req.Header.Set("Authorization", "Bearer "+env)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	// Expired inner token must cause 401.
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOCIAuthMiddleware_EnvelopeToken_BadEnvelopeSignature(t *testing.T) {
	const correctKey = "test-envelope-signing-key-at-least-32-chars-long"
	const wrongKey = "wrong-envelope-signing-key-at-least-32-chars-long"
	m := newMockOIDC(t)

	// Middleware uses the correct key.
	viper.Set("auth.envelope_signing_key", correctKey)
	defer viper.Set("auth.envelope_signing_key", "")

	hh := m.handlers(t)
	// Token signed with wrong key.
	token := mintEnvelopeForTest(t, m, "alice/zen2", []byte(wrongKey))

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
