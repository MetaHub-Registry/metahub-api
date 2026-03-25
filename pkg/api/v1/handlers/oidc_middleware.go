package handlers

import (
	"net/http"
	"strings"
)

// OIDCAuthMiddleware returns a chi-compatible middleware that validates the
// Bearer token in the Authorization header using the OIDC provider's JWKS.
//
// On success the validated OIDCClaims are stored in the request context and
// the next handler is called. On failure HTTP 401 is returned.
func (h Handlers) OIDCAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractRawToken(r)
			if rawToken == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// Try god-mode verification first, then OIDC
			var claims *OIDCClaims
			var err error

			if h.godmode.IsEnabled() {
				gmClaims, gmErr := h.godmode.VerifyToken(rawToken)
				if gmClaims != nil {
					claims = gmClaims.OIDCClaims
				} else {
					err = gmErr
				}
			}

			if claims == nil && h.oidc != nil {
				claims, err = h.oidc.oidcClaimsFromToken(r.Context(), rawToken)
			}

			if claims == nil {
				msg := "invalid token"
				if err != nil {
					msg = tokenErrMsg(err)
				}
				writeJSONError(w, http.StatusUnauthorized, msg)
				return
			}

			ctx := contextWithClaims(r.Context(), claims)
			ctx = contextWithRawToken(ctx, rawToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// tokenErrMsg maps a token verification error to a user-facing message.
func tokenErrMsg(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "token is expired"):
		return "token expired"
	case strings.Contains(s, "insufficient_scope"):
		return "insufficient_scope"
	default:
		return "invalid token"
	}
}
