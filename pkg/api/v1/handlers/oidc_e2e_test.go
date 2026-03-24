package handlers

// oidc_e2e_test.go — end-to-end OIDC authentication workflow tests.
//
// These tests exercise the complete authentication chains using:
//   - The package-level `h` (full Handlers with real SQLite/Neo4j/Minio
//     from TestMain) with its `oidc` field replaced by the mock OIDC provider.
//   - The mockOIDC server (defined in oidc_test.go) which serves /token so
//     the callback handler can perform a real code exchange in-process.
//
// Chains exercised:
//   1. OIDCCallback happy path: code exchange → user upserted in SQLite →
//      is_logged_in set → redirect contains access_token in fragment.
//   2. Full round-trip (frontend):
//      OIDCLogin → extract state/verifier → OIDCCallback → extract token →
//      use token as Bearer on /api/v1/protected/session → 200 with session.
//   3. Session after callback:
//      After OIDCCallback the issued access_token works on every protected
//      endpoint via the real OIDCAuthMiddleware (no claim injection).
//   4. CLI bearer auth:
//      A service-account token (signed by the mock JWKS, no login/callback)
//      is sent directly to a protected endpoint → 200.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// handlerWithOIDC returns a copy of the package-level `h` with its oidc field
// replaced by the given provider. This lets us use the real SQLite backend
// while controlling the identity provider.
func handlerWithOIDC(m *mockOIDC, t *testing.T) Handlers {
	t.Helper()
	return Handlers{
		environment: h.environment,
		mag:         h.mag,
		oidc:        m.provider(t),
	}
}

// ---------------------------------------------------------------------------
// 1. OIDCCallback happy path
// ---------------------------------------------------------------------------

func TestOIDCCallback_Success(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	m := newMockOIDC(t)
	// Register a code the mock /token will accept.
	const (
		testCode     = "auth-code-abc123"
		testSubject  = "sub-bob-42"
		testUsername = "bob"
		testEmail    = "bob@example.com"
	)
	m.addUser(testCode, testSubject, testUsername, testEmail)

	hh := handlerWithOIDC(m, t)
	state := "fixed-state-for-test"

	// Build the callback request: state matches cookie, code is registered.
	callbackURL := "/api/v1/callback?code=" + testCode + "&state=" + state
	req := httptest.NewRequest("GET", callbackURL, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "any-verifier"})

	rr := httptest.NewRecorder()
	hh.OIDCCallback(rr, req)

	// Must redirect (307) to the frontend URL.
	require.Equal(t, http.StatusTemporaryRedirect, rr.Code,
		"expected redirect after successful callback, got: %s", rr.Body.String())

	loc := rr.Header().Get("Location")
	require.NotEmpty(t, loc, "Location header must be set")
	assert.Contains(t, loc, "#access_token=", "access_token must appear in URL fragment")

	// User must now exist in SQLite and be logged in.
	sqlUsr, err := hh.mag.UserManagement.GetByUsername(ctx, testUsername)
	require.NoError(t, err, "user should have been upserted")
	assert.Equal(t, testUsername, sqlUsr.UserName)
	assert.Equal(t, testSubject, sqlUsr.OidcSub)
	assert.Equal(t, 1, sqlUsr.IsLoggedIn)
}

// ---------------------------------------------------------------------------
// 2. Full round-trip — login → callback → protected endpoint
// ---------------------------------------------------------------------------

func TestOIDCFullRoundTrip(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	m := newMockOIDC(t)
	const (
		testCode     = "roundtrip-code-xyz"
		testSubject  = "sub-roundtrip-99"
		testUsername = "roundtrip-user"
		testEmail    = "roundtrip@example.com"
	)
	m.addUser(testCode, testSubject, testUsername, testEmail)

	hh := handlerWithOIDC(m, t)

	// --- Step 1: GET /api/v1/login ---
	loginReq := httptest.NewRequest("GET", "/api/v1/login", nil)
	loginRR := httptest.NewRecorder()
	hh.OIDCLogin(loginRR, loginReq)
	require.Equal(t, http.StatusTemporaryRedirect, loginRR.Code)

	// Extract the state from the cookie set by OIDCLogin.
	var stateCookie, verifierCookie *http.Cookie
	for _, c := range loginRR.Result().Cookies() {
		switch c.Name {
		case oidcStateCookie:
			stateCookie = c
		case oidcVerifierCookie:
			verifierCookie = c
		}
	}
	require.NotNil(t, stateCookie, "state cookie must be set by OIDCLogin")
	require.NotNil(t, verifierCookie, "verifier cookie must be set by OIDCLogin")

	// Verify the redirect URL has the right shape.
	loginLoc := loginRR.Header().Get("Location")
	parsedLogin, err := url.Parse(loginLoc)
	require.NoError(t, err)
	assert.Equal(t, stateCookie.Value, parsedLogin.Query().Get("state"),
		"state in redirect URL must match the cookie")

	// --- Step 2: GET /api/v1/callback (simulate provider redirect back) ---
	callbackURL := "/api/v1/callback?code=" + testCode + "&state=" + stateCookie.Value
	callbackReq := httptest.NewRequest("GET", callbackURL, nil)
	callbackReq.AddCookie(stateCookie)
	callbackReq.AddCookie(verifierCookie)

	callbackRR := httptest.NewRecorder()
	hh.OIDCCallback(callbackRR, callbackReq)
	require.Equal(t, http.StatusTemporaryRedirect, callbackRR.Code,
		"callback should redirect: %s", callbackRR.Body.String())

	// Extract the access_token from the redirect fragment.
	cbLoc := callbackRR.Header().Get("Location")
	require.Contains(t, cbLoc, "#access_token=")
	fragment := cbLoc[strings.Index(cbLoc, "#")+1:]
	fragVals, err := url.ParseQuery(fragment)
	require.NoError(t, err)
	accessToken := fragVals.Get("access_token")
	require.NotEmpty(t, accessToken, "access_token must be present in fragment")

	// --- Step 3: use the access_token as Bearer on a protected endpoint ---
	r := chi.NewRouter()
	r.Use(hh.OIDCAuthMiddleware())
	r.Get("/api/v1/protected/session", hh.UserGetSession)

	sessionReq := httptest.NewRequest("GET", "/api/v1/protected/session", nil)
	sessionReq.Header.Set("Authorization", "Bearer "+accessToken)
	sessionRR := httptest.NewRecorder()
	r.ServeHTTP(sessionRR, sessionReq)

	require.Equal(t, http.StatusOK, sessionRR.Code,
		"protected session endpoint should return 200 with valid token: %s", sessionRR.Body.String())

	var session www.Session
	require.NoError(t, json.NewDecoder(sessionRR.Body).Decode(&session))
	assert.Equal(t, testUsername, session.Login)
	assert.Equal(t, testSubject, session.ID)
	_ = ctx
}

// ---------------------------------------------------------------------------
// 3. Session endpoint after callback (same token works multiple times)
// ---------------------------------------------------------------------------

func TestOIDCSessionAfterCallback(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	m := newMockOIDC(t)
	const (
		testCode     = "session-code-456"
		testSubject  = "sub-session-user"
		testUsername = "session-user"
		testEmail    = "session@example.com"
	)
	m.addUser(testCode, testSubject, testUsername, testEmail)

	hh := handlerWithOIDC(m, t)
	state := "session-state"

	// Perform the callback to create + log in the user.
	cbReq := httptest.NewRequest("GET", "/api/v1/callback?code="+testCode+"&state="+state, nil)
	cbReq.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	cbReq.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "verifier"})
	cbRR := httptest.NewRecorder()
	hh.OIDCCallback(cbRR, cbReq)
	require.Equal(t, http.StatusTemporaryRedirect, cbRR.Code,
		"callback failed: %s", cbRR.Body.String())

	// Extract access_token from redirect.
	cbLoc := cbRR.Header().Get("Location")
	fragment := cbLoc[strings.Index(cbLoc, "#")+1:]
	fragVals, _ := url.ParseQuery(fragment)
	accessToken := fragVals.Get("access_token")
	require.NotEmpty(t, accessToken)

	// Build a test server that uses the real middleware chain.
	r := chi.NewRouter()
	r.Use(hh.OIDCAuthMiddleware())
	r.Get("/api/v1/protected/session", hh.UserGetSession)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Hit the session endpoint multiple times with the same token.
	for i := 0; i < 3; i++ {
		hdr := http.Header{"Authorization": {"Bearer " + accessToken}}
		status, body := testRequest(t, ts, "GET", "/api/v1/protected/session", hdr, nil)
		assert.Equal(t, http.StatusOK, status, "attempt %d: %s", i+1, body)

		var session www.Session
		require.NoError(t, json.Unmarshal([]byte(body), &session))
		assert.Equal(t, testUsername, session.Login)
	}
	_ = ctx
}

// ---------------------------------------------------------------------------
// 4. CLI bearer auth — no login/callback, direct service-account token
// ---------------------------------------------------------------------------

func TestOIDCCLIBearerAuth(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	m := newMockOIDC(t)

	// The CLI already has a token; it never goes through the login/callback flow.
	// We need the user to exist in SQLite and be logged in so the session handler
	// can find them. In practice the CLI user would be pre-provisioned.
	const (
		cliSubject  = "sub-cli-svc"
		cliUsername = "cli-admin"
		cliEmail    = "cli@example.com"
	)
	cliUser := types_User(cliUsername, cliSubject, cliEmail)
	_, err = h.mag.UserManagement.InsertUser(ctx, cliUser)
	require.NoError(t, err)
	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, cliUsername)
	require.NoError(t, err)
	require.NoError(t, h.mag.UserManagement.SetLoggedIn(ctx, sqlUsr.ID))

	hh := handlerWithOIDC(m, t)

	// Mint a token signed by the mock JWKS key — no exchange, just direct use.
	cliToken := m.validToken(t, cliSubject, cliUsername)

	r := chi.NewRouter()
	r.Use(hh.OIDCAuthMiddleware())
	r.Get("/api/v1/protected/session", hh.UserGetSession)
	ts := httptest.NewServer(r)
	defer ts.Close()

	hdr := http.Header{"Authorization": {"Bearer " + cliToken}}
	status, body := testRequest(t, ts, "GET", "/api/v1/protected/session", hdr, nil)
	assert.Equal(t, http.StatusOK, status, "CLI bearer auth should succeed: %s", body)

	var session www.Session
	require.NoError(t, json.Unmarshal([]byte(body), &session))
	assert.Equal(t, cliUsername, session.Login)
	assert.Equal(t, cliSubject, session.ID)
	_ = ctx
}

// types_User is a small helper to build a types.User for test setup.
func types_User(username, oidcSub, email string) types.User {
	return types.User{
		Username: username,
		OidcSub:  oidcSub,
		Email:    email,
		Active:   1,
	}
}
