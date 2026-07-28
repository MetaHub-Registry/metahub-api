package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
)

// AuthHandler's god-mode branch: valid god-mode credentials short-circuit OIDC
// and return a minted god-mode token. (The OIDC/anonymous/envelope paths are
// covered in auth_test.go.)
func TestAuthHandler_GodModeSuccess(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}
	r := httptest.NewRequest("GET", "/auth", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["token"])
	// the minted token must verify as a god-mode token
	_, err := h.godmode.VerifyToken(resp["token"])
	assert.NoError(t, err)
}

func TestAuthHandler_GodModeWithProfileAccountParam(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}
	// Docker sends the full user/profile in ?account=; god-mode still authorises
	// on the parsed username and mints a token carrying the profile suffix.
	r := httptest.NewRequest("GET", "/auth?account=oci/zen2", nil)
	r.SetBasicAuth("oci", "oci-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["token"])
}

func TestAuthHandler_GodModeWrongPassword_NoProvider(t *testing.T) {
	// god-mode enabled but credentials don't match, and OIDC is not configured:
	// the handler falls through to the "no auth provider" 401.
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}
	r := httptest.NewRequest("GET", "/auth", nil)
	r.SetBasicAuth("oci", "wrong-pass")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_UnknownUser_NoProvider(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{}, godmode: newTestGodMode()}
	r := httptest.NewRequest("GET", "/auth", nil)
	r.SetBasicAuth("nobody", "pw")
	w := httptest.NewRecorder()
	h.AuthHandler(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
