package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// ociConformanceManifest builds a minimal valid OCI manifest referencing
// the supplied config and layer digests. It returns the canonical JSON
// bytes and the sha256 digest string.
func ociConformanceManifest(configDigest, configData string, configSize int, layerDigest string, layerSize int) ([]byte, string) {
	m := map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config": map[string]any{
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest":    configDigest,
			"size":      configSize,
			"data":      configData,
		},
		"layers": []map[string]any{
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"digest":    layerDigest,
				"size":      layerSize,
			},
		},
	}
	b, _ := json.Marshal(m)
	dig := fmt.Sprintf("sha256:%x", sha256.Sum256(b))
	return b, dig
}

// pushBlobDirect is a helper that performs a POST+PUT blob upload sequence,
// returning the upload UUID. It mirrors the OCI conformance suite's blob
// upload flow.
func pushBlobDirect(t *testing.T, ctx context.Context, repo, image, profile, token string, blobData []byte, blobDigest string) {
	t.Helper()
	// POST to initiate upload
	wPost, rPost := utils.CreateNewHTTPTest(ctx, http.MethodPost,
		map[string]string{"repo": repo, "image": image},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/{repo}/{image}/blobs/uploads/", nil,
	)
	h.BlobUploadPostHandler(&wPost, rPost)
	require.Equal(t, http.StatusAccepted, wPost.Code, "POST /blobs/uploads/ should return 202")

	uuid := wPost.Header().Get("Docker-Upload-UUID")
	require.NotEmpty(t, uuid, "POST should return Docker-Upload-UUID")

	// PUT to complete upload
	putURL := fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", blobDigest)
	wPut, rPut := utils.CreateNewHTTPTest(ctx, http.MethodPut,
		map[string]string{"repo": repo, "image": image, "uuid": uuid},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Content-Type":  "application/octet-stream",
		},
		putURL, blobData,
	)
	h.BlobUploadPutHandler(&wPut, rPut)
	require.Equal(t, http.StatusCreated, wPut.Code, "PUT /blobs/uploads/<uuid> should return 201")
}

// TestConformancePushThenHeadByTag replicates the OCI conformance suite's
// flow: push blobs + manifest by tag, then HEAD by tag with manifest-only
// Accept header. Asserts Docker-Content-Digest matches the pushed manifest.
func TestConformancePushThenHeadByTag(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")

	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	repo := "oci"
	image := "cnf-head"
	tag := "latest"
	profile := "generic"
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	// --- Build test content ---
	configJSON := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(configJSON))

	layerData := []byte("fake-layer-data-for-conformance-test")
	layerDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(layerData))

	manifestBytes, manifestDigest := ociConformanceManifest(
		configDigest, "", len(configJSON),
		layerDigest, len(layerData),
	)

	// --- Push blobs ---
	pushBlobDirect(t, ctx, repo, image, profile, token, configJSON, configDigest)
	pushBlobDirect(t, ctx, repo, image, profile, token, layerData, layerDigest)

	// --- Push manifest by tag ---
	wPut, rPut := utils.CreateNewHTTPTest(ctx, http.MethodPut,
		map[string]string{"repo": repo, "image": image, "reference": tag},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Content-Type":  "application/vnd.oci.image.manifest.v1+json",
		},
		"/{repo}/{image}/manifests/{reference}", manifestBytes,
	)
	h.ManifestPutHandler(&wPut, rPut)
	require.Equal(t, http.StatusCreated, wPut.Code, "PUT manifest should return 201")
	putDigest := wPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, manifestDigest, putDigest, "PUT response digest should match computed digest")

	// --- HEAD by tag with manifest-only Accept ---
	wHead, rHead := utils.CreateNewHTTPTest(ctx, http.MethodHead,
		map[string]string{"repo": repo, "image": image, "reference": tag},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Accept":        "application/vnd.oci.image.manifest.v1+json",
		},
		"/{repo}/{image}/manifests/{reference}", nil,
	)
	h.ManifestHeadHandler(&wHead, rHead)
	assert.Equal(t, http.StatusOK, wHead.Code, "HEAD by tag should return 200")
	headDigest := wHead.Header().Get("Docker-Content-Digest")
	assert.Equal(t, manifestDigest, headDigest,
		"HEAD by tag with manifest-only Accept should return the pushed manifest digest, not the index digest")
	assert.Equal(t, "application/vnd.oci.image.manifest.v1+json",
		wHead.Header().Get("Content-Type"),
		"Content-Type should be manifest, not index")
}

// TestConformancePushThenGetByTag mirrors TestConformancePushThenHeadByTag
// but uses GET and additionally verifies the response body.
func TestConformancePushThenGetByTag(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")

	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	repo := "oci"
	image := "cnf-get"
	tag := "latest"
	profile := "generic"
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	// --- Build test content ---
	configJSON := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(configJSON))

	layerData := []byte("fake-layer-data-for-get-conformance-test")
	layerDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(layerData))

	manifestBytes, manifestDigest := ociConformanceManifest(
		configDigest, "", len(configJSON),
		layerDigest, len(layerData),
	)

	// --- Push blobs + manifest ---
	pushBlobDirect(t, ctx, repo, image, profile, token, configJSON, configDigest)
	pushBlobDirect(t, ctx, repo, image, profile, token, layerData, layerDigest)

	wPut, rPut := utils.CreateNewHTTPTest(ctx, http.MethodPut,
		map[string]string{"repo": repo, "image": image, "reference": tag},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Content-Type":  "application/vnd.oci.image.manifest.v1+json",
		},
		"/{repo}/{image}/manifests/{reference}", manifestBytes,
	)
	h.ManifestPutHandler(&wPut, rPut)
	require.Equal(t, http.StatusCreated, wPut.Code)

	// --- GET by tag with manifest-only Accept ---
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet,
		map[string]string{"repo": repo, "image": image, "reference": tag},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Accept":        "application/vnd.oci.image.manifest.v1+json",
		},
		"/{repo}/{image}/manifests/{reference}", nil,
	)
	h.ManifestGetHandler(&wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code, "GET by tag should return 200")
	getDigest := wGet.Header().Get("Docker-Content-Digest")
	assert.Equal(t, manifestDigest, getDigest,
		"GET by tag with manifest-only Accept should return the pushed manifest digest")
	assert.Equal(t, "application/vnd.oci.image.manifest.v1+json",
		wGet.Header().Get("Content-Type"))

	// Verify the body is the original manifest
	assert.JSONEq(t, string(manifestBytes), wGet.Body.String(),
		"GET response body should match the pushed manifest bytes")
}

// TestConformancePushThenGetByTagWithIndexAccept verifies that requesting
// a tag with Accept: application/vnd.oci.image.index.v1+json returns the
// server-side aggregated index.
func TestConformancePushThenGetByTagWithIndexAccept(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")

	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	repo := "oci"
	image := "cnf-idx"
	tag := "latest"
	profile := "generic"
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	// --- Build test content ---
	configJSON := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(configJSON))

	layerData := []byte("fake-layer-data-for-index-accept-test")
	layerDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(layerData))

	manifestBytes, manifestDigest := ociConformanceManifest(
		configDigest, "", len(configJSON),
		layerDigest, len(layerData),
	)

	// --- Push blobs + manifest ---
	pushBlobDirect(t, ctx, repo, image, profile, token, configJSON, configDigest)
	pushBlobDirect(t, ctx, repo, image, profile, token, layerData, layerDigest)

	wPut, rPut := utils.CreateNewHTTPTest(ctx, http.MethodPut,
		map[string]string{"repo": repo, "image": image, "reference": tag},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Content-Type":  "application/vnd.oci.image.manifest.v1+json",
		},
		"/{repo}/{image}/manifests/{reference}", manifestBytes,
	)
	h.ManifestPutHandler(&wPut, rPut)
	require.Equal(t, http.StatusCreated, wPut.Code)

	// --- GET by tag with index Accept ---
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet,
		map[string]string{"repo": repo, "image": image, "reference": tag},
		map[string]string{"accountName": token, "userName": repo, "profileName": profile},
		map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Accept":        "application/vnd.oci.image.index.v1+json",
		},
		"/{repo}/{image}/manifests/{reference}", nil,
	)
	h.ManifestGetHandler(&wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code, "GET by tag should return 200")
	assert.Equal(t, "application/vnd.oci.image.index.v1+json",
		wGet.Header().Get("Content-Type"),
		"Content-Type should be index when client accepts it")

	// Parse the response as an index and verify it contains our manifest
	var idx struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Manifests     []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	err = json.Unmarshal(wGet.Body.Bytes(), &idx)
	require.NoError(t, err, "response should be valid JSON index")
	assert.Equal(t, "application/vnd.oci.image.index.v1+json", idx.MediaType)
	require.NotEmpty(t, idx.Manifests, "index should have at least one descriptor")

	// The index should contain the pushed manifest's digest
	found := false
	for _, m := range idx.Manifests {
		if m.Digest == manifestDigest {
			found = true
			break
		}
	}
	assert.True(t, found,
		"index should contain the pushed manifest digest %s, got descriptors: %v", manifestDigest, idx.Manifests)
}
