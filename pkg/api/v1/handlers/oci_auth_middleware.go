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
				unauthorized(w, realm, det, invalidRequest, missingAuthHeader)
				return
			}

			claims, err := h.oidc.oidcClaimsFromToken(r.Context(), rawToken)
			if err != nil {
				realm := realmForRequest(r)
				det := helper.NewDetail("repository", "", r.Method)
				msg := tokenErrMsg(err)
				unauthorized(w, realm, det, invalidToken, "%s", msg)
				return
			}

			// Parse username/profile from the preferred_username claim
			username, profile := parseUsernameProfile(claims.Username)

			ociAuth := &OCIAuthContext{
				Username: username,
				Profile:  profile,
				Role:     "user", // Role derivation from Authentik groups is planned for a future change
			}

			ctx := contextWithOCIAuth(r.Context(), ociAuth)

			// Bridge: set gorilla context for backward compatibility with
			// metalib's types.NewRequest which reads from gorilla context.
			gCtx.Set(r, "userName", username)
			gCtx.Set(r, "profileName", profile)
			gCtx.Set(r, "userRole", ociAuth.Role)
			gCtx.Set(r, "accountName", rawToken)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OCIHeartbeatHandler handles GET /v2/ — returns 200 per OCI Distribution Spec.
// This is registered BEFORE the auth middleware so it doesn't require authentication.
func (h Handlers) OCIHeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "{}")
}
