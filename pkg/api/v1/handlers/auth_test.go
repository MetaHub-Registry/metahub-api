package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

// ---------------------------------------------------------------------------
// realmForRequest tests
// ---------------------------------------------------------------------------

func TestRealmForRequest(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		proto    string
		expected string
	}{
		{"metahub-registry.org host", "metahub-registry.org", "HTTP/1.1", "https://metahub-registry.org"},
		{"metahub.qnib.org host", "metahub.qnib.org", "HTTP/1.1", "https://metahub.qnib.org"},
		{"HTTP/2.0 unknown host", "custom.example.com", "HTTP/2.0", "https://custom.example.com"},
		{"HTTP/1.1 unknown host", "custom.example.com", "HTTP/1.1", "http://custom.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tc.host
			r.Proto = tc.proto
			got := realmForRequest(r)
			assert.Equal(t, tc.expected, got)
		})
	}
}
