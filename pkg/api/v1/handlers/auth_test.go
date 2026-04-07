package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseBasicAuth tests
// ---------------------------------------------------------------------------

func TestParseBasicAuth_Valid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth", nil)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:secret123")))
	user, pass, ok := parseBasicAuth(r)
	assert.True(t, ok)
	assert.Equal(t, "alice", user)
	assert.Equal(t, "secret123", pass)
}

func TestParseBasicAuth_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth", nil)
	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_NotBasic(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_InvalidBase64(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth", nil)
	r.Header.Set("Authorization", "Basic !!!invalid!!!")
	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_NoColon(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth", nil)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("nocolonhere")))
	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// parseUsernameProfile tests
// ---------------------------------------------------------------------------

func TestParseUsernameProfile(t *testing.T) {
	tests := []struct {
		input       string
		wantUser    string
		wantProfile string
	}{
		{"alice", "alice", "generic"},
		{"alice/staging", "alice", "staging"},
		{"alice/my-profile", "alice", "my-profile"},
		{"bob@example.com", "bob@example.com", "generic"},
		{"svc.account", "svc.account", "generic"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			user, profile := parseUsernameProfile(tc.input)
			assert.Equal(t, tc.wantUser, user)
			assert.Equal(t, tc.wantProfile, profile)
		})
	}
}

// ---------------------------------------------------------------------------
// AuthHandler (Authentik proxy) tests
// ---------------------------------------------------------------------------

func TestAuthHandler_Anonymous(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth", nil)
	h.AuthHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "REDACTED-IN-HISTORY", resp["token"])
}

func TestAuthHandler_WithCredentials_AuthentikUnreachable(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth?account=alice", nil)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:app-password")))
	h.AuthHandler(w, r)
	// Token exchange will fail since the mock OIDC server doesn't support client_credentials
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// mintFakeAuthentikJWT creates a signed JWT that mimics an Authentik access token.
func mintFakeAuthentikJWT(t *testing.T, username string, exp time.Time, signingKey []byte) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":                "https://authentik.test/",
		"preferred_username": username,
		"exp":                exp.Unix(),
		"iat":                time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(signingKey)
	require.NoError(t, err)
	return s
}

// newHandlerWithMockAuthentikAndKey builds a Handlers instance whose OIDC token URL
// points at a fake httptest.Server that returns the provided JWT.
// It also sets auth.envelope_signing_key in viper for EnvelopeSigningKey() to work.
// The returned cleanup function closes the server and clears viper state.
func newHandlerWithMockAuthentikAndKey(t *testing.T, returnJWT, signingKey string) (Handlers, *httptest.Server, func()) {
	t.Helper()
	fakeAuthentik := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": returnJWT,
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))

	// Patch viper so config.EnvelopeSigningKey() returns our test key.
	viper.Set("auth.envelope_signing_key", signingKey)
	cleanup := func() {
		viper.Set("auth.envelope_signing_key", "")
		fakeAuthentik.Close()
	}

	// Build a minimal OIDCProvider that only needs the TokenURL for ROPC exchange.
	oidcProv := &OIDCProvider{}
	oidcProv.Config.Endpoint.TokenURL = fakeAuthentik.URL + "/token"
	oidcProv.Config.ClientID = "test-client"

	hh := NewHandlers("test", h.mag, oidcProv, nil)
	return hh, fakeAuthentik, cleanup
}

func TestAuthHandler_WithProfile_ReturnsEnvelopeToken(t *testing.T) {
	const signingKey = "test-envelope-signing-key-at-least-32-chars-long"
	fakeJWT := mintFakeAuthentikJWT(t, "alice", time.Now().Add(30*time.Minute), []byte("fake-inner-key"))

	hh, _, cleanup := newHandlerWithMockAuthentikAndKey(t, fakeJWT, signingKey)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth?account=alice%2Fzen2", nil)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice/zen2:password")))
	hh.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	token := resp["token"]
	require.NotEmpty(t, token)

	// The returned token must be an envelope (iss=metahub-profile-token).
	rawUsername, innerJWT, err := VerifyEnvelopeToken(token, []byte(signingKey))
	require.NoError(t, err)
	assert.Equal(t, "alice/zen2", rawUsername)
	assert.Equal(t, fakeJWT, innerJWT)
}

func TestAuthHandler_WithoutProfile_ReturnsAuthentikJWTUnchanged(t *testing.T) {
	const signingKey = "test-envelope-signing-key-at-least-32-chars-long"
	fakeJWT := mintFakeAuthentikJWT(t, "alice", time.Now().Add(30*time.Minute), []byte("fake-inner-key"))

	hh, _, cleanup := newHandlerWithMockAuthentikAndKey(t, fakeJWT, signingKey)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth?account=alice", nil)
	r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:password")))
	hh.AuthHandler(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Without a profile the Authentik JWT is returned directly — not an envelope.
	assert.Equal(t, fakeJWT, resp["token"])
	// Must NOT be an envelope token.
	assert.False(t, strings.Contains(resp["token"], envelopeIssuer), "should not be an envelope token")
}

// ---------------------------------------------------------------------------
// realmForRequest tests
// ---------------------------------------------------------------------------

func TestRealmForRequest(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		proto          string
		forwardedProto string
		expected       string
	}{
		// Hard-coded known hosts always return https regardless of headers.
		{"metahub-registry.org host", "metahub-registry.org", "HTTP/1.1", "", "https://metahub-registry.org"},
		{"metahub.qnib.org host", "metahub.qnib.org", "HTTP/1.1", "", "https://metahub.qnib.org"},
		// X-Forwarded-Proto takes precedence over r.Proto for unknown hosts.
		{"X-Forwarded-Proto http", "custom.example.com", "HTTP/2.0", "http", "http://custom.example.com"},
		{"X-Forwarded-Proto https", "custom.example.com", "HTTP/2.0", "https", "https://custom.example.com"},
		// When no X-Forwarded-Proto is present, fall back to TLS detection.
		// httptest.NewRequest has no TLS so this produces http://.
		{"no header HTTP/2.0 no TLS", "custom.example.com", "HTTP/2.0", "", "http://custom.example.com"},
		{"no header HTTP/1.1 no TLS", "custom.example.com", "HTTP/1.1", "", "http://custom.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tc.host
			r.Proto = tc.proto
			if tc.forwardedProto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}
			got := realmForRequest(r)
			assert.Equal(t, tc.expected, got)
		})
	}
}
