package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type ProfilesResponse struct {
	Profiles []interface{} `json:"profiles"`
}

// TestProfilesGetAll_Empty verifies that the endpoint returns 200 with an empty list when no profiles exist.
func TestProfilesGetAll_Empty(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	w := httptest.NewRecorder()
	h.ProfilesGetAll(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfilesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Profiles)
	assert.Len(t, resp.Profiles, 0)
}

// TestProfilesGetAll_WithProfiles verifies that profiles bootstrapped in the graph are returned.
func TestProfilesGetAll_WithProfiles(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	w := httptest.NewRecorder()
	h.ProfilesGetAll(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfilesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, len(resp.Profiles), 1, "expected at least one profile")
}
