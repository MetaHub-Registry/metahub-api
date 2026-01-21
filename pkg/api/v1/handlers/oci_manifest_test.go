package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/distribution/distribution/manifest/schema2"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestPutOnlyWithLayerDelete(t *testing.T) {
	bs := utils.NewBootstrap()
	ctx := context.Background()
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestManifestPutOnlyWithLayerDelete",
	})
	defer span.End()
	debug := 0
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	//bs.PostTestSleep = 300
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
	/// Vars
	br.Add("repo", "test")
	br.Add("profile", "generic")
	br.Add("image", "conformance")
	br.Add("tag", "tagtest0")
	br.Add("mediaType", schema2.MediaTypeManifest)
	// Put with Data
	if w, r, e := getHttpTestManifestUploadPutOnlyHandler(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestPutHandler(w, r)
		assert.Equal(t, http.StatusCreated, w.Code)
		dig := w.Header().Get("Docker-Content-Digest")
		assert.Equal(t, "sha256:9ba2504eacad8f7fa4d2bfadcc406d1e7b7021985513944bf67a11fbb5f36dd2", dig)
		br.Add("digest", dig)
		br.Add("reference", dig)
	}
	br.Add("mediaType", "FAIL") // Forcing default media type in next test
	// Put with Data
	if w, r, e := getHttpTestManifestUploadPutOnlyHandler(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestPutHandler(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}

	// Head with Digest
	if w, r, e := getHttpTestManifestHeadByDigest(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestHeadHandler(w, r)
		assert.Equal(t, http.StatusOK, w.Code)

	}
	// Get with Digest
	br.Add("method", http.MethodGet)
	if w, r, e := getHttpTestManifestByDigest(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestGetHandler(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	// TODO: Fix Test for Tag Head
	// t.Log(" --- TEST HEAD w/ tag --- ")
	// logrus.SetLevel(logrus.DebugLevel)
	// // Head with Tag
	// if w, r, e := getHttpTestManifestHeadByTag(&br); e != nil {
	// 	t.Fatal(e)
	// } else {
	// 	h.ManifestHeadHandler(w, r)
	// 	assert.Equal(t, http.StatusOK, w.Code)
	// 	t.Log("Headers:", w.Header())
	// }
	// ManifestDelHandler
	if w, r, e := getHttpTestManifestDeleteByDigest(ctx, &br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestDelHandler(w, r)
		assert.Equal(t, http.StatusAccepted, w.Code)
	}
	if w, r, e := getHttpTestManifestHeadByDigest(&br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestHeadHandler(w, r)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
	if w, r, e := getHttpTestManifestDeleteByDigest(ctx, &br); e != nil {
		t.Fatal(e)
	} else {
		h.ManifestDelHandler(w, r)
		// TODO: Should not be found... :shrug
		assert.Equal(t, http.StatusAccepted, w.Code)
	}
	span.End()
}

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

func TestUploadManifestFTarmShortWithRemove(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	testTag := "arm64"
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
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	//bs.PostTestSleep = 120
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	/// Manifest
	mani := types.FT_arm_ManifestData()
	wManiPut, rManiPut := utils.CreateNewHTTPTest(ctx, http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	if wManiPut.Code != http.StatusCreated {
		t.Fatalf("Code: %d, expected %d", wManiPut.Code, http.StatusCreated)
	}
	digest := wManiPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, mani.Digest, digest)
	// Check for Manifest
	_, rManiGet := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": mani.Digest,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	gotMani, found, err := h.mag.ManifestGetByHTTPRequest(ctx, rManiGet)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Manifest not found after upload")
	}
	assert.Equal(t, mani.Digest, gotMani.Digest)

	// Remove
	_, rManiDel := utils.CreateNewHTTPTest(ctx, http.MethodDelete, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": digest,
	}, map[string]string{
		"accountName": token,
		"userName":    "qnib",
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	err = h.mag.ManifestDeleteByHTTPRequest(ctx, rManiDel)
	if err != nil {
		t.Fatal(err)
	}

	_, found, err = h.mag.ManifestGetByHTTPRequest(ctx, rManiGet)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
}

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
	l0bytes := types.FT_arm_Layer0Byte()
	l0ref := types.FT_arm_Layer0Reference()
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
	l1bytes := types.FT_arm_Layer1Byte()
	l1ref := types.FT_arm_Layer1Reference()
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
	mani := types.ManifestData{
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
	// cnt, err := h.mag.Database.CollectionTag_CountRelationships(ct.ID)
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
	mani := types.ManifestData{
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
}
