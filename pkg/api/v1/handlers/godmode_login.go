package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const pkgNameGodModeLogin = "handlers/godmode_login.go"

// godModeUserStore is the minimal subset of metalib's user-management API
// needed by the god-mode login upsert path. It exists so tests can stub the
// store without standing up a real SQLite database.
type godModeUserStore interface {
	UserNameExists(ctx context.Context, username string) (bool, error)
	InsertUser(ctx context.Context, u types.User) (int64, error)
	GetByUsername(ctx context.Context, username string) (*sqlite.User, error)
	SetLoggedIn(ctx context.Context, userID int) error
}

// godModeLoginRequest is the JSON body accepted by POST /api/v1/godmode/login.
type godModeLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// godModeLoginResponse is the JSON body returned on successful login.
type godModeLoginResponse struct {
	Token string `json:"token"`
}

// authConfigResponse is the JSON body returned by GET /api/v1/auth/config.
// It reports which authentication methods are currently enabled so that
// browser clients can decide which login flow to present.
type authConfigResponse struct {
	OIDCEnabled    bool `json:"oidc_enabled"`
	GodModeEnabled bool `json:"godmode_enabled"`
}

// GodModeLogin handles POST /api/v1/godmode/login.
//
// It accepts a JSON body `{ "username": "...", "password": "..." }`, validates
// the credentials against the configured `godmode.users`, and returns a signed
// god-mode JWT on success. The token is identical to the one minted by
// `GET /auth` for god-mode credentials and is accepted by both OCIAuthMiddleware
// and OIDCAuthMiddleware.
//
// Responses:
//   - 200 { "token": "<jwt>" }      on success
//   - 400 { "error": "invalid request body" }
//     on missing/malformed body or missing fields
//   - 401 { "error": "invalid credentials" }
//     on unknown username or wrong password
//   - 404 { "error": "god-mode login is not enabled" }
//     when god-mode is disabled in the running server
//   - 500 { "error": "user upsert failed" }
//     when the SQLite user store cannot be updated
func (h Handlers) GodModeLogin(w http.ResponseWriter, r *http.Request) {
	// Pick the user store: if the configured magazine has no SQLite file
	// (typical for unit tests with a bare Handlers value) we pass nil and
	// godModeLoginInternal skips the upsert step.
	var store godModeUserStore
	if h.mag.UserManagement.MhFile != "" {
		store = h.mag.UserManagement
	}
	h.godModeLoginInternal(w, r, store)
}

// godModeLoginInternal is the testable core of GodModeLogin. It takes the
// user store as an explicit dependency so unit tests can inject a stub that
// fails any of the upsert sub-steps without standing up a real SQLite DB.
// A nil store means "skip the user upsert" — used in tests that don't care
// about session bookkeeping.
func (h Handlers) godModeLoginInternal(w http.ResponseWriter, r *http.Request, store godModeUserStore) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameGodModeLogin,
		"func":    "GodModeLogin",
	})
	defer span.End()

	if !h.godmode.IsEnabled() {
		writeJSONError(w, http.StatusNotFound, "god-mode login is not enabled")
		return
	}

	var req godModeLoginRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		utils.Debug(l, span, "god-mode login: invalid JSON body: %v", err)
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !h.godmode.CheckCredentials(req.Username, req.Password) {
		utils.Debug(l, span, "god-mode login: invalid credentials for user '%s'", req.Username)
		writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Upsert the god-mode user in the user store and mark them logged in.
	// This mirrors the OIDC callback handler so that `/api/v1/protected/*`
	// routes (which consult `is_logged_in`) work identically for OIDC and
	// god-mode sessions.
	if store != nil {
		if err := upsertGodModeUser(ctx, store, h.godmode, req.Username); err != nil {
			utils.Warn(l, span, "god-mode login: failed to upsert user '%s': %v", req.Username, err)
			writeJSONError(w, http.StatusInternalServerError, "user upsert failed")
			return
		}
	}

	token, err := h.godmode.MintToken(req.Username)
	if err != nil {
		// MintToken only fails when the GodModeAuth receiver is nil, which
		// IsEnabled() above already rules out. Kept as defensive code.
		utils.Warn(l, span, "god-mode login: token minting failed for user '%s': %v", req.Username, err)
		writeJSONError(w, http.StatusInternalServerError, "token creation failed")
		return
	}

	utils.Info(l, span, "god-mode login: issued token for user '%s'", req.Username)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(godModeLoginResponse{Token: token})
}

// upsertGodModeUser ensures the given username exists in the user store and
// flips its `is_logged_in` flag to true. It mirrors the upsert path used in
// OIDCCallback so protected routes behave identically for both auth methods.
//
// The store is taken as a small interface so tests can swap in a stub that
// fails the various branches without standing up a real SQLite database.
func upsertGodModeUser(ctx context.Context, store godModeUserStore, gm *GodModeAuth, username string) error {
	exists, err := store.UserNameExists(ctx, username)
	if err != nil {
		return err
	}
	if !exists {
		role := "admin"
		if gm != nil {
			role = gm.GetRole(username)
		}
		newUser := types.User{
			Username: username,
			OidcSub:  "", // god-mode users have no OIDC subject
			Email:    username + "@godmode",
			Active:   1,
			Role:     role,
		}
		if _, err := store.InsertUser(ctx, newUser); err != nil {
			return err
		}
	}
	sqlUsr, err := store.GetByUsername(ctx, username)
	if err != nil {
		return err
	}
	return store.SetLoggedIn(ctx, sqlUsr.ID)
}

// AuthConfig handles GET /api/v1/auth/config.
//
// It returns a JSON object describing which authentication methods are enabled
// on this server, so that browser clients can choose the correct login UI:
// OIDC redirect, god-mode form, or both.
//
// The endpoint is public (no auth middleware).
func (h Handlers) AuthConfig(w http.ResponseWriter, r *http.Request) {
	resp := authConfigResponse{
		OIDCEnabled:    h.oidc != nil,
		GodModeEnabled: h.godmode.IsEnabled(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
