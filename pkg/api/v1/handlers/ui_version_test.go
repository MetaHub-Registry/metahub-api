package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mlver "github.com/metahub-registry/metahub-core/pkg/version"
)

// TestVersionGetOne confirms the endpoint returns both api and metalib
// versions as JSON. The api version is injected via SetAPIVersion so
// we set it explicitly here instead of relying on the default
// "unknown" value and package init order.
func TestVersionGetOne(t *testing.T) {
	prev := apiVersion
	defer func() { apiVersion = prev }()
	SetAPIVersion("1.2.3-dev7")

	h := Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.VersionGetOne(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "1.2.3-dev7", body["api"], "api version must come from SetAPIVersion")
	assert.Equal(t, mlver.Version, body["metalib"], "metalib version must come from the imported package")
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// TestVersionGetOne_DefaultUnknown confirms the zero-state (no
// SetAPIVersion call) reports 'unknown' so operators can distinguish
// an image built without the injection wiring from a fresh one.
func TestVersionGetOne_DefaultUnknown(t *testing.T) {
	prev := apiVersion
	defer func() { apiVersion = prev }()
	apiVersion = "unknown"

	h := Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.VersionGetOne(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "unknown", body["api"])
}
