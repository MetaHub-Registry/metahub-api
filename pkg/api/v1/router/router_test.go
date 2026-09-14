package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-api/pkg/api/v1/handlers"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

// newTestGodMode returns a GodModeAuth with a couple of fixed test users.
// It is a local copy of the helper used in the handlers package tests so that
// this router-level test does not depend on unexported fixtures from another
// package.
func newTestGodMode() *handlers.GodModeAuth {
	return handlers.NewGodModeAuth(config.GodModeConfig{
		SigningKey: "test-signing-key-that-is-at-least-32-characters-long",
		Users: []config.GodModeUser{
			{Username: "oci", Password: "oci-pass"},
		},
	})
}

// TestRouter_GodModeLoginRoute_Public confirms that the router wires
// POST /api/v1/godmode/login to the GodModeLogin handler and that the route
// is reachable without any authentication.
func TestRouter_GodModeLoginRoute_Public(t *testing.T) {
	r := Initialize(magazine.Magazine{}, nil, newTestGodMode(), nil)

	// Valid god-mode credentials → 200
	body := strings.NewReader(`{"username":"oci","password":"oci-pass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/godmode/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["token"])
}

// TestRouter_GodModeLoginRoute_DisabledReturns404 confirms that when the
// router is initialised without god-mode, POST /api/v1/godmode/login returns
// 404 (feature not enabled) rather than 401 or 405.
func TestRouter_GodModeLoginRoute_DisabledReturns404(t *testing.T) {
	r := Initialize(magazine.Magazine{}, nil, nil, nil)

	body := strings.NewReader(`{"username":"oci","password":"oci-pass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/godmode/login", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestRouter_AuthConfigRoute_Public confirms that GET /api/v1/auth/config is
// wired and reachable without authentication.
func TestRouter_AuthConfigRoute_Public(t *testing.T) {
	r := Initialize(magazine.Magazine{}, nil, newTestGodMode(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]bool
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp["oidc_enabled"])
	assert.True(t, resp["godmode_enabled"])
}

// TestRouter_AuthConfigRoute_NoAuthRequired confirms that no Authorization
// header is required to reach the auth/config endpoint.
func TestRouter_AuthConfigRoute_NoAuthRequired(t *testing.T) {
	r := Initialize(magazine.Magazine{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	// Intentionally no Authorization header.
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
