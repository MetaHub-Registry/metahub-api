package handlers

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/metahub-registry/metahub-api/pkg/config"
)

const (
	godModeIssuer      = "metahub-godmode"
	godModeTokenExpiry = 1 * time.Hour
)

// godModeUserInfo holds password and role for a god-mode user.
type godModeUserInfo struct {
	password string
	role     string
}

// GodModeAuth handles authentication bypass for configured service accounts.
type GodModeAuth struct {
	signingKey []byte
	users      map[string]godModeUserInfo // username -> info
}

// NewGodModeAuth creates a new GodModeAuth from the config.
// Returns nil if god-mode is not enabled.
func NewGodModeAuth(cfg config.GodModeConfig) *GodModeAuth {
	if !cfg.IsEnabled() {
		return nil
	}
	users := make(map[string]godModeUserInfo, len(cfg.Users))
	for _, u := range cfg.Users {
		role := u.Role
		if role == "" {
			role = "admin" // default to admin for CI/CD service accounts
		}
		users[u.Username] = godModeUserInfo{
			password: u.Password,
			role:     role,
		}
	}
	return &GodModeAuth{
		signingKey: []byte(cfg.SigningKey),
		users:      users,
	}
}

// IsEnabled returns true if god-mode is configured and active.
func (g *GodModeAuth) IsEnabled() bool {
	return g != nil && len(g.users) > 0
}

// CheckCredentials verifies a username/password pair against the god-mode user list.
// Uses constant-time comparison to prevent timing attacks.
func (g *GodModeAuth) CheckCredentials(username, password string) bool {
	if g == nil {
		return false
	}
	info, exists := g.users[username]
	if !exists {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(info.password)) == 1
}

// GetRole returns the configured role for a god-mode user.
// Returns "admin" if the user exists but has no explicit role.
func (g *GodModeAuth) GetRole(username string) string {
	if g == nil {
		return "user"
	}
	info, exists := g.users[username]
	if !exists {
		return "user"
	}
	return info.role
}

// MintToken creates an HMAC-SHA256 signed JWT for a god-mode user.
// rawUsername may contain a profile suffix (e.g. "oci/zen2") which is preserved
// in the JWT claims so the OCI middleware can extract the profile. The role is
// resolved from the base username (without profile suffix).
func (g *GodModeAuth) MintToken(rawUsername string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("god-mode is not enabled")
	}

	// Resolve role from the base username (without profile suffix)
	baseUsername, _ := parseUsernameProfile(rawUsername)
	role := g.GetRole(baseUsername)

	now := time.Now()
	claims := jwt.MapClaims{
		"sub":                rawUsername,
		"preferred_username": rawUsername,
		"email":              baseUsername + "@godmode",
		"iss":                godModeIssuer,
		"aud":                godModeIssuer,
		"iat":                now.Unix(),
		"exp":                now.Add(godModeTokenExpiry).Unix(),
		"role":               role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(g.signingKey)
}

// GodModeClaims extends OIDCClaims with a Role field for god-mode users.
type GodModeClaims struct {
	*OIDCClaims
	Role string
}

// VerifyToken attempts to verify a JWT as a god-mode token.
// Returns the extracted claims if the token is a valid god-mode JWT,
// or an error if it's not a god-mode token or verification fails.
func (g *GodModeAuth) VerifyToken(rawToken string) (*GodModeClaims, error) {
	if g == nil {
		return nil, fmt.Errorf("god-mode is not enabled")
	}

	// Parse without verification first to check the issuer
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	unverified, _, err := parser.ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := unverified.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Only handle tokens issued by god-mode
	iss, _ := claims["iss"].(string)
	if iss != godModeIssuer {
		return nil, fmt.Errorf("not a god-mode token (iss=%q)", iss)
	}

	// Now verify the signature properly
	token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return g.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("god-mode token verification failed: %w", err)
	}

	verifiedClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid god-mode token")
	}

	// Extract claims
	username, _ := verifiedClaims["preferred_username"].(string)
	email, _ := verifiedClaims["email"].(string)
	sub, _ := verifiedClaims["sub"].(string)
	role, _ := verifiedClaims["role"].(string)
	if role == "" {
		role = "admin"
	}

	return &GodModeClaims{
		OIDCClaims: &OIDCClaims{
			Username: username,
			Email:    email,
			Sub:      sub,
		},
		Role: role,
	}, nil
}
