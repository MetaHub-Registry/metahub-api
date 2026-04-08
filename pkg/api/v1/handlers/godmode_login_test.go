package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// newGodModeLoginHandlers returns a Handlers value whose godmode field is
// populated via newTestGodMode() (which is defined in godmode_test.go).
// OIDC is nil so AuthConfig reports oidc_enabled=false by default.
func newGodModeLoginHandlers() Handlers {
	return NewHandlers("test", magazine.Magazine{}, nil, newTestGodMode())
}

// newGodModeLoginHandlersNoGodMode returns a Handlers value with god-mode
// explicitly disabled (nil). Used for the "god-mode disabled" scenarios.
func newGodModeLoginHandlersNoGodMode() Handlers {
	return NewHandlers("test", magazine.Magazine{}, nil, nil)
}

// newGodModeLoginHandlersBothEnabled returns a Handlers value with both OIDC
// and god-mode set to non-nil values (OIDC is a stub — we never call it).
func newGodModeLoginHandlersBothEnabled() Handlers {
	return NewHandlers("test", magazine.Magazine{}, &OIDCProvider{}, newTestGodMode())
}

// doLogin is a small helper that posts a JSON body to the GodModeLogin handler
// and returns the response recorder for assertions.
func doLogin(t *testing.T, hh Handlers, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/godmode/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	hh.GodModeLogin(w, r)
	return w
}

// ---------------------------------------------------------------------------
// GodModeLogin tests
// ---------------------------------------------------------------------------

func TestGodModeLogin_Disabled(t *testing.T) {
	hh := newGodModeLoginHandlersNoGodMode()
	w := doLogin(t, hh, `{"username":"oci","password":"oci-pass"}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "god-mode login is not enabled", resp["error"])
}

func TestGodModeLogin_InvalidJSON(t *testing.T) {
	hh := newGodModeLoginHandlers()

	tests := []struct {
		name string
		body string
	}{
		{"empty body", ``},
		{"not json", `not-json`},
		{"array instead of object", `[]`},
		{"unknown field", `{"foo":"bar"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doLogin(t, hh, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "invalid request body", resp["error"])
		})
	}
}

func TestGodModeLogin_MissingFields(t *testing.T) {
	hh := newGodModeLoginHandlers()

	tests := []struct {
		name string
		body string
	}{
		{"missing username", `{"password":"oci-pass"}`},
		{"missing password", `{"username":"oci"}`},
		{"both empty strings", `{"username":"","password":""}`},
		{"empty username", `{"username":"","password":"oci-pass"}`},
		{"empty password", `{"username":"oci","password":""}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doLogin(t, hh, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "invalid request body", resp["error"])
		})
	}
}

func TestGodModeLogin_UnknownUser(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := doLogin(t, hh, `{"username":"nobody","password":"whatever"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid credentials", resp["error"])
}

func TestGodModeLogin_WrongPassword(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := doLogin(t, hh, `{"username":"oci","password":"wrong"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid credentials", resp["error"])
}

func TestGodModeLogin_Success(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := doLogin(t, hh, `{"username":"oci","password":"oci-pass"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp godModeLoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token, "token should be set on success")

	// The returned token must be a valid god-mode JWT accepted by VerifyToken.
	claims, err := hh.godmode.VerifyToken(resp.Token)
	require.NoError(t, err)
	assert.Equal(t, "oci", claims.Username)
	assert.Equal(t, "oci", claims.Sub)
	assert.Equal(t, "admin", claims.Role)
}

func TestGodModeLogin_SuccessForSecondUser(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := doLogin(t, hh, `{"username":"test","password":"test-pass"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var resp godModeLoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token)
	claims, err := hh.godmode.VerifyToken(resp.Token)
	require.NoError(t, err)
	assert.Equal(t, "test", claims.Username)
}

// TestGodModeLogin_UpsertsAndMarksLoggedIn exercises the SQLite upsert path:
// it builds a Handlers value re-using the package-shared `h` (which has a
// real test sqlite via TestMain) and confirms that after a successful
// god-mode login the user exists in the DB and `is_logged_in == 1`. This
// mirrors the behaviour of the OIDC callback handler so that the
// `/api/v1/protected/*` routes (which check IsLoggedIn) work for both auth
// flows.
func TestGodModeLogin_UpsertsAndMarksLoggedIn(t *testing.T) {
	ctx := context.Background()
	gm := newTestGodMode()
	hh := NewHandlers("test", h.mag, nil, gm)

	// Sanity: the test sqlite store should be wired up.
	require.NotEmpty(t, hh.mag.UserManagement.MhFile, "test sqlite must be initialised")

	// Pick a username that does not exist yet so we exercise the insert path.
	const uname = "godmode-upsert-test"
	exists, err := hh.mag.UserManagement.UserNameExists(ctx, uname)
	require.NoError(t, err)
	if exists {
		// Clean slate — should not happen but be defensive.
		t.Skipf("user %q already exists in test DB", uname)
	}

	// Inject a fresh god-mode user into the helper so CheckCredentials passes.
	gm.users[uname] = godModeUserInfo{password: "secret-pass", role: "admin"}
	defer delete(gm.users, uname)

	w := doLogin(t, hh, `{"username":"`+uname+`","password":"secret-pass"}`)
	require.Equal(t, http.StatusOK, w.Code)

	exists, err = hh.mag.UserManagement.UserNameExists(ctx, uname)
	require.NoError(t, err)
	require.True(t, exists, "user should be inserted by GodModeLogin")

	sqlUsr, err := hh.mag.UserManagement.GetByUsername(ctx, uname)
	require.NoError(t, err)
	assert.Equal(t, 1, sqlUsr.IsLoggedIn, "user should be marked logged in")
	assert.Equal(t, "admin", sqlUsr.Role)

	// A second login on the same user should still work (the UPSERT path
	// doesn't InsertUser this time, only SetLoggedIn).
	w2 := doLogin(t, hh, `{"username":"`+uname+`","password":"secret-pass"}`)
	require.Equal(t, http.StatusOK, w2.Code)
}

// TestGodModeLogin_NoContentType covers the case where the client omits
// Content-Type but sends a valid JSON body. We still accept the body — the
// contract is "valid JSON body wins" — this protects us from overly strict
// clients accidentally locking users out.
func TestGodModeLogin_NoContentType(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/godmode/login",
		bytes.NewBufferString(`{"username":"oci","password":"oci-pass"}`),
	)
	// No Content-Type header set.
	hh.GodModeLogin(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// AuthConfig tests
// ---------------------------------------------------------------------------

func TestAuthConfig_GodModeOnly(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	hh.AuthConfig(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp authConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OIDCEnabled)
	assert.True(t, resp.GodModeEnabled)
}

func TestAuthConfig_NeitherEnabled(t *testing.T) {
	hh := newGodModeLoginHandlersNoGodMode()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	hh.AuthConfig(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp authConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OIDCEnabled)
	assert.False(t, resp.GodModeEnabled)
}

func TestAuthConfig_OIDCOnly(t *testing.T) {
	hh := NewHandlers("test", magazine.Magazine{}, &OIDCProvider{}, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	hh.AuthConfig(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp authConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OIDCEnabled)
	assert.False(t, resp.GodModeEnabled)
}

func TestAuthConfig_BothEnabled(t *testing.T) {
	hh := newGodModeLoginHandlersBothEnabled()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	hh.AuthConfig(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp authConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OIDCEnabled)
	assert.True(t, resp.GodModeEnabled)
}

// TestAuthConfig_IsPublic confirms the endpoint works without an Authorization
// header (i.e., it is not behind any auth middleware). The router layer decides
// whether the middleware is applied, but the handler itself must not require
// any claims from context.
func TestAuthConfig_IsPublic(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	// No Authorization header.
	hh.AuthConfig(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// upsertGodModeUser unit tests (using a stub store)
// ---------------------------------------------------------------------------

// stubUserStore is a configurable in-memory implementation of the
// godModeUserStore interface used to exercise every error branch of
// upsertGodModeUser without standing up a real SQLite database.
type stubUserStore struct {
	existsResult bool
	existsErr    error
	insertID     int64
	insertErr    error
	getResult    *sqlite.User
	getErr       error
	setErr       error

	insertedUser *types.User
	setLoggedID  int
	calls        []string
}

func (s *stubUserStore) UserNameExists(_ context.Context, _ string) (bool, error) {
	s.calls = append(s.calls, "exists")
	return s.existsResult, s.existsErr
}
func (s *stubUserStore) InsertUser(_ context.Context, u types.User) (int64, error) {
	s.calls = append(s.calls, "insert")
	s.insertedUser = &u
	return s.insertID, s.insertErr
}
func (s *stubUserStore) GetByUsername(_ context.Context, _ string) (*sqlite.User, error) {
	s.calls = append(s.calls, "get")
	return s.getResult, s.getErr
}
func (s *stubUserStore) SetLoggedIn(_ context.Context, id int) error {
	s.calls = append(s.calls, "setloggedin")
	s.setLoggedID = id
	return s.setErr
}

func TestUpsertGodModeUser_NewUserHappyPath(t *testing.T) {
	store := &stubUserStore{
		existsResult: false,
		insertID:     42,
		getResult:    &sqlite.User{ID: 42, UserName: "alice"},
	}
	gm := newTestGodMode()
	err := upsertGodModeUser(context.Background(), store, gm, "alice")
	require.NoError(t, err)
	assert.Equal(t, []string{"exists", "insert", "get", "setloggedin"}, store.calls)
	require.NotNil(t, store.insertedUser)
	assert.Equal(t, "alice", store.insertedUser.Username)
	assert.Equal(t, "alice@godmode", store.insertedUser.Email)
	assert.Equal(t, 1, store.insertedUser.Active)
	assert.Equal(t, 42, store.setLoggedID)
}

func TestUpsertGodModeUser_ExistingUserHappyPath(t *testing.T) {
	store := &stubUserStore{
		existsResult: true,
		getResult:    &sqlite.User{ID: 7, UserName: "oci"},
	}
	gm := newTestGodMode()
	err := upsertGodModeUser(context.Background(), store, gm, "oci")
	require.NoError(t, err)
	// existing user → no insert call
	assert.Equal(t, []string{"exists", "get", "setloggedin"}, store.calls)
	assert.Nil(t, store.insertedUser)
	assert.Equal(t, 7, store.setLoggedID)
}

func TestUpsertGodModeUser_ExistsError(t *testing.T) {
	store := &stubUserStore{existsErr: errors.New("db down")}
	err := upsertGodModeUser(context.Background(), store, newTestGodMode(), "alice")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
	assert.Equal(t, []string{"exists"}, store.calls)
}

func TestUpsertGodModeUser_InsertError(t *testing.T) {
	store := &stubUserStore{
		existsResult: false,
		insertErr:    errors.New("constraint violated"),
	}
	err := upsertGodModeUser(context.Background(), store, newTestGodMode(), "alice")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "constraint violated")
	assert.Equal(t, []string{"exists", "insert"}, store.calls)
}

func TestUpsertGodModeUser_GetError(t *testing.T) {
	store := &stubUserStore{
		existsResult: true,
		getErr:       errors.New("user vanished"),
	}
	err := upsertGodModeUser(context.Background(), store, newTestGodMode(), "alice")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user vanished")
	assert.Equal(t, []string{"exists", "get"}, store.calls)
}

func TestUpsertGodModeUser_SetLoggedInError(t *testing.T) {
	store := &stubUserStore{
		existsResult: true,
		getResult:    &sqlite.User{ID: 1, UserName: "alice"},
		setErr:       errors.New("write failed"),
	}
	err := upsertGodModeUser(context.Background(), store, newTestGodMode(), "alice")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
	assert.Equal(t, []string{"exists", "get", "setloggedin"}, store.calls)
}

func TestUpsertGodModeUser_NilGodModeAuth(t *testing.T) {
	// When godmode auth is nil (defensive: shouldn't happen in practice),
	// the helper should still work and default the role to "admin".
	store := &stubUserStore{
		existsResult: false,
		insertID:     1,
		getResult:    &sqlite.User{ID: 1, UserName: "alice"},
	}
	err := upsertGodModeUser(context.Background(), store, nil, "alice")
	require.NoError(t, err)
	require.NotNil(t, store.insertedUser)
	assert.Equal(t, "admin", store.insertedUser.Role)
}

// ---------------------------------------------------------------------------
// godModeLoginInternal tests (cover the handler-level upsert error path)
// ---------------------------------------------------------------------------

// doInternalLogin posts a JSON body to godModeLoginInternal with the provided
// store stub. Returns the response recorder for assertions.
func doInternalLogin(t *testing.T, hh Handlers, store godModeUserStore, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/godmode/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	hh.godModeLoginInternal(w, r, store)
	return w
}

func TestGodModeLoginInternal_UpsertFailureReturns500(t *testing.T) {
	hh := newGodModeLoginHandlers()
	store := &stubUserStore{existsErr: errors.New("db down")}

	w := doInternalLogin(t, hh, store, `{"username":"oci","password":"oci-pass"}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user upsert failed", resp["error"])
	// The handler should have stopped before token issuance.
	assert.NotContains(t, resp, "token")
}

func TestGodModeLoginInternal_UpsertSuccessReturnsToken(t *testing.T) {
	hh := newGodModeLoginHandlers()
	store := &stubUserStore{
		existsResult: false,
		insertID:     99,
		getResult:    &sqlite.User{ID: 99, UserName: "oci"},
	}

	w := doInternalLogin(t, hh, store, `{"username":"oci","password":"oci-pass"}`)
	require.Equal(t, http.StatusOK, w.Code)

	var resp godModeLoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token)

	// Sanity: the upsert path was actually exercised.
	assert.Equal(t, []string{"exists", "insert", "get", "setloggedin"}, store.calls)
	assert.Equal(t, 99, store.setLoggedID)
}

func TestGodModeLoginInternal_NilStoreSkipsUpsert(t *testing.T) {
	hh := newGodModeLoginHandlers()
	w := doInternalLogin(t, hh, nil, `{"username":"oci","password":"oci-pass"}`)
	require.Equal(t, http.StatusOK, w.Code)
	var resp godModeLoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token)
}

// TestGodModeLogin_DispatchesToInternal verifies that the public handler
// correctly forwards to the internal one — covering the small dispatch
// branch added when refactoring.
func TestGodModeLogin_DispatchesToInternal(t *testing.T) {
	hh := newGodModeLoginHandlers() // empty mag → store will be nil
	w := doLogin(t, hh, `{"username":"oci","password":"oci-pass"}`)
	require.Equal(t, http.StatusOK, w.Code)
	var resp godModeLoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token)
}
