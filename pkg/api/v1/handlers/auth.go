package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-api/pkg/usage"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type oauthError string

const (
	defaultProfile     = "generic"
	pkgNameAuth        = "api/auth.go"
	realFrmt           = "Realm: %s"
	uknAccTok          = "unknown access token"
	invalidTokenHeader = "invalid authorization header: %q"
	missingAuthHeader  = "missing authorization header"
	contentTypeJSON    = "application/json"
	headerContentType  = "Content-Type"
)

var (
	invalidRequest oauthError = "invalid_request"
	invalidToken   oauthError = "invalid_token"
	userReg                   = regexp.MustCompile(`(?P<user>[\w\-\.\_\@]+)(\/(?P<profile>[\/\w\-\.\_#]+))?`)
	FailGetToken              = fmt.Errorf("failed to get token")
)

// parseBasicAuth extracts username and password from a Basic Authorization header.
func parseBasicAuth(r *http.Request) (username, password string, ok bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "basic") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}
	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return "", "", false
	}
	return creds[0], creds[1], true
}

// parseUsernameProfile parses "user/profile" from a Docker username string.
// Returns username, profile. Profile defaults to "generic" if not present.
func parseUsernameProfile(rawUsername string) (username, profile string) {
	m := userReg.FindStringSubmatch(rawUsername)
	if m == nil {
		return rawUsername, defaultProfile
	}
	names := userReg.SubexpNames()
	result := make(map[string]string)
	for i, name := range names {
		if i != 0 && name != "" {
			result[name] = m[i]
		}
	}
	username = result["user"]
	profile = result["profile"]
	if profile == "" {
		profile = defaultProfile
	}
	return username, profile
}

// resolveProfileFromInventory consults the inventory service for suffix-less
// logins (openspec change hostname-profile-resolution). metahub-api is a thin
// consumer — the inventory service (metahub-admin-ui) owns the ENTIRE
// resolution chain.
//
// Skip conditions, in order:
//   - explicit user/profile suffix (profile != defaultProfile) — an explicit
//     suffix always wins, resolution is not consulted at all;
//   - no client configured (nil) — disabled, byte-identical legacy behaviour.
//
// The client fails open: on no-match/outage the inputs are returned unchanged
// (profile stays generic). On a hit the returned rawUsername carries the
// resolved suffix so minted tokens flow through the OCI middleware unchanged.
func (h Handlers) resolveProfileFromInventory(
	ctx context.Context, l *logrus.Entry, span trace.Span,
	rawUsername, username, profile string, r *http.Request,
) (string, string) {
	if profile != defaultProfile || h.inventory == nil {
		return rawUsername, profile
	}
	ip := clientIP(r)
	resolved, ok := h.inventory.Resolve(ctx, ip)
	if !ok {
		utils.Debug(l, span, "inventory no-match for client ip %s — profile stays '%s'", ip, profile)
		return rawUsername, profile
	}
	utils.Debug(l, span, "inventory resolved profile '%s' for client ip %s", resolved, ip)
	return username + "/" + resolved, resolved
}

// learnExplicitLogin reports an explicit suffixed login as a per-node
// observation (openspec add-learned-entries): the operator logged in as
// username/profile from this client IP, so the inventory learns ip→profile
// as the node's default for later suffix-less logins. Suffix-less and
// anonymous logins carry no identity statement and are never reported.
// Fire-and-forget — learning must never block or fail a login.
func (h Handlers) learnExplicitLogin(l *logrus.Entry, span trace.Span, profile string, r *http.Request) {
	if profile == defaultProfile || h.inventory == nil {
		return
	}
	ip := clientIP(r)
	utils.Debug(l, span, "reporting explicit login (profile '%s') for client ip %s to the inventory", profile, ip)
	h.inventory.LearnAsync(ip, profile)
}

// exchangeCredentialsWithAuthentik POSTs username/password to Authentik's
// ROPC/client_credentials token endpoint and returns the JWT access token.
func (h Handlers) exchangeCredentialsWithAuthentik(username, password string) (string, error) {
	if h.oidc == nil {
		return "", fmt.Errorf("OIDC provider not configured")
	}
	tokenURL := h.oidc.Config.Endpoint.TokenURL
	if tokenURL == "" {
		return "", fmt.Errorf("OIDC token endpoint not configured")
	}

	data := url.Values{
		"grant_type": {"client_credentials"},
		"client_id":  {h.oidc.Config.ClientID},
		"username":   {username},
		"password":   {password},
		"scope":      {"openid profile email groups"},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(tokenURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to contact Authentik token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Authentik token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode Authentik token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("Authentik returned empty access_token")
	}
	return tokenResp.AccessToken, nil
}

// AuthHandler handles GET /auth — the Docker registry token service.
// Docker clients call this endpoint with Basic auth credentials after receiving
// a 401 from /v2/. This handler proxies the credentials to Authentik's ROPC
// token endpoint and returns the resulting JWT as the registry bearer token.
func (h Handlers) AuthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameAuth,
		"func":    "AuthHandler",
	})
	defer span.End()
	_ = ctx

	rawUsername, password, hasAuth := parseBasicAuth(r)
	// Prefer the ?account= query param as the canonical raw username: Docker sends
	// the full "user/profile" value there unmodified, whereas the Basic auth header
	// may have URL-encoding differences depending on client version.
	acct := r.URL.Query().Get("account")
	if acct != "" {
		rawUsername = acct
	}

	// Anonymous access: no credentials provided
	if !hasAuth || rawUsername == "" {
		utils.Debug(l, span, "anonymous auth request, returning generic token")
		metrics.ObserveTokenIssued(metrics.AuthAnonymous, "")
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"token": "REDACTED-IN-HISTORY"})
		return
	}

	// Parse username/profile from the Docker username
	username, profile := parseUsernameProfile(rawUsername)
	utils.Debug(l, span, "auth request for user:'%s' profile:'%s' acct:'%s'", username, profile, acct)

	// Profile resolution (openspec change hostname-profile-resolution):
	// suffix-less logins consult the inventory service; the rewritten
	// rawUsername makes god-mode/envelope tokens carry the suffix through the
	// OCI middleware unchanged. Explicit suffixes and outages behave exactly
	// as before (see resolveProfileFromInventory).
	rawUsername, profile = h.resolveProfileFromInventory(ctx, l, span, rawUsername, username, profile, r)

	// God-mode check: bypass OIDC if credentials match a god-mode user.
	// Use rawUsername (which preserves the /profile suffix) so the minted
	// token carries the profile through to the OCI auth middleware.
	if h.godmode.IsEnabled() && h.godmode.CheckCredentials(username, password) {
		utils.Debug(l, span, "god-mode auth for user '%s' (profile '%s')", username, profile)
		token, err := h.godmode.MintToken(rawUsername)
		if err != nil {
			utils.Warn(l, span, "god-mode token minting failed for user '%s': %v", username, err)
			w.Header().Set(headerContentType, contentTypeJSON)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "token creation failed"})
			return
		}
		metrics.ObserveTokenIssued(metrics.AuthGodmode, profile)
		usage.Auth(username, profile, metrics.AuthGodmode, usage.ResultSuccess, clientIP(r))
		h.learnExplicitLogin(l, span, profile, r)
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"token": token})
		return
	}

	// OIDC path: exchange credentials with Authentik
	if h.oidc == nil {
		utils.Warn(l, span, "no auth provider available for user '%s' (OIDC disabled, not a god-mode user)", username)
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "no authentication provider configured"})
		return
	}

	authentikJWT, err := h.exchangeCredentialsWithAuthentik(username, password)
	if err != nil {
		utils.Warn(l, span, "Authentik token exchange failed for user '%s': %v", username, err)
		usage.Auth(username, profile, metrics.AuthOIDC, usage.ResultFailure, clientIP(r))
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}
	metrics.ObserveTokenIssued(metrics.AuthOIDC, profile)
	usage.Auth(username, profile, metrics.AuthOIDC, usage.ResultSuccess, clientIP(r))

	// If the login username contains a profile suffix (e.g., "alice/zen2"), wrap
	// the Authentik JWT in a profile envelope token so the profile is preserved
	// through to the OCI middleware. Without a profile suffix, return the
	// Authentik JWT unchanged (existing behaviour).
	if profile != defaultProfile {
		// Learning (openspec add-learned-entries): explicit suffix on a
		// successful OIDC login — report the per-node observation.
		h.learnExplicitLogin(l, span, profile, r)
		signingKey, keyErr := config.EnvelopeSigningKey()
		if keyErr != nil {
			utils.Warn(l, span, "envelope signing key unavailable, returning plain Authentik JWT: %v", keyErr)
			w.Header().Set(headerContentType, contentTypeJSON)
			json.NewEncoder(w).Encode(map[string]string{"token": authentikJWT})
			return
		}
		envelopeToken, mintErr := MintEnvelopeToken(rawUsername, authentikJWT, signingKey)
		if mintErr != nil {
			utils.Warn(l, span, "failed to mint envelope token for user '%s': %v", rawUsername, mintErr)
			w.Header().Set(headerContentType, contentTypeJSON)
			json.NewEncoder(w).Encode(map[string]string{"token": authentikJWT})
			return
		}
		utils.Debug(l, span, "issued envelope token for user '%s' with profile '%s'", username, profile)
		w.Header().Set(headerContentType, contentTypeJSON)
		json.NewEncoder(w).Encode(map[string]string{"token": envelopeToken})
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{"token": authentikJWT})
}

// realmForRequest returns the auth realm URL for the given request.
// It uses the X-Forwarded-Proto header (set by the gateway from the original
// client connection) rather than r.Proto, which is always HTTP/2.0 when the
// gateway proxies over h2c to the backend regardless of the client's scheme.
func realmForRequest(r *http.Request) string {
	switch r.Host {
	case "metahub-registry.org":
		return "https://metahub-registry.org"
	case "metahub.qnib.org":
		return "https://metahub.qnib.org"
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		// Fallback: no proxy header present — trust r.Proto directly.
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}

// Legacy auth functions (AuthMiddleware, AuthMiddlewareHandler, AuthMiddlewareFunc,
// parseTokenParts) have been removed. /v2/ routes now use OCIAuthMiddleware
// which validates JWTs via JWKS and sets gorilla context for metalib compatibility.

// forbidden returns a 403 DENIED for an authenticated user that lacks the
// privilege for the requested action (e.g. a pull-only account attempting a
// push). Unlike unauthorized (401), the client is known — re-authenticating
// will not help — so per the OCI distribution spec we return 403/DENIED and
// do NOT emit a WWW-Authenticate challenge.
func forbidden(w http.ResponseWriter, r *http.Request, det helper.Detail, msg string) {
	l := logrus.WithFields(logrus.Fields{
		"package": "middleware",
		"func":    "forbidden",
		"host":    r.Host,
		"method":  r.Method,
	})
	l.Infof("write denied: %s", msg)
	w.Header().Add(headerContentType, contentTypeJSON)
	e := helper.NewErrPayload("DENIED", msg, det)
	helper.WriteOCIErrror(w, http.StatusForbidden, e)
}

// unauthorized returns a request to the user for auth - e.g., "get a token from this endpoint"
func unauthorized(w http.ResponseWriter, r *http.Request, realm string, det helper.Detail, err oauthError, format string, v ...interface{}) {
	l := logrus.WithFields(logrus.Fields{
		"package": "middleware",
		"func":    "unauthorized",
		"host":    r.Host,
		"method":  r.Method,
		"oauthErr": string(err),
	})
	// Log the internal reason for the 401 (e.g. "inner token invalid: ...").
	// The HTTP body stays generic per the OCI spec, but silently discarding
	// this description previously made auth failures undebuggable.
	reason := fmt.Sprintf(format, v...)
	authUrl := realm + "/auth"
	authenticateHeader := fmt.Sprintf("Bearer realm=%q,service=%q", authUrl, r.Host)
	w.Header().Add(headerContentType, contentTypeJSON)
	l.Infof("401 unauthorized (reason=%q) authenticateHeader: %s", reason, authenticateHeader)
	w.Header().Add("WWW-Authenticate", authenticateHeader)
	//w.Header().Del("Vary")
	//w.Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	///{"errors":[{"code":"UNAUTHORIZED","message":"access to the requested resource is not authorized","detail":[{"Type":"repository","Name":"samalba/my-app","Action":"pull"},{"Type":"repository","Name":"samalba/my-app","Action":"push"}]}]}
	msg := "authentication required"
	e := helper.NewErrPayload("UNAUTHORIZED", msg, det)
	helper.WriteOCIErrror(w, http.StatusUnauthorized, e)
}
