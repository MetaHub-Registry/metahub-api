package handlers

import (
	"context"
	"fmt"
	"net/http"

	gCtx "github.com/gorilla/context"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-core/pkg/helper"
)

const (
	// groupMetahubAdmin is the Authentik group name that grants superuser (admin) role.
	groupMetahubAdmin = "metahub-admin"
	// roleAdmin is the role required to perform registry write operations.
	roleAdmin = "admin"
	// roleUser is the default role for authenticated, non-admin (pull-only) users.
	roleUser = "user"
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
			return roleAdmin
		}
	}
	return roleUser
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
	method string // auth method that produced the claims (godmode|oidc), for metrics
}

// authMethodOrUnknown returns a bounded auth-method label, defaulting to
// "unknown" when no verifier identified the token.
func authMethodOrUnknown(m string) string {
	if m == "" {
		return metrics.AuthUnknown
	}
	return m
}

// verifyGodModeToken attempts god-mode JWT verification.
func (h Handlers) verifyGodModeToken(rawToken string) ociClaimsResult {
	if !h.godmode.IsEnabled() {
		return ociClaimsResult{}
	}
	gm, err := h.godmode.VerifyToken(rawToken)
	if gm != nil {
		return ociClaimsResult{claims: gm.OIDCClaims, role: gm.Role, method: metrics.AuthGodmode}
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
	return ociClaimsResult{claims: merged, role: roleFromGroups(inner.Groups), method: metrics.AuthOIDC}
}

// verifyOIDCToken attempts OIDC JWKS verification.
func (h Handlers) verifyOIDCToken(ctx context.Context, rawToken string) ociClaimsResult {
	if h.oidc == nil {
		return ociClaimsResult{}
	}
	claims, err := h.oidc.oidcClaimsFromToken(ctx, rawToken)
	if claims != nil {
		return ociClaimsResult{claims: claims, role: roleFromGroups(claims.Groups), method: metrics.AuthOIDC}
	}
	return ociClaimsResult{err: err}
}

// resolveOCIClaims resolves claims from the Bearer token by trying god-mode,
// envelope, and OIDC paths in order. Returns the first successful result.
//
// Each verifier reports "not my kind of token" as a non-nil err with nil claims
// and reject=false (e.g. god-mode returns "not a god-mode token" for a real
// OIDC JWT). Such a soft error must NOT short-circuit the loop — otherwise the
// first verifier (god-mode) would reject every OIDC token before the OIDC path
// runs. We remember the first soft error only as a fallback message and keep
// trying the remaining verifiers; a hard reject (reject=true) still stops early.
func (h Handlers) resolveOCIClaims(r *http.Request, rawToken string) ociClaimsResult {
	var firstErr error
	for _, verify := range []func() ociClaimsResult{
		func() ociClaimsResult { return h.verifyGodModeToken(rawToken) },
		func() ociClaimsResult { return h.verifyEnvelopeToken(r.Context(), rawToken) },
		func() ociClaimsResult { return h.verifyOIDCToken(r.Context(), rawToken) },
	} {
		res := verify()
		if res.claims != nil || res.reject {
			return res
		}
		if res.err != nil && firstErr == nil {
			firstErr = res.err // keep as fallback, but keep trying other verifiers
		}
	}
	return ociClaimsResult{err: firstErr}
}

// ociAuthHandleRequest is the per-request core of OCIAuthMiddleware.
func (h Handlers) ociAuthHandleRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
	rawToken := extractRawToken(r)
	if rawToken == "" {
		metrics.ObserveAuthRequest(metrics.AuthAnonymous, "", metrics.ResultDeny)
		realm := realmForRequest(r)
		det := helper.NewDetail("repository", "", r.Method)
		unauthorized(w, r, realm, det, invalidRequest, missingAuthHeader)
		return
	}

	result := h.resolveOCIClaims(r, rawToken)

	if result.reject {
		metrics.ObserveAuthRequest(authMethodOrUnknown(result.method), "", metrics.ResultDeny)
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
		metrics.ObserveAuthRequest(authMethodOrUnknown(result.method), "", metrics.ResultDeny)
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

	metrics.ObserveAuthRequest(authMethodOrUnknown(result.method), profile, metrics.ResultAllow)
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

// isOCIWriteMethod reports whether the HTTP method mutates registry state
// (manifest/blob upload or delete). Reads (GET/HEAD) are not writes.
func isOCIWriteMethod(method string) bool {
	switch method {
	case http.MethodPut, http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// OCIWriteAuthMiddleware enforces role-based authorization for registry writes.
// Any authenticated user may pull (GET/HEAD); only users with the "admin" role
// may push or delete (PUT/POST/PATCH/DELETE). This is what makes a demo /
// pull-only account real: without it, every authenticated user could push.
//
// It must run AFTER OCIAuthMiddleware so the OCIAuthContext (with the resolved
// role) is present on the request context.
func (h Handlers) OCIWriteAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isOCIWriteMethod(r.Method) {
				auth, ok := OCIUserFromContext(r.Context())
				if !ok || auth.Role != roleAdmin {
					det := helper.NewDetail("repository", "", r.Method)
					forbidden(w, r, det, "write access requires the admin role")
					return
				}
			}
			next.ServeHTTP(w, r)
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
