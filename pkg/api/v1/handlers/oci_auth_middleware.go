package handlers

import (
	"context"
	"fmt"
	"net/http"

	gCtx "github.com/gorilla/context"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-core/pkg/helper"
)

const (
	// groupMetahubAdmin is the Authentik group name that grants superuser (admin) role.
	groupMetahubAdmin = "metahub-admin"
)

// OCIAuthContext carries authenticated user identity for OCI registry handlers.
type OCIAuthContext struct {
	Username string // parsed from preferred_username (without profile suffix)
	Profile  string // parsed from "user/profile" convention, defaults to "generic"
	Role     string // "admin" if user is in groupMetahubAdmin, otherwise "user"
}

// roleFromGroups returns "admin" if groups contains groupMetahubAdmin, otherwise "user".
func roleFromGroups(groups []string) string {
	for _, g := range groups {
		if g == groupMetahubAdmin {
			return "admin"
		}
	}
	return "user"
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

// ociClaimsResult bundles verified claims with their resolved role.
type ociClaimsResult struct {
	claims *OIDCClaims
	role   string
	err    error
	reject bool // true means send 401 immediately without further fallback
	msg    string
}

// verifyGodModeToken attempts god-mode JWT verification.
func (h Handlers) verifyGodModeToken(rawToken string) ociClaimsResult {
	if !h.godmode.IsEnabled() {
		return ociClaimsResult{}
	}
	gm, err := h.godmode.VerifyToken(rawToken)
	if gm != nil {
		return ociClaimsResult{claims: gm.OIDCClaims, role: gm.Role}
	}
	return ociClaimsResult{err: err}
}

// verifyEnvelopeToken attempts envelope JWT verification and inner JWKS validation.
// Returns reject=true if the token is definitely an envelope but invalid/expired.
func (h Handlers) verifyEnvelopeToken(ctx context.Context, rawToken string) ociClaimsResult {
	signingKey, keyErr := config.EnvelopeSigningKey()
	if keyErr != nil {
		return ociClaimsResult{}
	}
	rawUsername, innerJWT, envErr := VerifyEnvelopeToken(rawToken, signingKey)
	if envErr != nil {
		if isNotEnvelopeToken(envErr) {
			return ociClaimsResult{} // not an envelope — try next path
		}
		return ociClaimsResult{reject: true, msg: fmt.Sprintf("envelope token invalid: %s", tokenErrMsg(envErr))}
	}
	if h.oidc == nil {
		return ociClaimsResult{}
	}
	inner, innerErr := h.oidc.oidcClaimsFromToken(ctx, innerJWT)
	if innerErr != nil {
		return ociClaimsResult{reject: true, msg: fmt.Sprintf("inner token invalid: %s", tokenErrMsg(innerErr))}
	}
	merged := &OIDCClaims{Sub: inner.Sub, Username: rawUsername, Email: inner.Email, Groups: inner.Groups}
	return ociClaimsResult{claims: merged, role: roleFromGroups(inner.Groups)}
}

// verifyOIDCToken attempts OIDC JWKS verification.
func (h Handlers) verifyOIDCToken(ctx context.Context, rawToken string) ociClaimsResult {
	if h.oidc == nil {
		return ociClaimsResult{}
	}
	claims, err := h.oidc.oidcClaimsFromToken(ctx, rawToken)
	if claims != nil {
		return ociClaimsResult{claims: claims, role: roleFromGroups(claims.Groups)}
	}
	return ociClaimsResult{err: err}
}

// resolveOCIClaims resolves claims from the Bearer token by trying god-mode,
// envelope, and OIDC paths in order. Returns the first successful result.
func (h Handlers) resolveOCIClaims(r *http.Request, rawToken string) ociClaimsResult {
	for _, verify := range []func() ociClaimsResult{
		func() ociClaimsResult { return h.verifyGodModeToken(rawToken) },
		func() ociClaimsResult { return h.verifyEnvelopeToken(r.Context(), rawToken) },
		func() ociClaimsResult { return h.verifyOIDCToken(r.Context(), rawToken) },
	} {
		res := verify()
		if res.claims != nil || res.reject {
			return res
		}
		if res.err != nil {
			return res // propagate first non-nil error as fallback message
		}
	}
	return ociClaimsResult{}
}

// ociAuthHandleRequest is the per-request core of OCIAuthMiddleware.
func (h Handlers) ociAuthHandleRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
	rawToken := extractRawToken(r)
	if rawToken == "" {
		realm := realmForRequest(r)
		det := helper.NewDetail("repository", "", r.Method)
		unauthorized(w, r, realm, det, invalidRequest, missingAuthHeader)
		return
	}

	result := h.resolveOCIClaims(r, rawToken)

	if result.reject {
		realm := realmForRequest(r)
		det := helper.NewDetail("repository", "", r.Method)
		unauthorized(w, r, realm, det, invalidToken, "%s", result.msg)
		return
	}

	if result.claims == nil {
		realm := realmForRequest(r)
		det := helper.NewDetail("repository", "", r.Method)
		msg := "invalid token"
		if result.err != nil {
			msg = tokenErrMsg(result.err)
		}
		unauthorized(w, r, realm, det, invalidToken, "%s", msg)
		return
	}

	username, profile := parseUsernameProfile(result.claims.Username)
	ociAuth := &OCIAuthContext{Username: username, Profile: profile, Role: result.role}
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
}

// OCIAuthMiddleware returns a chi-compatible middleware that validates Bearer
// JWTs on /v2/ routes using the OIDC provider's JWKS verifier.
//
// On success: extracts claims, parses username/profile, and stores OCIAuthContext.
// On failure: returns 401 with WWW-Authenticate challenge header.
func (h Handlers) OCIAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ociAuthHandleRequest(w, r, next)
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
