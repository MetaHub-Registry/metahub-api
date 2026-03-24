package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/oauth2"
)

const (
	pkgNameOIDCOAuth  = "handlers/oidc_oauth.go"
	oidcStateCookie   = "oidc_state"
	oidcVerifierCookie = "oidc_verifier"
	cookieMaxAge      = 300 // 5 minutes
)

// randBase64URL returns n random bytes encoded as base64url (no padding).
func randBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// s256Challenge computes the PKCE S256 code challenge from a verifier.
func s256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// OIDCLogin initiates the OIDC Authorization Code + PKCE flow.
// It generates a random state and code_verifier, stores them in short-lived
// cookies, and redirects the user to Authentik's authorization endpoint.
func (h Handlers) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOIDCOAuth,
		"func":    "OIDCLogin",
	})
	defer span.End()
	_ = ctx

	state, err := randBase64URL(32)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to generate state")
		return
	}
	codeVerifier, err := randBase64URL(32)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to generate code verifier")
		return
	}
	codeChallenge := s256Challenge(codeVerifier)

	// Store state and verifier in HttpOnly cookies.
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcVerifierCookie,
		Value:    codeVerifier,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.oidc.Config.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	utils.Debug(l, span, "Redirecting to OIDC authorization URL")
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// OIDCCallback handles the OIDC authorization code callback.
// It validates the state cookie, exchanges the code for tokens,
// verifies the ID token, upserts the user, and redirects the frontend
// with the access token in the URL fragment.
func (h Handlers) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOIDCOAuth,
		"func":    "OIDCCallback",
	})
	defer span.End()

	// Validate state.
	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		utils.Debug(l, span, "state mismatch in OIDC callback")
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}

	// Check for error from provider.
	if oidcErr := r.URL.Query().Get("error"); oidcErr != "" {
		desc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("oidc error: %s — %s", oidcErr, desc), http.StatusBadRequest)
		return
	}

	// Retrieve code verifier from cookie.
	verifierCookie, err := r.Cookie(oidcVerifierCookie)
	if err != nil || verifierCookie.Value == "" {
		http.Error(w, "missing verifier cookie", http.StatusBadRequest)
		return
	}

	// Clear the state and verifier cookies.
	clearCookie(w, oidcStateCookie)
	clearCookie(w, oidcVerifierCookie)

	// Exchange code for tokens.
	code := r.URL.Query().Get("code")
	token, err := h.oidc.Config.Exchange(
		ctx, code,
		oauth2.SetAuthURLParam("code_verifier", verifierCookie.Value),
	)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "token exchange failed")
		return
	}

	// Extract and verify the raw ID token.
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusInternalServerError)
		return
	}
	idToken, err := h.oidc.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "id_token verification failed")
		return
	}

	// Extract claims.
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Sub               string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to extract claims")
		return
	}
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	utils.Debug(l, span, "OIDC callback: user=%s sub=%s", username, claims.Sub)

	// Upsert user in SQLite.
	usr := types.User{
		Username:  username,
		OidcSub:   claims.Sub,
		Email:     claims.Email,
		Active:    1,
	}
	exists, err := h.mag.UserManagement.UserNameExists(ctx, username)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to check user existence")
		return
	}
	if !exists {
		if _, err := h.mag.UserManagement.InsertUser(ctx, usr); err != nil {
			utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to insert user")
			return
		}
	}

	sqlUsr, err := h.mag.UserManagement.GetByUsername(ctx, username)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user")
		return
	}
	if err := h.mag.UserManagement.SetLoggedIn(ctx, sqlUsr.ID); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to set user logged in")
		return
	}

	// Redirect frontend with access_token in the URL fragment.
	accessToken := token.AccessToken
	redirectURL := fmt.Sprintf("%s/profile#access_token=%s", h.mag.Config.RedirectDoc, accessToken)
	utils.Debug(l, span, "Redirecting to frontend after successful login")
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// OIDCLogout marks the user as logged out and clears OIDC cookies.
// The route must be protected by OIDCAuthMiddleware.
func (h Handlers) OIDCLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOIDCOAuth,
		"func":    "OIDCLogout",
	})
	defer span.End()

	claims, ok := OIDCClaimsFromContext(r.Context())
	if !ok {
		utils.ErrorHandlerPlainWWW(w, l, span, fmt.Errorf("no claims"), "UNAUTHORIZED", "not authenticated")
		return
	}

	sqlUser, err := h.mag.UserManagement.GetByUsername(ctx, claims.Username)
	if err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to get user '%s'", claims.Username)
		return
	}
	if err := h.mag.UserManagement.SetLoggedOut(ctx, sqlUser.ID); err != nil {
		utils.ErrorHandlerPlainWWW(w, l, span, err, "INTERNAL_ERROR", "failed to log out user '%s'", claims.Username)
		return
	}
	clearCookie(w, oidcStateCookie)
	clearCookie(w, oidcVerifierCookie)

	utils.Info(l, span, "Logged out user '%s'", claims.Username)
	w.WriteHeader(http.StatusOK)
}

// clearCookie sets a cookie's MaxAge to -1 to instruct the browser to delete it.
func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
}
