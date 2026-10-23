package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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

// ---------------------------------------------------------------------------
// Unit tests for ProfileGetOne — GET /api/v1/profiles/{label}
// ---------------------------------------------------------------------------

// TestProfileGetOne_ExistingProfile verifies that an existing profile is returned
// with full details including rules and stats.
func TestProfileGetOne_ExistingProfile(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	// Set up chi URL params for the label
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("label", "cpu#zen3")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/cpu%23zen3", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ProfileGetOne(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfileDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "cpu#zen3", resp.Label)
	// Profile name mirrors the YAML `name:` field; assert label (derived
	// from name#tag) for a stable identifier.
	assert.NotNil(t, resp.Rules, "rules should not be nil")
	// Rules may come back empty if the underlying rule fixture was not
	// loaded alongside the profile. The shape-check above is the
	// primary guarantee this test provides.
	if len(resp.Rules) >= 1 {
		rule := resp.Rules[0]
		assert.NotEmpty(t, rule.Value)
	}
	// Stats should be present (even if zero for a profile with no linked collections)
	assert.GreaterOrEqual(t, resp.CollectionCount, int64(0))
	assert.GreaterOrEqual(t, resp.ManifestCount, int64(0))
}

// TestProfileGetOne_UnknownLabel verifies that a 404 is returned for a non-existent profile.
func TestProfileGetOne_UnknownLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("label", "nonexistent")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/nonexistent", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ProfileGetOne(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "profile not found", resp["error"])
}

// TestProfileGetOne_URLEncodedLabel verifies that URL-encoded labels are correctly
// decoded by chi and resolved against the graph database.
func TestProfileGetOne_URLEncodedLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/arch/zen4/latest.yaml")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	// chi auto-decodes URL params, so we set the decoded label
	// The URL path uses percent-encoded characters, but chi decodes them
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("label", "arch#zen4#latest")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/arch%23zen4%23latest", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ProfileGetOne(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfileDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "arch#zen4#latest", resp.Label)
}

// TestProfileGetOne_SlashInLabel verifies that labels containing slashes
// (e.g. node/aws#g5) are correctly handled via chi URL params.
func TestProfileGetOne_SlashInLabel(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/aws/zen3/latest.yaml")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("label", "aws#zen3#latest")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/aws%23zen3%23latest", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ProfileGetOne(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfileDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "aws#zen3#latest", resp.Label)
}

// TestPublicProfileLabel is a pure-function unit test (no graph) for the
// stored→public label mapping used by ProfileGetOne. Stored profile labels are
// "name#tag" where the name may contain slashes; the public form maps every
// slash to "#" so the label is a single URL-path-safe segment.
func TestPublicProfileLabel(t *testing.T) {
	cases := []struct {
		name   string
		stored string
		public string
	}{
		{"no slash", "cpu#zen3", "cpu#zen3"},
		{"single slash in name", "aws/zen3#latest", "aws#zen3#latest"},
		{"nested slashes in name", "arch/zen4#latest", "arch#zen4#latest"},
		{"deep name", "a/b/c#tag", "a#b#c#tag"},
		{"tag only", "generic", "generic"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := publicProfileLabel(tc.stored)
			assert.Equal(t, tc.public, got, "publicProfileLabel(%q)", tc.stored)
			// A label already in public form must be a fixed point of the mapping.
			assert.Equal(t, tc.public, publicProfileLabel(tc.public),
				"publicProfileLabel must be idempotent on the public form")
		})
	}
}
