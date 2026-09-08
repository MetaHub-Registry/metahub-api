package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestManifestDigest_UnlinkedNeverFiveHundred pins the handler-side contract
// from openspec fix-digest-manifest-existence (OpenProject #41): HEAD/GET
// **by digest** of a manifest that exists but is not linked to the requesting
// profile must be answered with the content (Path Z digest-serve contract) or
// 404 — never a 5xx. The unlinked state broke `oras cp` destination
// pre-checks, which abort the copy on a 500.
//
// Fixture: push a manifest, then strip every Profile↔Manifest linkage edge
// (including via computed Indexes) to reproduce the outage graph state.
func TestManifestDigest_UnlinkedNeverFiveHundred(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	if err := h.mag.Minio.EmptyAll(); err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}

	mani := types.Manifest{
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
	expDigest := digest.FromBytes(mani.Data)

	wPut, rPut := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo": "oci", "image": "unlinked", "reference": "v1",
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wPut, rPut)
	require.Equal(t, http.StatusCreated, wPut.Code)

	// Strip every profile linkage so the manifest exists but is unlinked:
	// direct Profile<->Manifest edges and Profile->Index-[:REFERENCES]->Manifest.
	for _, q := range []string{
		fmt.Sprintf(`MATCH (md:Manifest {digest: "%s"})-[r]->(:Profile) DELETE r`, expDigest),
		fmt.Sprintf(`MATCH (:Profile)-[r]->(md:Manifest {digest: "%s"}) DELETE r`, expDigest),
		fmt.Sprintf(`MATCH (:Profile)-[r]->(ml:Index)-[:REFERENCES]->(md:Manifest {digest: "%s"}) DELETE r`, expDigest),
	} {
		if _, err := h.mag.Database.Execute(q); err != nil {
			t.Fatalf("failed to strip linkage: %v", err)
		}
	}

	// HEAD by digest — was 500 in the outage; must now be 200 (content
	// addressed, Path Z) or 404, never 5xx.
	wHead, rHead := utils.CreateNewHTTPTest(ctx, http.MethodHead, map[string]string{
		"repo": "oci", "image": "unlinked", "reference": expDigest.String(),
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Accept":        "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestHeadHandler(&wHead, rHead)
	assert.Less(t, wHead.Code, 500,
		"HEAD by digest of an unlinked manifest must never 5xx (body: %s)", wHead.Body.String())
	assert.Equal(t, http.StatusOK, wHead.Code,
		"digest-addressed content is served regardless of profile linkage (body: %s)", wHead.Body.String())
	assert.Equal(t, expDigest.String(), wHead.Header().Get("Docker-Content-Digest"))

	// GET by digest — same contract for the body-carrying request.
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo": "oci", "image": "unlinked", "reference": expDigest.String(),
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Accept":        "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestGetHandler(&wGet, rGet)
	assert.Less(t, wGet.Code, 500,
		"GET by digest of an unlinked manifest must never 5xx (body: %s)", wGet.Body.String())
	assert.Equal(t, http.StatusOK, wGet.Code,
		"digest-addressed content is served regardless of profile linkage (body: %s)", wGet.Body.String())
	assert.Equal(t, expDigest.String(), wGet.Header().Get("Docker-Content-Digest"))
}
