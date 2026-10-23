package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OrasListResponse mirrors the JSON envelope returned by OrasGetAll.
type OrasListResponse struct {
	Repos   []string            `json:"repos"`
	BCrumbs []interface{}       `json:"bread_crumbs"`
	Next    map[string]OrasNext `json:"next"`
	SlugEnd int                 `json:"slug_end"`
}

// OrasArtifactResponse mirrors the JSON envelope returned by OrasGetOne.
type OrasArtifactResponse struct {
	Artifact interface{} `json:"artifact"`
	Error    string      `json:"error"`
}

// orasBootstrap sets up a minimal ORAS CycloneDX artifact via the scenario helper.
// The single "scenario:oras_cyclonedx" entry replaces the previous 14-step bootstrap
// of container-image fixtures (platforms, configs, upstreams, layers, manifests,
// manifestdata, collection) — none of which produced actual ORAS artifacts.
func orasBootstrap() utils.Bootstrap {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("scenario:oras_cyclonedx")
	return bs
}

// TestOrasGetAll_EmptyRepo checks that an unknown repo prefix returns 200 with an empty next map.
func TestOrasGetAll_EmptyRepo(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras", nil)
	r.URL.RawQuery = "repo=doesnotexist"
	h.OrasGetAll(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp OrasListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Next, "expected empty next map for unknown repo prefix")
}

// TestOrasGetAll_NoRepo checks that calling with no repo returns 200 and lists the ORAS artifact.
func TestOrasGetAll_NoRepo(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras", nil)
	h.OrasGetAll(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp OrasListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Next, "expected non-empty next map when ORAS artifact is present")
}

// TestOrasGetAll_QnibRepo checks that querying with "qnib" prefix returns the cyclonedx artifact listing.
func TestOrasGetAll_QnibRepo(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras", nil)
	r.URL.RawQuery = "repo=qnib"
	h.OrasGetAll(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp OrasListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Next, "expected next map entry for qnib prefix")
}

// TestOrasGetAll_FullRepo checks that querying with "qnib/featuretest" (the leaf repo) returns
// a next entry keyed by artifact type.
func TestOrasGetAll_FullRepo(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras", nil)
	r.URL.RawQuery = "repo=qnib/featuretest"
	h.OrasGetAll(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp OrasListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	entry, ok := resp.Next["application/vnd.cyclonedx"]
	require.True(t, ok, "expected next entry for application/vnd.cyclonedx")
	assert.Equal(t, "qnib/featuretest", entry.Repo)
	assert.Equal(t, "application/vnd.cyclonedx", entry.ArtifactType)
	assert.Equal(t, int64(1), entry.ArtifactCount)
	assert.Equal(t, 1, resp.SlugEnd)
}

// TestOrasGetOne_MissingParams checks that calling with neither id nor repo+artifact_type returns 400.
func TestOrasGetOne_MissingParams(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras/artifact", nil)
	h.OrasGetOne(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp OrasArtifactResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Error)
}

// TestOrasGetOne_NotFound checks that a valid but non-existent artifact type returns 404.
func TestOrasGetOne_NotFound(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras/artifact", nil)
	r.URL.RawQuery = "repo=qnib/featuretest&artifact_type=application/vnd.spdx+json"
	h.OrasGetOne(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestOrasGetOne_ByName checks that a known repo+artifact_type returns 200 with the artifact.
func TestOrasGetOne_ByName(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras/artifact", nil)
	r.URL.RawQuery = "repo=qnib/featuretest&artifact_type=application/vnd.cyclonedx"
	h.OrasGetOne(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp OrasArtifactResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Error, "expected no error in response")
	assert.NotNil(t, resp.Artifact, "expected artifact in response")
}

// TestOrasGetOne_StorageIDRejected verifies that the removed ?id= path is
// not part of the public API: artifacts are addressed by repo + artifact_type.
func TestOrasGetOne_StorageIDRejected(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras/artifact", nil)
	r.URL.RawQuery = "id=42"
	h.OrasGetOne(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOrasGetAll_ImageOnlyRepoAbsent verifies that a graph containing only container-image
// manifests returns an empty ORAS listing (the image-only repo must not appear).
func TestOrasGetAll_ImageOnlyRepoAbsent(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/oras", nil)
	h.OrasGetAll(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp OrasListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Next, "image-only repo should not appear in ORAS listing")
	_ = br
}

// TestOrasGetOne_BadID checks that a non-integer id parameter returns 400.
func TestOrasGetOne_BadID(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras/artifact", nil)
	r.URL.RawQuery = "id=notanumber"
	h.OrasGetOne(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOrasGetOne_ByName_HasBreadcrumbs verifies that OrasGetOne returns breadcrumbs
// matching the frontend's expected response shape.
func TestOrasGetOne_ByName_HasBreadcrumbs(t *testing.T) {
	ctx := context.Background()
	bs := orasBootstrap()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/oras/artifact", nil)
	r.URL.RawQuery = "repo=qnib/featuretest&artifact_type=application/vnd.cyclonedx"
	h.OrasGetOne(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	// Parse raw JSON to verify the bread_crumbs key is present
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	_, hasBreadCrumbs := raw["bread_crumbs"]
	assert.True(t, hasBreadCrumbs, "OrasGetOne response must include 'bread_crumbs' key")

	// Verify breadcrumb structure
	type BreadCrumb struct {
		Crumb  string `json:"crumb"`
		Active int    `json:"active"`
		Link   string `json:"link"`
		Repo   string `json:"repo"`
		IDX    int    `json:"idx"`
	}
	var crumbs []BreadCrumb
	require.NoError(t, json.Unmarshal(raw["bread_crumbs"], &crumbs))
	require.Len(t, crumbs, 2, "expected 2 breadcrumbs for 'qnib/featuretest'")
	assert.Equal(t, "qnib", crumbs[0].Crumb)
	assert.Equal(t, 1, crumbs[0].Active)
	assert.Equal(t, "qnib", crumbs[0].Link)
	assert.Equal(t, "featuretest", crumbs[1].Crumb)
	assert.Equal(t, 0, crumbs[1].Active, "last breadcrumb must have active == 0")
	assert.Equal(t, "qnib/featuretest", crumbs[1].Link)
}
