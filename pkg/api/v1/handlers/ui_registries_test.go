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

func newRegistriesTestHandlers(regs ...magazine.RegistryConfig) Handlers {
	return Handlers{
		environment: "dev",
		mag: magazine.Magazine{
			Config: magazine.MagConfig{
				Args:       map[string]string{},
				Registries: regs,
			},
		},
	}
}

func TestRegistriesGetAll_TwoRegistries(t *testing.T) {
	h := newRegistriesTestHandlers(
		magazine.RegistryConfig{Name: "zot.example.com", Hostname: "zot.example.com", AllowAnon: true, TLS: true},
		magazine.RegistryConfig{Name: "local-reg", URL: "registry:5050", AllowAnon: true, TLS: false},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries", nil)
	rr := httptest.NewRecorder()
	h.RegistriesGetAll(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var body map[string][]registryEntry
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)

	regs := body["registries"]
	require.Len(t, regs, 2)

	assert.Equal(t, "zot.example.com", regs[0].Name)
	assert.Equal(t, "zot.example.com", regs[0].URL)
	assert.True(t, regs[0].AllowAnon)
	assert.True(t, regs[0].TLS)

	assert.Equal(t, "local-reg", regs[1].Name)
	assert.Equal(t, "registry:5050", regs[1].URL)
	assert.True(t, regs[1].AllowAnon)
	assert.False(t, regs[1].TLS)
}

func TestRegistriesGetAll_EmptyReturnsEmptyArray(t *testing.T) {
	h := newRegistriesTestHandlers() // no registries

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries", nil)
	rr := httptest.NewRecorder()
	h.RegistriesGetAll(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string][]registryEntry
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)

	regs := body["registries"]
	assert.NotNil(t, regs, "empty registries should be [] not null")
	assert.Len(t, regs, 0)
}

func TestRegistriesGetAll_LegacyHostnameOnly(t *testing.T) {
	h := newRegistriesTestHandlers(
		magazine.RegistryConfig{Hostname: "zot.example.com", AllowAnon: true, TLS: false},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries", nil)
	rr := httptest.NewRecorder()
	h.RegistriesGetAll(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string][]registryEntry
	err := json.Unmarshal(rr.Body.Bytes(), &body)
	require.NoError(t, err)

	regs := body["registries"]
	require.Len(t, regs, 1)
	assert.Equal(t, "zot.example.com", regs[0].Name, "EffectiveName should fall back to Hostname")
	assert.Equal(t, "zot.example.com", regs[0].URL, "EffectiveURL should fall back to EffectiveName when URL is empty")
}
