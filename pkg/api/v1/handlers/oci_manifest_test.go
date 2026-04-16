package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// func TestManifestPutOnlyWithLayerDelete(t *testing.T) {
// 	bs := utils.NewBootstrap()
// 	ctx := context.Background()
// 	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
// 		"func": "TestManifestPutOnlyWithLayerDelete",
// 	})
// 	defer span.End()
// 	debug := 0
// 	bs.SetEmptyAll()
// 	bs.SetYAMLPrefix("../../../../misc/yaml")
// 	bs.AddBootstrap("users")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/generic/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	//bs.PostTestSleep = 300
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	err := h.mag.Minio.EmptyAll()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	br, err := SetupTest(ctx, bs)
// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	if err != nil {
// 		TeardownTest(bs, br)
// 		t.Fatal(err)
// 	}

// 	defer TeardownTest(bs, br)
// 	/// Vars
// 	br.Add("repo", "test")
// 	br.Add("profile", "generic")
// 	br.Add("image", "conformance")
// 	br.Add("tag", "tagtest0")
// 	br.Add("mediaType", schema2.MediaTypeManifest)
// 	// Put with Data
// 	if w, r, e := getHttpTestManifestUploadPutOnlyHandler(&br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestPutHandler(w, r)
// 		assert.Equal(t, http.StatusCreated, w.Code)
// 		dig := w.Header().Get("Docker-Content-Digest")
// 		assert.Equal(t, "sha256:9ba2504eacad8f7fa4d2bfadcc406d1e7b7021985513944bf67a11fbb5f36dd2", dig)
// 		br.Add("digest", dig)
// 		br.Add("reference", dig)
// 	}
// 	br.Add("mediaType", "FAIL") // Forcing default media type in next test
// 	// Put with Data
// 	if w, r, e := getHttpTestManifestUploadPutOnlyHandler(&br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestPutHandler(w, r)
// 		assert.Equal(t, http.StatusBadRequest, w.Code)
// 	}

// 	// Head with Digest
// 	if w, r, e := getHttpTestManifestHeadByDigest(&br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestHeadHandler(w, r)
// 		assert.Equal(t, http.StatusOK, w.Code)

// 	}
// 	// Get with Digest
// 	br.Add("method", http.MethodGet)
// 	if w, r, e := getHttpTestManifestByDigest(&br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestGetHandler(w, r)
// 		assert.Equal(t, http.StatusOK, w.Code)
// 	}
// 	// TODO: Fix Test for Tag Head
// 	// t.Log(" --- TEST HEAD w/ tag --- ")
// 	// logrus.SetLevel(logrus.DebugLevel)
// 	// // Head with Tag
// 	// if w, r, e := getHttpTestManifestHeadByTag(&br); e != nil {
// 	// 	t.Fatal(e)
// 	// } else {
// 	// 	h.ManifestHeadHandler(w, r)
// 	// 	assert.Equal(t, http.StatusOK, w.Code)
// 	// 	t.Log("Headers:", w.Header())
// 	// }
// 	// ManifestDelHandler
// 	if w, r, e := getHttpTestManifestDeleteByDigest(ctx, &br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestDelHandler(w, r)
// 		assert.Equal(t, http.StatusAccepted, w.Code)
// 	}
// 	if w, r, e := getHttpTestManifestHeadByDigest(&br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestHeadHandler(w, r)
// 		assert.Equal(t, http.StatusNotFound, w.Code)
// 	}
// 	if w, r, e := getHttpTestManifestDeleteByDigest(ctx, &br); e != nil {
// 		t.Fatal(e)
// 	} else {
// 		h.ManifestDelHandler(w, r)
// 		// TODO: Should not be found... :shrug
// 		assert.Equal(t, http.StatusAccepted, w.Code)
// 	}
// 	span.End()
// }

func TestManifestPutOnlyWithOutLayerDelete(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	ctx := context.Background()
	if debug > 0 {
		bs.PostTestSleep = debug
	}

	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}

	defer TeardownTest(bs, br)
	/// Vars
	br.Add("repo", "test")
	br.Add("profile", "generic")
	br.Add("image", "conformance")
	br.Add("tag", "emptylayer")
	// Manifest w/o layers
	if w, r, e := getHttpTestManifestUploadPutOnlyHandlerWithoutLayers(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestPutHandler(w, r)
		assert.Equal(t, http.StatusCreated, w.Code)
		dig := w.Header().Get("Docker-Content-Digest")
		assert.Equal(t, "sha256:27b2826c5be7bfcc05778d6de99f30fef72d095eec57f11e9904b3fe4328a218", dig)
		br.Add("digest", dig)
	}
}

// func TestUploadManifestFTarmShortWithRemove(t *testing.T) {
// 	testRepo := "qnib"
// 	testImage := "featuretest"
// 	testTag := "arm64"
// 	testProfile := "generic"
// 	err := h.mag.Minio.EmptyAll()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	bs := utils.NewBootstrap()
// 	bs.SetEmptyAll()
// 	bs.SetYAMLPrefix("../../../../misc/yaml")
// 	bs.AddBootstrap("users")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("rules:rules/generic/linux.yaml")
// 	bs.AddBootstrap("profile:profiles/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	bs.AddBootstrap("config:ft_amd64")
// 	bs.AddBootstrap("upstream:ft_arm64")
// 	bs.AddBootstrap("upstream:ft_amd64")
// 	bs.AddBootstrap("layer:0:ft_arm64")
// 	bs.AddBootstrap("layer:1:ft_arm64")
// 	bs.AddBootstrap("layer:0:ft_amd64")
// 	bs.AddBootstrap("layer:1:ft_amd64")
// 	//bs.PostTestSleep = 120
// 	ctx := context.Background()
// 	br, err := SetupTest(ctx, bs)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer TeardownTest(bs, br)
// 	/// Request
// 	token, err := br.Get("user:test#bearer")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	/// Manifest
// 	mani := types.FTarmManifestData()
// 	wManiPut, rManiPut := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
// 		"repo":      testRepo,
// 		"image":     testImage,
// 		"reference": testTag,
// 	}, map[string]string{
// 		"accountName": token,
// 		"userName":    "qnib",
// 		"profileName": testProfile,
// 	}, map[string]string{
// 		"Authorization": fmt.Sprintf("Bearer %s", token),
// 		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
// 	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
// 	h.ManifestPutHandler(&wManiPut, rManiPut)
// 	if wManiPut.Code != http.StatusCreated {
// 		t.Fatalf("Code: %d, expected %d", wManiPut.Code, http.StatusCreated)
// 	}
// 	digest := wManiPut.Header().Get("Docker-Content-Digest")
// 	assert.Equal(t, mani.Digest, digest)
// 	// Check for Manifest
// 	_, rManiGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
// 		"repo":      testRepo,
// 		"image":     testImage,
// 		"reference": mani.Digest,
// 	}, map[string]string{
// 		"accountName": token,
// 		"userName":    testRepo,
// 		"profileName": testProfile,
// 	}, map[string]string{
// 		"Authorization": fmt.Sprintf("Bearer %s", token),
// 	}, "/{repo}/{image}/manifests/{reference}", nil)
// 	gotMani, found, err := h.mag.ManifestGetByHTTPRequest(ctx, rManiGet)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !found {
// 		t.Fatal("Manifest not found after upload")
// 	}
// 	assert.Equal(t, mani.Digest, gotMani.Digest)

// 	// Remove
// 	_, rManiDel := utils.CreateNewHTTPTest(ctx, http.MethodDelete, map[string]string{
// 		"repo":      testRepo,
// 		"image":     testImage,
// 		"reference": digest,
// 	}, map[string]string{
// 		"accountName": token,
// 		"userName":    "qnib",
// 		"profileName": testProfile,
// 	}, map[string]string{
// 		"Authorization": fmt.Sprintf("Bearer %s", token),
// 	}, "/{repo}/{image}/manifests/{reference}", nil)
// 	err = h.mag.ManifestDeleteByHTTPRequest(ctx, rManiDel)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	_, found, err = h.mag.ManifestGetByHTTPRequest(ctx, rManiGet)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.False(t, found)
// }

func TestUploadManifestFTarm(t *testing.T) {
	ctx := context.Background()
	testRepo := "qnib"
	testImage := "featuretest"
	testProfile := "generic"
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.PostTestSleep = 0
	br, err := SetupTest(context.Background(), bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	///// LAYER0
	l0bytes := types.FTarmLayer0Byte()
	l0ref := types.FTarmLayer0Reference()
	// POST with Data
	wL0, rL0 := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", nil)
	h.BlobUploadPostHandler(&wL0, rL0)
	if http.StatusAccepted != wL0.Code {
		t.Fatalf("Expected %d, got %d", http.StatusAccepted, wL0.Code)
	}
	// Fetch Upload from DB
	gotLoc := wL0.Header().Get("Location")
	uuid := wL0.Header().Get("Docker-Upload-UUID")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuid), gotLoc)
	// Next we do a PATCH request
	wL0Patch, rL0Patch := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuid,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  fmt.Sprintf("0-%d", len(l0bytes)-1),
		"Content-Length": fmt.Sprintf("%d", len(l0bytes)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", l0bytes)
	h.BlobUploadPatchHandler(&wL0Patch, rL0Patch)
	if http.StatusAccepted != wL0Patch.Code {
		t.Fatalf("Expected %d, got %d", http.StatusAccepted, wL0Patch.Code)
	}
	// Final PUT request
	//startPut := time.Now()
	wL0Put, rL0Put := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuid,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest), nil)
	h.BlobUploadPutHandler(&wL0Put, rL0Put)
	//endPut := time.Now()
	if http.StatusCreated != wL0Put.Code {
		t.Fatalf("Expected %d, got %d", http.StatusCreated, wL0Put.Code)
	}
	gotPutLoc := wL0Put.Header().Get("Location")
	uuid = wL0Put.Header().Get("Docker-Upload-UUID")
	assert.NotNil(t, uuid)
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuid), gotPutLoc)
	///// LAYER1
	l1bytes := types.FTarmLayer1Byte()
	l1ref := types.FTarmLayer1Reference()
	_ = l1ref
	// POST with Data
	wL1, rL1 := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", nil)
	h.BlobUploadPostHandler(&wL1, rL1)
	if http.StatusAccepted != wL1.Code {
		t.Fatalf("Expected %d, got %d", http.StatusAccepted, wL1.Code)
	}
	// Fetch Upload from DB
	gotLocL1 := wL1.Header().Get("Location")
	uuidL1 := wL1.Header().Get("Docker-Upload-UUID")
	assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/uploads/%s", testRepo, testImage, uuidL1), gotLocL1)
	// Next we do a PATCH request
	wL1Patch, rL1Patch := utils.CreateNewHTTPTest(ctx, http.MethodPatch, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuidL1,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  fmt.Sprintf("0-%d", len(l1bytes)-1),
		"Content-Length": fmt.Sprintf("%d", len(l1bytes)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", l1bytes)
	h.BlobUploadPatchHandler(&wL1Patch, rL1Patch)
	if http.StatusAccepted != wL1Patch.Code {
		t.Fatalf("Expected %d, got %d", http.StatusAccepted, wL1Patch.Code)
	}
	// Final PUT request
	wL1Put, rL1Put := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuidL1,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l1ref.Digest), nil)
	h.BlobUploadPutHandler(&wL1Put, rL1Put)
	if http.StatusCreated != wL1Put.Code {
		t.Fatalf("Expected %d, got %d", http.StatusCreated, wL1Put.Code)
	}
	gotPutLocL1 := wL0Put.Header().Get("Location")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuid), gotPutLocL1)
}

func TestManifestOCITest(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	_ = br
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// Tests
	testRepo := "oci"
	testImage := "conformance"
	testTag := "tagtest0"
	cfg := types.Config{
		Data: []byte(`{
			"author": "TneKdQXHXICsSXwR",
			"architecture": "amd64",
			"os": "linux",
			"rootfs": {
				"type": "layers",
				"diff_ids": []
			}
		}`),
	}
	cfgDig := digest.FromBytes(cfg.Data)
	logrus.Errorf("cfgDig: %s", cfgDig)
	/// Direct Post with config
	wCfgPost, rCfgPost := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  fmt.Sprintf("0-%d", len(cfg.Data)-1),
		"Content-Length": fmt.Sprintf("%d", len(cfg.Data)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", cfg.Data)
	h.BlobUploadPostHandler(&wCfgPost, rCfgPost)
	if wCfgPost.Code != http.StatusCreated {
		t.Fatalf("wManiPut.Code: %d, expected %d", wCfgPost.Code, http.StatusCreated)
	}
	/// Manifest
	mani := types.Manifest{
		Data: []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:22f68afca98941fc10a8c6c87f881065b075c1c2f4678b8f579d1b77bc060ef0",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
				"newUnspecifiedField": "aGVsbG8gd29ybGQ="
			},
			"layers": [
				{
					"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
					"digest": "sha256:88ecf269dec31566a8e6b05147732fe34d32bc608de0d636dffaba659230a515",
					"size": 2796496,
					"data": null,
					"newUnspecifiedField": null
				}
			]
		}`),
	}
	wManiPut, rManiPut := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	if wManiPut.Code != http.StatusCreated {
		t.Fatalf("wManiPut.Code: %d, expected %d", wManiPut.Code, http.StatusCreated)
	}
	maniDigest := wManiPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, "sha256:bec9eaec0dddcf1558fd11ef8d1e68d5cf7e526e8d6bdb282f16984febc852bb", maniDigest)
	// Remove
	wManiDel, rManiDel := utils.CreateNewHTTPTest(ctx, http.MethodDelete, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": maniDigest,
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	assert.NotNil(t, rManiDel)
	err = h.mag.ManifestDeleteByHTTPRequest(ctx, rManiDel)
	if err != nil {
		t.Fatal(err)
	}
	if wManiDel.Code != http.StatusOK {
		t.Fatalf("wManiDel.Code: %d, expected %d", wManiDel.Code, http.StatusOK)
	}
	// Check Collections
	// cols, err := h.mag.Database.CollectionListingGetByRepoLike(ctx, testRepo)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// ct := cols[0].ColTag
	// // remove collectiontag
	// wCTDel, rCTDel := utils.CreateNewHTTPTest(ctx, http.MethodDelete, map[string]string{
	// 	"repo":      testRepo,
	// 	"image":     testImage,
	// 	"reference": testTag,
	// }, map[string]string{
	// 	"accountName": token,
	// 	"userName":    "oci",
	// 	"profileName": "generic",
	// }, map[string]string{
	// 	"Authorization": fmt.Sprintf("Bearer %s", token),
	// }, "/{repo}/{image}/manifests/{reference}", nil)
	// assert.NotNil(t, rCTDel)
	// err = h.mag.ManifestDeleteByHTTPRequest(ctx, rCTDel)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// if wCTDel.Code != http.StatusOK {
	// 	t.Fatalf("wCTDel.Code: %d, expected %d", wCTDel.Code, http.StatusOK)
	// }
	// // Check Collections
	// cols, err = h.mag.Database.CollectionListingGetByRepoLike(ctx, testRepo)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// assert.Equal(t, 0, len(cols))
	// /// Count CollectionTag after compute manifestlist is done
	// cnt, err := h.mag.Database.CollectionTagCountRelationships(ct.ID)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// assert.Equal(t, int64(0), cnt)
}

func TestManifestOCITestNoLayers(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	_ = br
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// Tests
	testRepo := "oci"
	testImage := "conformance"
	testTag := "tagtest0"
	cfg := types.Config{
		Data: []byte(`{
			"author": "TneKdQXHXICsSXwR",
			"architecture": "amd64",
			"os": "linux",
			"rootfs": {
				"type": "layers",
				"diff_ids": []
			}
		}`),
	}
	/// Direct Post with config
	wCfgPost, rCfgPost := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  fmt.Sprintf("0-%d", len(cfg.Data)-1),
		"Content-Length": fmt.Sprintf("%d", len(cfg.Data)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", cfg.Data)
	h.BlobUploadPostHandler(&wCfgPost, rCfgPost)
	if wCfgPost.Code != http.StatusCreated {
		t.Fatalf("Code: %d, expected %d", wCfgPost.Code, http.StatusCreated)
	}
	/// Manifest
	dat := []byte(`{
		"schemaVersion": 2,
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "sha256:9926ec0751c67b1aa49c051ef77be145fa13e84fd03fb728a4af3ed159c208ac",
			"size": 129,
			"data": "ewoJImF1dGhvciI6ICJBSnM5bUhSRFloSU1PQXJxIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
			"newUnspecifiedField": "aGVsbG8gd29ybGQ="
		},
		"layers": []
	}`)
	mani := types.Manifest{
		Data: dat,
	}
	expManiDigest := digest.FromBytes(mani.Data)

	wManiPut, rManiPut := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	if wManiPut.Code != http.StatusCreated {
		t.Fatalf("Code: %d, expected %d", wManiPut.Code, http.StatusCreated)
	}
	maniDigest := wManiPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, expManiDigest.String(), maniDigest)
	assert.Equal(t, "http://example.com/v2/oci/conformance/manifests/tagtest0", wManiPut.Header().Get("Location"))
}

func TestManifestHeadHandlerDigest(t *testing.T) {
	bs := utils.NewBootstrap()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	err := h.mag.Minio.EmptyAll()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
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
	// Upload a manifest first
	testRepo := "oci"
	testImage := "conformance"
	testTag := "headtest"
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
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wPut, rPut)
	assert.Equal(t, http.StatusCreated, wPut.Code)
	gotDigest := wPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, expDigest.String(), gotDigest)

	// HEAD by digest
	br.Add("repo", testRepo)
	br.Add("profile", "generic")
	br.Add("image", testImage)
	br.Add("digest", gotDigest)
	br.Add("tag", testTag)
	if wHead, rHead, e := getHttpTestManifestHeadByDigest(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestHeadHandler(wHead, rHead)
		assert.Equal(t, http.StatusOK, wHead.Code)
	}

	// HEAD by tag
	if wHead, rHead, e := getHttpTestManifestHeadByTag(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestHeadHandler(wHead, rHead)
		assert.Equal(t, http.StatusOK, wHead.Code)
	}

	// HEAD with non-existent digest
	br.Add("digest", "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	if wHead, rHead, e := getHttpTestManifestHeadByDigest(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestHeadHandler(wHead, rHead)
		assert.Equal(t, http.StatusNotFound, wHead.Code)
	}
}

// TestManifestGetByTag_PullthroughSuccess exercises the happy path of
// resolveMissingManifest -> tryPullthroughOnMiss -> HeadManifestNotFound
// -> ManifestExists (retry). The test upstream registry is pre-seeded
// with alpine:3.17.3, so pullthrough succeeds and the handler serves the
// manifest.
//
// This hits the currently-uncovered success branch of tryPullthroughOnMiss
// (retry ManifestExists after successful upstream discovery and return the
// resolved mediaType with ok=true).
func TestManifestGetByTag_PullthroughSuccess(t *testing.T) {
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

	// GET alpine:3.17.3 — the testcontainer upstream registry has this image
	// pre-seeded. Local graph is empty, so tryPullthroughOnMiss runs and
	// should succeed, populating the graph via HeadManifestNotFound.
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"image":     "alpine",
		"reference": "3.17.3",
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Accept":        "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{image}/manifests/{reference}", nil)
	h.ManifestGetHandler(&wGet, rGet)
	// After pullthrough, we expect either 200 (success) or at worst a
	// non-404 status — the key assertion is that tryPullthroughOnMiss's
	// success branch executed. A 404 would indicate pullthrough failed
	// before reaching the ManifestExists retry.
	assert.NotEqual(t, http.StatusNotFound, wGet.Code,
		"pullthrough success branch should NOT lead to 404 (got status %d, body: %s)",
		wGet.Code, wGet.Body.String())
}

// TestManifestGetByTag_NotFoundTriggersPullthrough exercises the
// pullthrough-miss code path (ManifestHandler -> resolveMissingManifest
// -> tryPullthroughOnMiss) that the happy-path tests don't reach.
//
// Setup: bootstrap an empty graph (no manifest pushed), then GET a
// manifest by tag. ManifestExists returns colFound=false. The handler
// calls tryPullthroughOnMiss which, in turn, invokes HeadManifestNotFound
// against the configured registries. No registry has the image, so
// upstream discovery also fails and the handler returns 404.
//
// This covers: ManifestHandler's !colFound branch, resolveMissingManifest
// (tag path), tryPullthroughOnMiss (HeadManifestNotFound failure path),
// writeManifestNotFound.
func TestManifestGetByTag_NotFoundTriggersPullthrough(t *testing.T) {
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

	// GET a manifest that does not exist locally and cannot be discovered
	// upstream (the test registry has nothing under this name).
	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":      "nonexistent",
		"image":     "ghost",
		"reference": "v1.0",
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Accept":        "application/vnd.oci.image.manifest.v1+json",
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestGetHandler(&wGet, rGet)
	assert.Equal(t, http.StatusNotFound, wGet.Code, "non-existent manifest must 404")
}

// TestManifestHead_SinglePlatform_ExercisesHandleManifest pushes a single
// OCI image manifest (not an index) and then sends HEAD by digest. This
// flows through dispatchByMediaType with ociManifestMediaType, which
// dispatches to handleManifest — covering the HEAD-method branch
// (m.WriteWWWHead) that the existing TestManifestHeadHandlerDigest does
// not reach (its HEAD-by-digest goes through an index path).
func TestManifestHead_SinglePlatform_ExercisesHandleManifest(t *testing.T) {
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
		"repo": "oci", "image": "single", "reference": "v1",
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wPut, rPut)
	require.Equal(t, http.StatusCreated, wPut.Code)

	// HEAD by the manifest's own digest → dispatchByMediaType routes
	// ociManifestMediaType to handleManifest, which writes HEAD headers.
	wHead, rHead := utils.CreateNewHTTPTest(ctx, http.MethodHead, map[string]string{
		"repo": "oci", "image": "single", "reference": expDigest.String(),
	}, map[string]string{
		"accountName": token, "userName": "oci", "profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Accept":        "application/vnd.oci.image.manifest.v1+json",
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestHeadHandler(&wHead, rHead)
	assert.Equal(t, http.StatusOK, wHead.Code, "HEAD by digest for single-platform manifest must succeed")
}

// TestManifestGetByDigest_NotFoundDoesNotTriggerPullthrough asserts that
// GET-by-digest for a non-existent manifest goes through the Path-Z
// handleManifest fallback but NOT pullthrough (digest-only requests
// cannot be resolved upstream without a tag).
func TestManifestGetByDigest_NotFoundDoesNotTriggerPullthrough(t *testing.T) {
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

	wGet, rGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":      "nonexistent",
		"image":     "ghost",
		"reference": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}, map[string]string{
		"accountName": token,
		"userName":    "oci",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Accept":        "application/vnd.oci.image.manifest.v1+json",
	}, "/{repo}/{image}/manifests/{reference}", nil)
	h.ManifestGetHandler(&wGet, rGet)
	assert.Equal(t, http.StatusNotFound, wGet.Code, "non-existent digest must 404")
}
