package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	envelopeIssuer  = "metahub-profile-token"
	envelopeMaxTTL  = 1 * time.Hour
	claimInnerToken = "inner_token"
)

// MintEnvelopeToken creates an HMAC-SHA256 signed JWT envelope that wraps the
// provided Authentik JWT and preserves the rawUsername (including any /profile
// suffix) as preferred_username.
//
// The envelope exp is set to the lesser of (inner JWT exp) and (now + 1h).
// Returns an error if the inner JWT cannot be parsed or signing fails.
func MintEnvelopeToken(rawUsername, authentikJWT string, signingKey []byte) (string, error) {
	innerExp, err := expFromJWT(authentikJWT)
	if err != nil {
		return "", fmt.Errorf("failed to parse inner token exp: %w", err)
	}

	now := time.Now()
	cap := now.Add(envelopeMaxTTL)
	exp := innerExp
	if exp.After(cap) {
		exp = cap
	}

	claims := jwt.MapClaims{
		"iss":                envelopeIssuer,
		"preferred_username": rawUsername,
		claimInnerToken:      authentikJWT,
		"iat":                now.Unix(),
		"exp":                exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// VerifyEnvelopeToken verifies an envelope token's HMAC-SHA256 signature and
// returns the rawUsername and inner Authentik JWT string embedded in it.
// Returns an error if the token is not an envelope token, the signature is
// invalid, or the envelope is expired.
func VerifyEnvelopeToken(rawToken string, signingKey []byte) (rawUsername, innerJWT string, err error) {
	// First check issuer without full validation to short-circuit non-envelope tokens.
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	unverified, _, parseErr := parser.ParseUnverified(rawToken, jwt.MapClaims{})
	if parseErr != nil {
		return "", "", fmt.Errorf("failed to parse token: %w", parseErr)
	}
	uclaims, ok := unverified.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid claims type")
	}
	iss, _ := uclaims["iss"].(string)
	if iss != envelopeIssuer {
		return "", "", fmt.Errorf("not an envelope token (iss=%q)", iss)
	}

	// Now verify with full validation (signature + expiry).
	token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return signingKey, nil
	})
	if err != nil {
		return "", "", fmt.Errorf("envelope token verification failed: %w", err)
	}

	vclaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", fmt.Errorf("invalid envelope token")
	}

	rawUsername, _ = vclaims["preferred_username"].(string)
	innerJWT, _ = vclaims[claimInnerToken].(string)

	if rawUsername == "" {
		return "", "", fmt.Errorf("envelope token missing preferred_username claim")
	}
	if innerJWT == "" {
		return "", "", fmt.Errorf("envelope token missing inner_token claim")
	}

	return rawUsername, innerJWT, nil
}

// isNotEnvelopeToken returns true when the error from VerifyEnvelopeToken indicates
// that the token is simply not an envelope token (wrong issuer), as opposed to a
// verification failure on a token that IS an envelope token (bad sig, expired, etc.).
// Use this to distinguish "skip to next auth path" from "reject outright".
func isNotEnvelopeToken(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not an envelope token")
}

// expFromJWT parses the exp claim from a JWT without verifying its signature.
// Used to inherit the inner token's expiry for the envelope.
func expFromJWT(rawJWT string) (time.Time, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(rawJWT, jwt.MapClaims{})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid JWT claims")
	}
	expVal, ok := claims["exp"]
	if !ok {
		return time.Time{}, fmt.Errorf("JWT missing exp claim")
	}
	switch v := expVal.(type) {
	case float64:
		return time.Unix(int64(v), 0), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid exp value: %w", err)
		}
		return time.Unix(n, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unexpected exp type: %T", expVal)
	}
}
