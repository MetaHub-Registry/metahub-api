package handlers

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testEnvelopeKey = "test-envelope-signing-key-at-least-32-chars-long"

// mintTestAuthentikJWT creates a minimal JWT to act as a fake Authentik token.
func mintTestAuthentikJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":                "https://authentik.example.com/",
		"preferred_username": "alice",
		"exp":                exp.Unix(),
		"iat":                time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign with a fake key — envelope tests don't verify inner signature.
	s, err := tok.SignedString([]byte("fake-authentik-key-not-verified-in-envelope"))
	require.NoError(t, err)
	return s
}

func TestMintEnvelopeToken_ProfilePreserved(t *testing.T) {
	inner := mintTestAuthentikJWT(t, time.Now().Add(30*time.Minute))
	env, err := MintEnvelopeToken("alice/zen2", inner, []byte(testEnvelopeKey))
	require.NoError(t, err)
	require.NotEmpty(t, env)

	rawUsername, innerJWT, err := VerifyEnvelopeToken(env, []byte(testEnvelopeKey))
	require.NoError(t, err)
	assert.Equal(t, "alice/zen2", rawUsername)
	assert.Equal(t, inner, innerJWT)
}

func TestMintEnvelopeToken_TTL_InheritsShortInner(t *testing.T) {
	// Inner expires in 20 minutes — envelope should match.
	innerExp := time.Now().Add(20 * time.Minute)
	inner := mintTestAuthentikJWT(t, innerExp)
	env, err := MintEnvelopeToken("alice/zen2", inner, []byte(testEnvelopeKey))
	require.NoError(t, err)

	// Parse the envelope without verification to inspect exp.
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	tok, _, err := parser.ParseUnverified(env, jwt.MapClaims{})
	require.NoError(t, err)
	claims := tok.Claims.(jwt.MapClaims)
	expFloat, ok := claims["exp"].(float64)
	require.True(t, ok)
	envExp := time.Unix(int64(expFloat), 0)

	// Envelope exp should be within a few seconds of inner exp (≤ cap).
	assert.WithinDuration(t, innerExp, envExp, 5*time.Second)
}

func TestMintEnvelopeToken_TTL_CappedAt1Hour(t *testing.T) {
	// Inner expires in 24 hours — envelope must be capped at 1h.
	inner := mintTestAuthentikJWT(t, time.Now().Add(24*time.Hour))
	env, err := MintEnvelopeToken("alice/zen2", inner, []byte(testEnvelopeKey))
	require.NoError(t, err)

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	tok, _, err := parser.ParseUnverified(env, jwt.MapClaims{})
	require.NoError(t, err)
	claims := tok.Claims.(jwt.MapClaims)
	expFloat, ok := claims["exp"].(float64)
	require.True(t, ok)
	envExp := time.Unix(int64(expFloat), 0)

	expectedCap := time.Now().Add(envelopeMaxTTL)
	assert.WithinDuration(t, expectedCap, envExp, 5*time.Second)
}

func TestVerifyEnvelopeToken_ExpiredEnvelope(t *testing.T) {
	// Manually mint an envelope that is already expired.
	inner := mintTestAuthentikJWT(t, time.Now().Add(1*time.Hour))
	pastExp := time.Now().Add(-10 * time.Minute)
	claims := jwt.MapClaims{
		"iss":                envelopeIssuer,
		"preferred_username": "alice/zen2",
		claimInnerToken:      inner,
		"iat":                pastExp.Add(-1 * time.Hour).Unix(),
		"exp":                pastExp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	env, err := tok.SignedString([]byte(testEnvelopeKey))
	require.NoError(t, err)

	_, _, err = VerifyEnvelopeToken(env, []byte(testEnvelopeKey))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestVerifyEnvelopeToken_InvalidSignature(t *testing.T) {
	inner := mintTestAuthentikJWT(t, time.Now().Add(30*time.Minute))
	env, err := MintEnvelopeToken("alice/zen2", inner, []byte(testEnvelopeKey))
	require.NoError(t, err)

	wrongKey := []byte("wrong-envelope-key-that-is-at-least-32-chars-long")
	_, _, err = VerifyEnvelopeToken(env, wrongKey)
	assert.Error(t, err)
}

func TestVerifyEnvelopeToken_NotEnvelopeToken(t *testing.T) {
	// A plain godmode-style token should not be recognised as an envelope.
	claims := jwt.MapClaims{
		"iss": godModeIssuer,
		"sub": "alice",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := tok.SignedString([]byte(testEnvelopeKey))
	require.NoError(t, err)

	_, _, err = VerifyEnvelopeToken(raw, []byte(testEnvelopeKey))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an envelope token")
}

func TestMintEnvelopeToken_InvalidInnerJWT(t *testing.T) {
	_, err := MintEnvelopeToken("alice/zen2", "not-a-jwt", []byte(testEnvelopeKey))
	assert.Error(t, err)
}
