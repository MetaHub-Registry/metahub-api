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

// TestProfileGetOne_IncludesEmptyManipulations verifies that when a
// profile has no manipulations linked, the response contains an empty
// `manipulations` slice (not null) so the UI can rely on a stable shape.
func TestProfileGetOne_IncludesEmptyManipulations(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("profile:profiles/cpu/zen3.yaml")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("label", "cpu#zen3")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/cpu%23zen3", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.ProfileGetOne(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfileDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Manipulations, "manipulations slice must be non-nil")
	assert.Len(t, resp.Manipulations, 0)
	// Ensure raw JSON contains `manipulations` key with an empty array.
	assert.Contains(t, w.Body.String(), `"manipulations":[]`)
}

// TestProfileGetOne_IncludesExtendedManipulation bootstraps a manipulation
// in the graph plus a profile that links to it, then asserts the
// manipulations tree is exposed via the API with default + variants
// populated.
func TestProfileGetOne_IncludesExtendedManipulation(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	// Rules referenced by the profile must exist before ProfileGetInsert
	// attempts to link them.
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	// Bootstrap the manipulation first, then the profile (which links to
	// the manipulation by label).
	bs.AddBootstrap("manipulation:manipulations/layer/test-extended/1.0.0.yaml")
	bs.AddBootstrap("profile:profiles/test-extended/1.0.0.yaml")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("label", "test-extended#1.0.0")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/test-extended%231.0.0", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.ProfileGetOne(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProfileDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Manipulations, 1, "expected one manipulation linked to profile")
	mp := resp.Manipulations[0]
	assert.Equal(t, "layer/test-extended#1.0.0", mp.Label)
	require.NotNil(t, mp.Default, "extended manipulation must carry a default block")
	assert.Equal(t, "/opt/test", mp.Default.Workdir)
	assert.Equal(t, "1000:1000", mp.Default.User)
	require.NotNil(t, mp.Default.Entrypoint)
	assert.Equal(t, "replace", mp.Default.Entrypoint.Op)
	assert.Equal(t, []string{"/opt/test/bin/entry"}, mp.Default.Entrypoint.Value)
	require.NotNil(t, mp.Default.Cmd)
	assert.Equal(t, "append", mp.Default.Cmd.Op)
	assert.Equal(t, []string{"--verbose"}, mp.Default.Cmd.Value)
	require.Len(t, mp.Variants, 2)
	// Index by rule for order-agnostic assertions.
	byRule := map[string]UIManipuVariant{}
	for _, v := range mp.Variants {
		byRule[v.Rule] = v
	}
	require.Contains(t, byRule, "arch:arm64=true")
	require.Contains(t, byRule, "arch:amd64=true")
	assert.Equal(t, "arm64", byRule["arch:arm64=true"].Config.Env["ARCH"])
	require.NotNil(t, byRule["arch:amd64=true"].Config.Cmd)
	assert.Equal(t, []string{"--avx"}, byRule["arch:amd64=true"].Config.Cmd.Value)
}
