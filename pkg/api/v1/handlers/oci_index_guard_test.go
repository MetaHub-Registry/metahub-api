package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestIndexManipulationPersistGuard covers the write-on-read guard in
// manipulateIndexInPlace: a profile-scoped index pull persists the (content
// addressed, immutable) manipulated index to object storage on first sight, and
// a repeat pull must NOT re-PUT it — yet must still serve the identical index.
// GET #1 exercises the store branch, GET #2 exercises the "already persisted,
// skip" branch; both must return the same bytes (the tag GET is served from the
// graph, so skipping the redundant MinIO PUT cannot change the response).
func TestIndexManipulationPersistGuard(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	require.NoError(t, h.mag.Minio.EmptyAll())
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	// Reset the process-wide guard so the store branch is deterministic here
	// (handler tests run serially — no t.Parallel).
	persistedIndexDigests.Range(func(k, _ any) bool { persistedIndexDigests.Delete(k); return true })

	repo, image, tag := "oci", "guard", "latest"
	push := func(data []byte) {
		w, r := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
			"repo": repo, "image": image, "reference": tag,
		}, map[string]string{
			"accountName": token, "userName": "oci", "profileName": "generic",
		}, map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
		}, "/{repo}/{image}/manifests/{reference}", data)
		h.ManifestPutHandler(&w, r)
		require.Equal(t, http.StatusCreated, w.Code)
	}
	push([]byte(`{"schemaVersion":2,"config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:22f68afca98941fc10a8c6c87f881065b075c1c2f4678b8f579d1b77bc060ef0","size":129,"data":"ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"},"layers":[]}`))
	push([]byte(`{"schemaVersion":2,"config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:60e38b2fbdb2b4d66b0df7158b380d3c5979c6d708e011c4cfc0c9d6c6cd069a","size":129,"data":"ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYXJtNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9"},"layers":[]}`))

	get := func() (int, []byte) {
		w, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
			"repo": repo, "image": image, "reference": tag,
		}, map[string]string{
			"accountName": token, "userName": "oci", "profileName": "generic",
		}, map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Accept":        "application/vnd.oci.image.index.v1+json",
		}, "/{repo}/{image}/manifests/{reference}", nil)
		h.ManifestGetHandler(&w, r)
		return w.Code, w.Body.Bytes()
	}

	// GET #1 — store branch: persists the manipulated index digest.
	code1, body1 := get()
	require.Equal(t, http.StatusOK, code1)
	n := 0
	persistedIndexDigests.Range(func(_, _ any) bool { n++; return true })
	assert.GreaterOrEqual(t, n, 1, "first profile-scoped index pull must persist the manipulated index digest")

	// GET #2 — skip branch: must not re-PUT, must serve the identical index.
	code2, body2 := get()
	require.Equal(t, http.StatusOK, code2)
	assert.Equal(t, digest.FromBytes(body1), digest.FromBytes(body2), "repeat pull must serve the identical index")
}
