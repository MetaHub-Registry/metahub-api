package handlers

import (
	"context"
	"fmt"
	"net/http"

	gCtx "github.com/gorilla/context"
	"github.com/metahub-registry/metahub-core/pkg/helper"
)

// OCIAuthContext carries authenticated user identity for OCI registry handlers.
type OCIAuthContext struct {
	Username string // parsed from preferred_username (without profile suffix)
	Profile  string // parsed from "user/profile" convention, defaults to "generic"
	Role     string // "user" for now; future: derive from Authentik groups
}

// ociAuthCtxKey is the typed context key for OCIAuthContext.
type ociAuthCtxKey struct{}

// contextWithOCIAuth stores OCIAuthContext in the request context.
func contextWithOCIAuth(ctx context.Context, auth *OCIAuthContext) context.Context {
	return context.WithValue(ctx, ociAuthCtxKey{}, auth)
}

// OCIUserFromContext retrieves the OCIAuthContext from the request context.
// Returns nil, false if not present.
func OCIUserFromContext(ctx context.Context) (*OCIAuthContext, bool) {
	v, ok := ctx.Value(ociAuthCtxKey{}).(*OCIAuthContext)
	return v, ok && v != nil
}

// OCIAuthMiddleware returns a chi-compatible middleware that validates Bearer
// JWTs on /v2/ routes using the OIDC provider's JWKS verifier.
//
// On success: extracts claims, parses username/profile, and stores OCIAuthContext.
// On failure: returns 401 with WWW-Authenticate challenge header.
func (h Handlers) OCIAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractRawToken(r)
			if rawToken == "" {
				realm := realmForRequest(r)
				det := helper.NewDetail("repository", "", r.Method)
				unauthorized(w, r, realm, det, invalidRequest, missingAuthHeader)
				return
			}

			// Try god-mode verification first, then OIDC
			var claims *OIDCClaims
			var role string
			var err error

			if h.godmode.IsEnabled() {
				gmClaims, gmErr := h.godmode.VerifyToken(rawToken)
				if gmClaims != nil {
					claims = gmClaims.OIDCClaims
					role = gmClaims.Role
				} else {
					err = gmErr
				}
			}

			if claims == nil && h.oidc != nil {
				claims, err = h.oidc.oidcClaimsFromToken(r.Context(), rawToken)
				if claims != nil {
					role = "user" // OIDC users default to "user"; role derivation from Authentik groups planned
				}
			}

			if claims == nil {
				realm := realmForRequest(r)
				det := helper.NewDetail("repository", "", r.Method)
				msg := "invalid token"
				if err != nil {
					msg = tokenErrMsg(err)
				}
				unauthorized(w, r, realm, det, invalidToken, "%s", msg)
				return
			}

			// Parse username/profile from the preferred_username claim
			username, profile := parseUsernameProfile(claims.Username)

			ociAuth := &OCIAuthContext{
				Username: username,
				Profile:  profile,
				Role:     role,
			}

			ctx := contextWithOCIAuth(r.Context(), ociAuth)
			rWithCtx := r.WithContext(ctx)

			// Bridge: set gorilla context on the NEW request for backward
			// compatibility with metalib's types.NewRequest which reads
			// from gorilla context. Must use rWithCtx (not r) because
			// r.WithContext() returns a new *http.Request pointer and
			// gorilla context is keyed by pointer.
			gCtx.Set(rWithCtx, "userName", username)
			gCtx.Set(rWithCtx, "profileName", profile)
			gCtx.Set(rWithCtx, "userRole", ociAuth.Role)
			gCtx.Set(rWithCtx, "accountName", rawToken)

			next.ServeHTTP(w, rWithCtx)
		})
	}
}

// OCIHeartbeatHandler handles GET /v2/ — returns 200 per OCI Distribution Spec.
// This is registered BEFORE the auth middleware so it doesn't require authentication.
func (h Handlers) OCIHeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "{}")
}
