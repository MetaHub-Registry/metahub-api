package handlers

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-api/pkg/config"
)

func newTestGodMode() *GodModeAuth {
	return NewGodModeAuth(config.GodModeConfig{
		SigningKey: "test-signing-key-that-is-at-least-32-characters-long",
		Users: []config.GodModeUser{
			{Username: "oci", Password: "oci-pass"},
			{Username: "test", Password: "test-pass"},
		},
	})
}

func TestGodModeAuth_IsEnabled(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var g *GodModeAuth
		assert.False(t, g.IsEnabled())
	})

	t.Run("enabled", func(t *testing.T) {
		g := newTestGodMode()
		assert.True(t, g.IsEnabled())
	})

	t.Run("no users", func(t *testing.T) {
		g := NewGodModeAuth(config.GodModeConfig{})
		assert.Nil(t, g)
	})
}

func TestGodModeAuth_CheckCredentials(t *testing.T) {
	g := newTestGodMode()

	t.Run("valid credentials", func(t *testing.T) {
		assert.True(t, g.CheckCredentials("oci", "oci-pass"))
	})

	t.Run("wrong password", func(t *testing.T) {
		assert.False(t, g.CheckCredentials("oci", "wrong"))
	})

	t.Run("unknown user", func(t *testing.T) {
		assert.False(t, g.CheckCredentials("nobody", "whatever"))
	})

	t.Run("nil receiver", func(t *testing.T) {
		var nilG *GodModeAuth
		assert.False(t, nilG.CheckCredentials("oci", "oci-pass"))
	})
}

func TestGodModeAuth_MintToken(t *testing.T) {
	g := newTestGodMode()

	t.Run("mints valid JWT", func(t *testing.T) {
		tokenStr, err := g.MintToken("oci")
		require.NoError(t, err)
		assert.NotEmpty(t, tokenStr)

		// Parse and verify the token
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-that-is-at-least-32-characters-long"), nil
		})
		require.NoError(t, err)
		assert.True(t, token.Valid)

		claims := token.Claims.(jwt.MapClaims)
		assert.Equal(t, "oci", claims["sub"])
		assert.Equal(t, "oci", claims["preferred_username"])
		assert.Equal(t, "oci@godmode", claims["email"])
		assert.Equal(t, "metahub-godmode", claims["iss"])
		assert.Equal(t, "metahub-godmode", claims["aud"])
	})

	t.Run("nil receiver", func(t *testing.T) {
		var nilG *GodModeAuth
		_, err := nilG.MintToken("oci")
		assert.Error(t, err)
	})
}

func TestGodModeAuth_VerifyToken(t *testing.T) {
	g := newTestGodMode()

	t.Run("valid token", func(t *testing.T) {
		tokenStr, err := g.MintToken("oci")
		require.NoError(t, err)

		claims, err := g.VerifyToken(tokenStr)
		require.NoError(t, err)
		assert.Equal(t, "oci", claims.Username)
		assert.Equal(t, "oci@godmode", claims.Email)
		assert.Equal(t, "oci", claims.Sub)
		assert.Equal(t, "admin", claims.Role)
	})

	t.Run("expired token", func(t *testing.T) {
		// Mint a token with expired time
		now := time.Now().Add(-2 * time.Hour)
		claims := jwt.MapClaims{
			"sub":                "oci",
			"preferred_username": "oci",
			"email":              "oci@godmode",
			"iss":                godModeIssuer,
			"aud":                godModeIssuer,
			"iat":                now.Unix(),
			"exp":                now.Add(1 * time.Hour).Unix(), // expired 1 hour ago
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString([]byte("test-signing-key-that-is-at-least-32-characters-long"))
		require.NoError(t, err)

		_, err = g.VerifyToken(tokenStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("wrong signing key", func(t *testing.T) {
		wrongG := NewGodModeAuth(config.GodModeConfig{
			SigningKey: "different-signing-key-that-is-at-least-32-characters",
			Users:      []config.GodModeUser{{Username: "oci", Password: "pass"}},
		})
		tokenStr, err := wrongG.MintToken("oci")
		require.NoError(t, err)

		// Verify with original god-mode (different key)
		_, err = g.VerifyToken(tokenStr)
		assert.Error(t, err)
	})

	t.Run("non-godmode issuer", func(t *testing.T) {
		// Create a token with a different issuer
		claims := jwt.MapClaims{
			"sub": "oci",
			"iss": "some-other-issuer",
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, err := token.SignedString([]byte("test-signing-key-that-is-at-least-32-characters-long"))
		require.NoError(t, err)

		_, err = g.VerifyToken(tokenStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a god-mode token")
	})

	t.Run("tampered token", func(t *testing.T) {
		tokenStr, err := g.MintToken("oci")
		require.NoError(t, err)

		// Tamper with the token
		tampered := tokenStr[:len(tokenStr)-5] + "XXXXX"
		_, err = g.VerifyToken(tampered)
		assert.Error(t, err)
	})

	t.Run("nil receiver", func(t *testing.T) {
		var nilG *GodModeAuth
		_, err := nilG.VerifyToken("some-token")
		assert.Error(t, err)
	})
}
