package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestIndexPullAfterTwoArchPushes validates the full multi-arch flow:
// push amd64 manifest, push arm64 manifest, then GET the computed
// index and verify it contains both descriptors with platform metadata.
func TestIndexPullAfterTwoArchPushes(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	testRepo := "oci"
	testImage := "multiarch"
	testTag := "latest"

	// --- Push amd64 manifest ---
	amd64Manifest := types.Manifest{
		Data: []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:22f68afca98941fc10a8c6c87f881065b075c1c2f4678b8f579d1b77bc060ef0",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
			},
			"layers": []
		}`),
	}
	amd64Digest := digest.FromBytes(amd64Manifest.Data)
	wPut1, rPut1 := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", amd64Manifest.Data)
	h.ManifestPutHandler(&wPut1, rPut1)
	require.Equal(t, http.StatusCreated, wPut1.Code, "amd64 push must return 201")
	assert.Equal(t, amd64Digest.String(), wPut1.Header().Get("Docker-Content-Digest"))

	// --- Push arm64 manifest ---
	arm64Manifest := types.Manifest{
		Data: []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:60e38b2fbdb2b4d66b0df7158b380d3c5979c6d708e011c4cfc0c9d6c6cd069a",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYXJtNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
			},
			"layers": []
		}`),
	}
	arm64Digest := digest.FromBytes(arm64Manifest.Data)
	wPut2, rPut2 := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", arm64Manifest.Data)
	h.ManifestPutHandler(&wPut2, rPut2)
	require.Equal(t, http.StatusCreated, wPut2.Code, "arm64 push must return 201")
	assert.Equal(t, arm64Digest.String(), wPut2.Header().Get("Docker-Content-Digest"))

	// --- GET the index by tag ---
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestGetHandler(&wGet, rGet)
	require.Equal(t, http.StatusOK, wGet.Code, "GET index must return 200")
	assert.Equal(t, "application/vnd.oci.image.index.v1+json",
		wGet.Header().Get("Content-Type"), "response must be an OCI index")

	// Parse the index JSON
	var idx struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Manifests     []struct {
			Digest   string `json:"digest"`
			Size     int64  `json:"size"`
			Platform struct {
				Architecture string `json:"architecture"`
				OS           string `json:"os"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	err = json.Unmarshal(wGet.Body.Bytes(), &idx)
	require.NoError(t, err, "index JSON must be valid")
	assert.Equal(t, 2, idx.SchemaVersion)
	require.Len(t, idx.Manifests, 2, "index must have 2 descriptors")

	// Collect architectures
	archs := map[string]bool{}
	for _, m := range idx.Manifests {
		archs[m.Platform.Architecture] = true
		assert.Equal(t, "linux", m.Platform.OS)
		assert.NotZero(t, m.Size, "descriptor size must be non-zero")
		assert.NotEmpty(t, m.Digest, "descriptor digest must be non-empty")
	}
	assert.True(t, archs["amd64"], "index must contain amd64")
	assert.True(t, archs["arm64"], "index must contain arm64")
}

// TestIndexPut_ClientPushedIndex validates the handleIndexPut path:
// push two single-arch manifests, then push a client-assembled OCI image
// index referencing them. The index should be stored with origin=client-pushed.
func TestIndexPut_ClientPushedIndex(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	testRepo := "oci"
	testImage := "idxput"
	testTag := "latest"

	// Push amd64 manifest
	amd64Data := []byte(`{
		"schemaVersion": 2,
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "sha256:22f68afca98941fc10a8c6c87f881065b075c1c2f4678b8f579d1b77bc060ef0",
			"size": 129,
			"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
		},
		"layers": []
	}`)
	amd64Digest := digest.FromBytes(amd64Data)
	wP1, rP1 := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", amd64Data)
	h.ManifestPutHandler(&wP1, rP1)
	require.Equal(t, http.StatusCreated, wP1.Code)

	// Push arm64 manifest
	arm64Data := []byte(`{
		"schemaVersion": 2,
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "sha256:60e38b2fbdb2b4d66b0df7158b380d3c5979c6d708e011c4cfc0c9d6c6cd069a",
			"size": 129,
			"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYXJtNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
		},
		"layers": []
	}`)
	arm64Digest := digest.FromBytes(arm64Data)
	wP2, rP2 := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", arm64Data)
	h.ManifestPutHandler(&wP2, rP2)
	require.Equal(t, http.StatusCreated, wP2.Code)

	// Now push a client-assembled OCI image index
	indexJSON := fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": [
			{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"size": %d,
				"digest": "%s",
				"platform": {"architecture": "amd64", "os": "linux"}
			},
			{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"size": %d,
				"digest": "%s",
				"platform": {"architecture": "arm64", "os": "linux"}
			}
		]
	}`, len(amd64Data), amd64Digest, len(arm64Data), arm64Digest)

	wIdx, rIdx := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.oci.image.index.v1+json",
	}, "/{repo}/{image}/manifests/{reference}", []byte(indexJSON))
	h.ManifestPutHandler(&wIdx, rIdx)
	assert.Equal(t, http.StatusCreated, wIdx.Code, "index push must return 201")
	assert.NotEmpty(t, wIdx.Header().Get("Docker-Content-Digest"), "must return index digest")
}

// TestIndexPut_MissingMember validates that pushing an index referencing
// a non-existent manifest returns an error.
func TestIndexPut_MissingMember(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	// Push an index referencing a non-existent manifest
	indexJSON := `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": [
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"size": 100,
				"digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"platform": {"architecture": "amd64", "os": "linux"}
			}
		]
	}`

	wIdx, rIdx := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": "oci", "image": "missingmember", "reference": "latest",
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.oci.image.index.v1+json",
	}, "/{repo}/{image}/manifests/{reference}", []byte(indexJSON))
	h.ManifestPutHandler(&wIdx, rIdx)
	// Should fail — the member manifest doesn't exist
	assert.NotEqual(t, http.StatusCreated, wIdx.Code,
		"index push with missing member must not succeed")
}

// TestManifestGetByDigestFallback validates that a manifest pushed to
// a shared tag can still be retrieved by digest even when the wrapper
// :Manifest node only links to the most recent push.
func TestManifestGetByDigestFallback(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	err := h.mag.Minio.EmptyAll()
	require.NoError(t, err)
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	testRepo := "oci"
	testImage := "digesttest"
	testTag := "latest"

	// Push first manifest (will lose wrapper link after second push)
	mani1 := types.Manifest{
		Data: []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:22f68afca98941fc10a8c6c87f881065b075c1c2f4678b8f579d1b77bc060ef0",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
			},
			"layers": []
		}`),
	}
	dig1 := digest.FromBytes(mani1.Data)
	wPut1, rPut1 := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani1.Data)
	h.ManifestPutHandler(&wPut1, rPut1)
	require.Equal(t, http.StatusCreated, wPut1.Code)

	// Push second manifest to the same tag (overwrites wrapper link)
	mani2 := types.Manifest{
		Data: []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:60e38b2fbdb2b4d66b0df7158b380d3c5979c6d708e011c4cfc0c9d6c6cd069a",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYXJtNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"
			},
			"layers": []
		}`),
	}
	wPut2, rPut2 := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": testRepo, "image": testImage, "reference": testTag,
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani2.Data)
	h.ManifestPutHandler(&wPut2, rPut2)
	require.Equal(t, http.StatusCreated, wPut2.Code)

	// GET first manifest by digest — should succeed via direct fallback
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo": testRepo, "image": testImage, "reference": dig1.String(),
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestGetHandler(&wGet, rGet)
	assert.Equal(t, http.StatusOK, wGet.Code,
		"GET by digest must return 200 even when wrapper link is stale")
	assert.Equal(t, dig1.String(), wGet.Header().Get("Docker-Content-Digest"),
		"response digest must match the requested digest")
}
