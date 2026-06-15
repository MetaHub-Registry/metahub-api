package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiutils "github.com/metahub-registry/metahub-api/internal/utils"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type CollectionResponse struct {
	Repos      []string              `json:"repos"`
	BCrumbs    []apiutils.BreadCrumb `json:"bread_crumbs"`
	Collection UICollectionDetail    `json:"collection"`
}

type CollectionsResponse struct {
	Next    map[string]Next       `json:"next"`
	SlugEnd int                   `json:"slug_end"`
	Repos   []string              `json:"repos"`
	BCrumbs []apiutils.BreadCrumb `json:"bread_crumbs"`
}

type ReqRepo struct {
	Repo string `json:"repo"`
}

func (rr ReqRepo) ToByte() []byte {
	b, _ := json.Marshal(rr)
	return b
}

type ReqPayload struct {
	ColID   int64  `json:"collection_id"`
	TagID   int64  `json:"tag_id"`
	// No omitempty: tests need to send `"profile":""` explicitly to
	// verify the handler normalises an empty string to "no filter".
	Profile string `json:"profile"`
}

func (rpl ReqPayload) ToByte() []byte {
	newFsConfigBytes, _ := json.Marshal(rpl)
	return newFsConfigBytes
}

func TestCollectionGetOne(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	//bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	//bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	//bs.PostTestSleep = 180
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	_ = br
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatal(err)
	}
	colTagID, err := br.GetInt64("tag:arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	// Tests
	testProfile := "generic"
	reqBody := ReqPayload{
		ColID: colID,
		TagID: colTagID,
	}
	wGet := httptest.NewRecorder()
	_, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
	h.RepoGetOne(wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code)
	var resp CollectionResponse
	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "qnib/featuretest", resp.Collection.Name)
	assert.Equal(t, "arm64", resp.Collection.Tag)
}

func TestCollectionGetAll(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("computeManifests")
	bs.AddBootstrap("computeManifestlists")
	//bs.PostTestSleep = 300
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Log(err)
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	_ = br
	testRepo := "qnib"
	testProfile := "generic"
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// Tests
	reqBody := ReqRepo{
		Repo: "qnib",
	}
	wGet := httptest.NewRecorder()
	_, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
	h.RepoGetAll(wGet, rGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, wGet.Code)
	}
	var resp CollectionsResponse
	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, resp.SlugEnd)
	assert.Len(t, resp.Next, 1)
	// qnib/featuretest
	reqBodyFT := ReqRepo{
		Repo: "qnib/featuretest",
	}
	wGetFT := httptest.NewRecorder()
	_, rGetFT := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/api/v1/collections", reqBodyFT.ToByte())
	h.RepoGetAll(wGetFT, rGetFT)
	if wGetFT.Code != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, wGetFT.Code)
	}
	var respFT CollectionsResponse
	body := wGetFT.Body.Bytes()
	err = json.Unmarshal(body, &respFT)
	if err != nil {
		t.Fatal(err)
	}
	if false {
		var prettyJSON bytes.Buffer
		error := json.Indent(&prettyJSON, body, "", "\t")
		if error != nil {
			t.Fatalf("error indenting json: %v", error)
		}
		fmt.Println(prettyJSON.String())
	}
	if len(respFT.Next) != 3 {
		t.Fatalf("Expected 3, got %d", len(respFT.Next))
	}
	_, ok := respFT.Next["generic"]
	if !ok {
		t.Fatalf("Expected 'latest' key in Next: %v", respFT.Next)
	}
	n := respFT.Next["generic"]
	assert.Equal(t, "qnib/featuretest", n.Repo)
	assert.Equal(t, "generic", n.Tag)

	// Breadcrumb assertions: link must be bare repo path (no /collections/ prefix)
	// and the last crumb must have active == 0
	assert.NotEmpty(t, respFT.BCrumbs, "expected breadcrumbs when repo is specified")
	for _, bc := range respFT.BCrumbs {
		assert.NotContains(t, bc.Link, "/collections/", "breadcrumb link must not contain /collections/ prefix")
	}
	lastBC := respFT.BCrumbs[len(respFT.BCrumbs)-1]
	assert.Equal(t, 0, lastBC.Active, "last breadcrumb must have active == 0")
}

func TestCollectionGetOneBreadcrumbs(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	colID, err := br.GetInt64("collection:ft#id")
	if err != nil {
		t.Fatal(err)
	}
	colTagID, err := br.GetInt64("tag:arm64#id")
	if err != nil {
		t.Fatal(err)
	}

	reqBody := ReqPayload{ColID: colID, TagID: colTagID}
	wGet := httptest.NewRecorder()
	_, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":  "qnib",
		"image": "featuretest",
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/octet-stream",
	}, "/{repo}/{image}/blobs/uploads/{uuid}", reqBody.ToByte())
	h.RepoGetOne(wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code)

	var resp CollectionResponse
	err = json.Unmarshal(wGet.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}

	// Breadcrumb assertions
	assert.NotEmpty(t, resp.BCrumbs, "expected breadcrumbs in collection detail response")
	for _, bc := range resp.BCrumbs {
		assert.NotContains(t, bc.Link, "/collections/", "breadcrumb link must not contain /collections/ prefix")
	}
	lastBC := resp.BCrumbs[len(resp.BCrumbs)-1]
	assert.Equal(t, 0, lastBC.Active, "last breadcrumb must have active == 0")
}

// TestCollectionGetAll_OrasOnlyRepoAbsent verifies that a graph containing only ORAS artifact
// manifests returns an empty collections listing (the ORAS-only repo must not appear).
func TestCollectionGetAll_OrasOnlyRepoAbsent(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("scenario:oras_cyclonedx")

	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	body := ReqRepo{Repo: ""}.ToByte()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collections",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetAll(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp CollectionsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	assert.Empty(t, resp.Next, "ORAS-only repo should not appear in collections listing")
}

// TestCollectionGetOne_UIResponseShape validates that CollectionGetOne returns the exact
// JSON structure expected by the React frontend:
//   - collection.name (string) — not "repo"
//   - collection.tag (string)
//   - collection.manifests (array of objects, not a map)
//   - Each manifest has: digest, media_type, platform.{os, architecture, variant}, size, registry
func TestCollectionGetOne_UIResponseShape(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("computeManifests")
	bs.AddBootstrap("computeManifestlists")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	colID, err := br.GetInt64("collection:ft#id")
	require.NoError(t, err)
	colTagID, err := br.GetInt64("tag:generic#id")
	require.NoError(t, err)

	reqBody := ReqPayload{ColID: colID, TagID: colTagID}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collection",
		bytes.NewBuffer(reqBody.ToByte()))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetOne(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Parse raw JSON to verify structure independent of Go type definitions
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))

	// 1. "collection" must be present
	colRaw, ok := raw["collection"]
	require.True(t, ok, "response must contain 'collection' key")

	var colMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(colRaw, &colMap))

	// 2. "name" field (not "repo")
	_, hasName := colMap["name"]
	assert.True(t, hasName, "collection must have 'name' field")
	_, hasRepo := colMap["repo"]
	assert.False(t, hasRepo, "collection must NOT have 'repo' field (use 'name' instead)")

	// 3. "tag" field
	_, hasTag := colMap["tag"]
	assert.True(t, hasTag, "collection must have 'tag' field")

	// 4. "manifests" must be a JSON array (not an object/map)
	manifestsRaw, hasManifests := colMap["manifests"]
	require.True(t, hasManifests, "collection must have 'manifests' field")
	var manifestsArr []json.RawMessage
	err = json.Unmarshal(manifestsRaw, &manifestsArr)
	require.NoError(t, err, "manifests must be a JSON array, not an object")
	assert.NotEmpty(t, manifestsArr, "manifests array should not be empty")

	// 5. Parse into our DTO type and validate field values
	var resp CollectionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "qnib/featuretest", resp.Collection.Name)
	assert.Equal(t, "generic", resp.Collection.Tag)
	assert.NotEmpty(t, resp.Collection.Manifests, "manifests array must have entries")

	for _, m := range resp.Collection.Manifests {
		assert.NotEmpty(t, m.Digest, "manifest must have digest")
		assert.NotEmpty(t, m.MediaType, "manifest must have media_type")
		assert.NotEmpty(t, m.Platform.OS, "manifest must have platform.os")
		assert.NotEmpty(t, m.Platform.Architecture, "manifest must have platform.architecture")
		assert.Greater(t, m.Size, int64(0), "manifest must have size > 0")
	}

	// 6. bread_crumbs must be present and well-formed
	assert.NotEmpty(t, resp.BCrumbs, "response must contain breadcrumbs")
	lastBC := resp.BCrumbs[len(resp.BCrumbs)-1]
	assert.Equal(t, 0, lastBC.Active, "last breadcrumb must have active == 0")
}

// TestCollectionGetOne_ManifestMediaTypeField verifies that manifests use "media_type"
// (snake_case) rather than "mediaType" (camelCase) in the JSON response.
func TestCollectionGetOne_ManifestMediaTypeField(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("computeManifests")
	bs.AddBootstrap("computeManifestlists")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	colID, err := br.GetInt64("collection:ft#id")
	require.NoError(t, err)
	colTagID, err := br.GetInt64("tag:generic#id")
	require.NoError(t, err)

	reqBody := ReqPayload{ColID: colID, TagID: colTagID}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collection",
		bytes.NewBuffer(reqBody.ToByte()))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetOne(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Parse raw JSON and check manifest field names
	body := w.Body.String()
	assert.Contains(t, body, `"media_type"`, "manifest must use snake_case 'media_type' field")
	assert.NotContains(t, body, `"mediaType"`, "manifest must NOT use camelCase 'mediaType' field")
}
