package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"golang.org/x/oauth2"
)

// OIDCClaims holds the identity claims extracted from a validated OIDC token.
type OIDCClaims struct {
	Sub      string
	Username string // preferred_username
	Email    string
	Groups   []string
}

// oidcClaimsKey is the typed context key used to store OIDCClaims.
type oidcClaimsKey struct{}

// OIDCClaimsFromContext retrieves OIDCClaims from the request context.
func OIDCClaimsFromContext(ctx context.Context) (*OIDCClaims, bool) {
	v, ok := ctx.Value(oidcClaimsKey{}).(*OIDCClaims)
	return v, ok && v != nil
}

// contextWithClaims returns a new context with the given OIDCClaims stored.
// Used by middleware and by tests to inject claims without a real token.
func contextWithClaims(ctx context.Context, claims *OIDCClaims) context.Context {
	return context.WithValue(ctx, oidcClaimsKey{}, claims)
}

// OIDCProvider wraps the go-oidc provider, oauth2 config and ID-token verifier.
type OIDCProvider struct {
	Provider *gooidc.Provider
	Config   oauth2.Config
	Verifier *gooidc.IDTokenVerifier
}

// NewOIDCProvider performs OIDC discovery against cfg.Issuer and returns a
// ready-to-use OIDCProvider. Returns an error if the issuer is unreachable or
// required config fields are missing.
func NewOIDCProvider(ctx context.Context, cfg magazine.OIDCCfg) (*OIDCProvider, error) {
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("oidc.issuer is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oidc.clientid is required")
	}
	if cfg.RedirectURI == "" {
		return nil, fmt.Errorf("oidc.redirect_uri is required")
	}

	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for issuer %q: %w", cfg.Issuer, err)
	}

	verifier := provider.Verifier(&gooidc.Config{
		ClientID: cfg.ClientID,
	})

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	return &OIDCProvider{
		Provider: provider,
		Config:   oauthCfg,
		Verifier: verifier,
	}, nil
}

// extractRawToken returns the Bearer token from the Authorization header,
// or an empty string if not present or malformed.
func extractRawToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// oidcClaimsFromToken verifies rawToken and extracts claims into OIDCClaims.
func (op *OIDCProvider) oidcClaimsFromToken(ctx context.Context, rawToken string) (*OIDCClaims, error) {
	idToken, err := op.Verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	var extra struct {
		PreferredUsername string   `json:"preferred_username"`
		Email             string   `json:"email"`
		Groups            []string `json:"groups"`
	}
	if err := idToken.Claims(&extra); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	username := extra.PreferredUsername
	if username == "" {
		username = extra.Email
	}

	return &OIDCClaims{
		Sub:      idToken.Subject,
		Username: username,
		Email:    extra.Email,
		Groups:   extra.Groups,
	}, nil
}

// writeJSONError writes a JSON error response with the given status code.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(map[string]string{"error": msg})
	w.Write(b)
}
