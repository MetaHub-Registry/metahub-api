package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/oauth2"
	"gopkg.in/go-jose/go-jose.v2"
	josejwt "gopkg.in/go-jose/go-jose.v2/jwt"
)

// ---------------------------------------------------------------------------
// mockOIDC — a self-contained in-process OIDC identity provider for tests.
//
// It serves:
//   /.well-known/openid-configuration   — discovery document
//   /jwks                               — JSON Web Key Set (public key)
//   /authorize                          — echoes back as a redirect (unused in unit tests)
//   /token                              — issues a signed id_token + access_token
//
// The /token endpoint accepts any "code" value and issues tokens for
// whichever subject/username were registered via addUser().
// ---------------------------------------------------------------------------

type mockOIDC struct {
	srv  *httptest.Server
	priv *rsa.PrivateKey
	// code -> tokenPayload registered by addUser
	codes map[string]mockTokenPayload
}

type mockTokenPayload struct {
	Subject  string
	Username string
	Email    string
}

// newMockOIDC creates and starts the mock OIDC server.
func newMockOIDC(t *testing.T) *mockOIDC {
	t.Helper()
	m := &mockOIDC{codes: map[string]mockTokenPayload{}}

	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	m.priv = k

	mux := http.NewServeMux()

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwk := jose.JSONWebKey{
			Key:       &m.priv.PublicKey,
			KeyID:     "test-kid",
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		issuer := m.srv.URL
		doc := map[string]interface{}{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/authorize",
			"token_endpoint":                        issuer + "/token",
			"jwks_uri":                              issuer + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	})

	// /token: exchange any registered code for a signed id_token + access_token.
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		payload, ok := m.codes[code]
		if !ok {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		issuer := m.srv.URL

		idTok := m.signJWT(t, issuer, payload.Subject, time.Now().Add(5*time.Minute),
			map[string]interface{}{
				"preferred_username": payload.Username,
				"email":              payload.Email,
				"aud":                []string{"test-client"},
			})
		// The access token is also a signed JWT so the middleware can verify it directly.
		accessTok := m.signJWT(t, issuer, payload.Subject, time.Now().Add(5*time.Minute),
			map[string]interface{}{
				"preferred_username": payload.Username,
				"email":              payload.Email,
				"aud":                []string{"test-client"},
			})

		resp := map[string]interface{}{
			"access_token": accessTok,
			"token_type":   "Bearer",
			"id_token":     idTok,
			"expires_in":   300,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

// addUser registers an authorization code that /token will accept.
func (m *mockOIDC) addUser(code, subject, username, email string) {
	m.codes[code] = mockTokenPayload{Subject: subject, Username: username, Email: email}
}

// signJWT creates a signed RS256 JWT with the given claims.
func (m *mockOIDC) signJWT(t *testing.T, issuer, subject string, exp time.Time, extra map[string]interface{}) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: m.priv},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-kid"),
	)
	require.NoError(t, err)
	claims := josejwt.Claims{
		Issuer:   issuer,
		Subject:  subject,
		Audience: josejwt.Audience{"test-client"},
		Expiry:   josejwt.NewNumericDate(exp),
		IssuedAt: josejwt.NewNumericDate(time.Now()),
	}
	raw, err := josejwt.Signed(sig).Claims(claims).Claims(extra).CompactSerialize()
	require.NoError(t, err)
	return raw
}

// provider builds an OIDCProvider wired to this mock server.
func (m *mockOIDC) provider(t *testing.T) *OIDCProvider {
	t.Helper()
	ctx := gooidc.InsecureIssuerURLContext(context.Background(), m.srv.URL)
	p, err := gooidc.NewProvider(ctx, m.srv.URL)
	require.NoError(t, err)

	verifier := p.Verifier(&gooidc.Config{
		ClientID:        "test-client",
		SkipIssuerCheck: false,
		Now:             time.Now,
	})
	return &OIDCProvider{
		Provider: p,
		Verifier: verifier,
		Config: oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://localhost/callback",
			Endpoint:     p.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
		},
	}
}

// handlers returns a minimal Handlers that has only the OIDC provider set
// (no magazine/SQLite). Use the package-level `h` for tests that need SQLite.
func (m *mockOIDC) handlers(t *testing.T) Handlers {
	return Handlers{oidc: m.provider(t)}
}

// validToken issues a fresh signed JWT for the given user.
func (m *mockOIDC) validToken(t *testing.T, subject, username string) string {
	return m.signJWT(t, m.srv.URL, subject, time.Now().Add(5*time.Minute),
		map[string]interface{}{
			"preferred_username": username,
			"email":              username + "@example.com",
			"aud":                []string{"test-client"},
		})
}

// expiredToken issues a JWT that is already expired.
func (m *mockOIDC) expiredToken(t *testing.T, subject string) string {
	return m.signJWT(t, m.srv.URL, subject, time.Now().Add(-10*time.Minute), nil)
}

// wrongKeyToken issues a JWT signed with a fresh key NOT in the JWKS.
func (m *mockOIDC) wrongKeyToken(t *testing.T, subject string) string {
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: wrongKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-kid"),
	)
	require.NoError(t, err)
	claims := josejwt.Claims{
		Issuer:   m.srv.URL,
		Subject:  subject,
		Audience: josejwt.Audience{"test-client"},
		Expiry:   josejwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		IssuedAt: josejwt.NewNumericDate(time.Now()),
	}
	raw, err := josejwt.Signed(sig).Claims(claims).CompactSerialize()
	require.NoError(t, err)
	return raw
}

// ---------------------------------------------------------------------------
// OIDCAuthMiddleware unit tests
// ---------------------------------------------------------------------------

func TestOIDCAuthMiddleware_MissingHeader(t *testing.T) {
	m := newMockOIDC(t)
	h := m.handlers(t)

	r := chi.NewRouter()
	r.Use(h.OIDCAuthMiddleware())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest("GET", "/protected", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "unauthorized")
}

func TestOIDCAuthMiddleware_ValidToken(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.validToken(t, "user-sub-123", "alice")

	r := chi.NewRouter()
	r.Use(hh.OIDCAuthMiddleware())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := OIDCClaimsFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "user-sub-123", claims.Sub)
		assert.Equal(t, "alice", claims.Username)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOIDCAuthMiddleware_ExpiredToken(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.expiredToken(t, "user-sub-456")

	r := chi.NewRouter()
	r.Use(hh.OIDCAuthMiddleware())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "token expired")
}

func TestOIDCAuthMiddleware_InvalidSignature(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)
	token := m.wrongKeyToken(t, "user-sub-789")

	r := chi.NewRouter()
	r.Use(hh.OIDCAuthMiddleware())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ---------------------------------------------------------------------------
// OIDCLogin unit tests
// ---------------------------------------------------------------------------

func TestOIDCLogin_StateAndVerifierCookieSet(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	req := httptest.NewRequest("GET", "/api/v1/login", nil)
	rr := httptest.NewRecorder()
	hh.OIDCLogin(rr, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)

	var stateCookie, verifierCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		switch c.Name {
		case oidcStateCookie:
			stateCookie = c
		case oidcVerifierCookie:
			verifierCookie = c
		}
	}
	require.NotNil(t, stateCookie, "state cookie missing")
	require.NotNil(t, verifierCookie, "verifier cookie missing")
	assert.NotEmpty(t, stateCookie.Value)
	assert.NotEmpty(t, verifierCookie.Value)
	assert.True(t, stateCookie.HttpOnly)
}

func TestOIDCLogin_RedirectURLContainsRequiredParams(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	req := httptest.NewRequest("GET", "/api/v1/login", nil)
	rr := httptest.NewRecorder()
	hh.OIDCLogin(rr, req)

	loc := rr.Header().Get("Location")
	require.NotEmpty(t, loc)
	assert.Contains(t, loc, "response_type=code")
	assert.Contains(t, loc, "client_id=test-client")
	assert.Contains(t, loc, "code_challenge_method=S256")
	assert.Contains(t, loc, "code_challenge=")
	assert.Contains(t, loc, "state=")
}

// ---------------------------------------------------------------------------
// NewOIDCProvider validation tests (no real network needed for error paths)
// ---------------------------------------------------------------------------

func TestNewOIDCProvider_MissingIssuer(t *testing.T) {
	ctx := context.Background()
	_, err := NewOIDCProvider(ctx, magazine.OIDCCfg{
		ClientID:    "id",
		RedirectURI: "http://localhost/cb",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.issuer")
}

func TestNewOIDCProvider_MissingClientID(t *testing.T) {
	ctx := context.Background()
	_, err := NewOIDCProvider(ctx, magazine.OIDCCfg{
		Issuer:      "https://auth.example.com",
		RedirectURI: "http://localhost/cb",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.clientid")
}

func TestNewOIDCProvider_MissingRedirectURI(t *testing.T) {
	ctx := context.Background()
	_, err := NewOIDCProvider(ctx, magazine.OIDCCfg{
		Issuer:   "https://auth.example.com",
		ClientID: "id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc.redirect_uri")
}

func TestNewOIDCProvider_DiscoveryFailure(t *testing.T) {
	ctx := context.Background()
	// Use a real mock server that returns 404 on /.well-known/openid-configuration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := NewOIDCProvider(ctx, magazine.OIDCCfg{
		Issuer:      srv.URL,
		ClientID:    "id",
		RedirectURI: "http://localhost/cb",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OIDC discovery failed")
}

// ---------------------------------------------------------------------------
// extractRawToken tests
// ---------------------------------------------------------------------------

func TestExtractRawToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"empty header", "", ""},
		{"no space", "BearerTOKEN", ""},
		{"wrong scheme", "Basic dXNlcjpwYXNz", ""},
		{"valid bearer", "Bearer mytoken123", "mytoken123"},
		{"bearer uppercase", "BEARER mytoken123", "mytoken123"},
		{"bearer mixed case", "BeArEr mytoken123", "mytoken123"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			got := extractRawToken(req)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// tokenErrMsg tests
// ---------------------------------------------------------------------------

func TestTokenErrMsg(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected string
	}{
		{"token is expired: some detail", "token expired"},
		{"insufficient_scope for this request", "insufficient_scope"},
		{"some other verification error", "invalid token"},
		{"failed to verify signature", "invalid token"},
	}
	for _, tc := range tests {
		t.Run(tc.errMsg, func(t *testing.T) {
			got := tokenErrMsg(fmt.Errorf("%s", tc.errMsg))
			assert.Equal(t, tc.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// OIDCCallback unit tests (error paths — no SQLite needed)
// ---------------------------------------------------------------------------

func TestOIDCCallback_StateMismatch(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	req := httptest.NewRequest("GET", "/api/v1/callback?code=abc&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "correct-state"})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "verifier-value"})

	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "state mismatch")
}

func TestOIDCCallback_ProviderError(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	state := "mystate"
	req := httptest.NewRequest("GET",
		fmt.Sprintf("/api/v1/callback?error=access_denied&error_description=user+denied&state=%s", state), nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "verifier"})

	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "access_denied")
}

func TestOIDCCallback_MissingStateCookie(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	req := httptest.NewRequest("GET", "/api/v1/callback?code=abc&state=s", nil)
	// no state cookie added

	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "missing state cookie")
}

func TestOIDCCallback_MissingVerifierCookie(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	state := "fixed-state"
	req := httptest.NewRequest("GET", "/api/v1/callback?code=abc&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	// no verifier cookie

	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "missing verifier cookie")
}

func TestOIDCCallback_InvalidCode(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	state := "fixed-state"
	// code "bad-code" is not registered in the mock /token, so exchange fails
	req := httptest.NewRequest("GET", "/api/v1/callback?code=bad-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "any-verifier"})

	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	// token exchange will fail → 500
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// NewOIDCProvider success path tests
// ---------------------------------------------------------------------------

func TestNewOIDCProvider_Success(t *testing.T) {
	m := newMockOIDC(t)
	// Use InsecureIssuerURLContext to allow http:// issuer in test
	ctx := gooidc.InsecureIssuerURLContext(context.Background(), m.srv.URL)
	p, err := NewOIDCProvider(ctx, magazine.OIDCCfg{
		Issuer:       m.srv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "http://localhost/callback",
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.NotNil(t, p.Provider)
	assert.NotNil(t, p.Verifier)
	assert.Equal(t, "test-client", p.Config.ClientID)
	assert.Equal(t, "test-secret", p.Config.ClientSecret)
	assert.Equal(t, "http://localhost/callback", p.Config.RedirectURL)
}

// ---------------------------------------------------------------------------
// oidcClaimsFromToken tests
// ---------------------------------------------------------------------------

func TestOIDCClaimsFromToken_ValidToken(t *testing.T) {
	m := newMockOIDC(t)
	provider := m.provider(t)
	token := m.validToken(t, "sub-123", "alice")
	ctx := context.Background()
	claims, err := provider.oidcClaimsFromToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, "sub-123", claims.Sub)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "alice@example.com", claims.Email)
}

func TestOIDCClaimsFromToken_ExpiredToken(t *testing.T) {
	m := newMockOIDC(t)
	provider := m.provider(t)
	token := m.expiredToken(t, "sub-456")
	ctx := context.Background()
	_, err := provider.oidcClaimsFromToken(ctx, token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestOIDCClaimsFromToken_InvalidSignature(t *testing.T) {
	m := newMockOIDC(t)
	provider := m.provider(t)
	token := m.wrongKeyToken(t, "sub-789")
	ctx := context.Background()
	_, err := provider.oidcClaimsFromToken(ctx, token)
	require.Error(t, err)
}

func TestOIDCClaimsFromToken_FallbackToEmail(t *testing.T) {
	m := newMockOIDC(t)
	provider := m.provider(t)
	// Create a token with no preferred_username, only email
	token := m.signJWT(t, m.srv.URL, "sub-no-username", time.Now().Add(5*time.Minute),
		map[string]interface{}{
			"preferred_username": "",
			"email":              "fallback@example.com",
			"aud":                []string{"test-client"},
		})
	ctx := context.Background()
	claims, err := provider.oidcClaimsFromToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, "fallback@example.com", claims.Username)
	assert.Equal(t, "fallback@example.com", claims.Email)
}

// ---------------------------------------------------------------------------
// OIDCClaimsFromContext tests
// ---------------------------------------------------------------------------

func TestOIDCClaimsFromContext_NilContext(t *testing.T) {
	ctx := context.Background()
	claims, ok := OIDCClaimsFromContext(ctx)
	assert.False(t, ok)
	assert.Nil(t, claims)
}

func TestContextWithClaims_RoundTrip(t *testing.T) {
	original := &OIDCClaims{Sub: "sub-1", Username: "bob", Email: "bob@example.com"}
	ctx := contextWithClaims(context.Background(), original)
	claims, ok := OIDCClaimsFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, original, claims)
}

// ---------------------------------------------------------------------------
// writeJSONError tests
// ---------------------------------------------------------------------------

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, http.StatusForbidden, "access denied")
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "access denied")
}

// ---------------------------------------------------------------------------
// s256Challenge test
// ---------------------------------------------------------------------------

func TestS256Challenge(t *testing.T) {
	// Verify it produces a deterministic non-empty result
	result := s256Challenge("test-verifier")
	assert.NotEmpty(t, result)
	// Same input should produce same output
	assert.Equal(t, result, s256Challenge("test-verifier"))
	// Different input should produce different output
	assert.NotEqual(t, result, s256Challenge("different-verifier"))
}

// ---------------------------------------------------------------------------
// OIDCCallback additional error path tests
// ---------------------------------------------------------------------------

func TestOIDCCallback_NoIDTokenInResponse(t *testing.T) {
	// Create a mock OIDC server that returns tokens without id_token
	m := newMockOIDC(t)
	// Override the /token endpoint to not include id_token
	noIDTokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			resp := map[string]interface{}{
				"access_token": "some-access-token",
				"token_type":   "Bearer",
				"expires_in":   300,
				// no id_token
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Forward everything else to the real mock
		m.srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer noIDTokenSrv.Close()

	ctx := gooidc.InsecureIssuerURLContext(context.Background(), m.srv.URL)
	p, err := gooidc.NewProvider(ctx, m.srv.URL)
	require.NoError(t, err)

	hh := Handlers{
		oidc: &OIDCProvider{
			Provider: p,
			Verifier: p.Verifier(&gooidc.Config{ClientID: "test-client"}),
			Config: oauth2.Config{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURL:  "http://localhost/callback",
				Endpoint: oauth2.Endpoint{
					TokenURL: noIDTokenSrv.URL + "/token",
				},
				Scopes: []string{gooidc.ScopeOpenID},
			},
		},
	}

	state := "test-state"
	req := httptest.NewRequest("GET", "/callback?code=valid-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "verifier"})
	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	body, _ := io.ReadAll(rr.Body)
	assert.Contains(t, string(body), "no id_token")
}

// ---------------------------------------------------------------------------
// OIDCCallback — ID token verification failure
// ---------------------------------------------------------------------------

func TestOIDCCallback_InvalidIDToken(t *testing.T) {
	m := newMockOIDC(t)

	// Create a mock token endpoint that returns a bad id_token
	badTokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			resp := map[string]interface{}{
				"access_token": "some-access-token",
				"token_type":   "Bearer",
				"id_token":     "not-a-valid-jwt",
				"expires_in":   300,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		m.srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer badTokenSrv.Close()

	ctx := gooidc.InsecureIssuerURLContext(context.Background(), m.srv.URL)
	p, err := gooidc.NewProvider(ctx, m.srv.URL)
	require.NoError(t, err)

	hh := Handlers{
		oidc: &OIDCProvider{
			Provider: p,
			Verifier: p.Verifier(&gooidc.Config{ClientID: "test-client"}),
			Config: oauth2.Config{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURL:  "http://localhost/callback",
				Endpoint: oauth2.Endpoint{
					TokenURL: badTokenSrv.URL + "/token",
				},
				Scopes: []string{gooidc.ScopeOpenID},
			},
		},
	}

	state := "test-state"
	req := httptest.NewRequest("GET", "/callback?code=valid-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "verifier"})
	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)
	// Verify should fail on a non-JWT id_token
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// OIDCCallback — empty preferred_username falls back to email
// ---------------------------------------------------------------------------

func TestOIDCCallback_FallbackUsernameToEmail(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	m := newMockOIDC(t)
	// Register a user with empty username, only email
	m.codes["fallback-code"] = mockTokenPayload{
		Subject:  "sub-fallback",
		Username: "",
		Email:    "fallback@example.com",
	}

	hh := handlerWithOIDC(m, t)
	state := "fallback-state"
	req := httptest.NewRequest("GET", "/callback?code=fallback-code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "verifier"})
	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	// Should redirect successfully, using email as username
	require.Equal(t, http.StatusTemporaryRedirect, rr.Code,
		"expected redirect, got: %s", rr.Body.String())

	// Verify user was created with email as username
	sqlUsr, err := hh.mag.UserManagement.GetByUsername(ctx, "fallback@example.com")
	require.NoError(t, err)
	assert.Equal(t, "fallback@example.com", sqlUsr.UserName)
}

// ---------------------------------------------------------------------------
// OIDCLogout — GetByUsername error (user not found)
// ---------------------------------------------------------------------------

func TestOIDCLogout_UserNotFound(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	m := newMockOIDC(t)
	hh := handlerWithOIDC(m, t)

	// Inject claims for a user that doesn't exist in SQLite
	claims := &OIDCClaims{
		Sub:      "sub-nonexistent",
		Username: "nonexistent-user-xyz",
		Email:    "nonexistent@example.com",
	}
	req := httptest.NewRequest("GET", "/api/v1/protected/logout", nil)
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	rr := httptest.NewRecorder()
	hh.OIDCLogout(rr, req)

	// GetByUsername should fail → 500
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	_ = ctx
}

// ---------------------------------------------------------------------------
// OIDCLogout error path tests
// ---------------------------------------------------------------------------

func TestOIDCLogout_NoClaims(t *testing.T) {
	m := newMockOIDC(t)
	hh := m.handlers(t)

	// No claims injected — should return unauthorized
	req := httptest.NewRequest("GET", "/api/v1/protected/logout", nil)
	rr := httptest.NewRecorder()
	hh.OIDCLogout(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
