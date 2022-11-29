package handlers

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_UploadManifest_FTarm_Short(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	testTag := "arm64"
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	err := h.mag.Minio.EmptyAll()
	assert.NoError(t, err)
	err = h.mag.SQlite.EmptyData(true)
	assert.NoError(t, err)
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	// TODO: REMOVE
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	/// Manifest
	mani := types.FT_arm_Manifest()
	wManiPut, rManiPut := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	assert.Equal(t, http.StatusCreated, wManiPut.Code)
	/*
		// Check DB
		// Upstream
		q := fmt.Sprintf(`SELECT id,repo,tag FROM main.upstreams WHERE digest="%s";`, mani.Digest)
		rows, err := h.mag.SQlite.Query(q)
		assert.NoError(t, err)
		defer rows.Close()
		var id int
		var repo, tag string
		if rows.Next() {
			err = rows.Scan(&id, &repo, &tag)
			assert.NoError(t, err)
		} else {
			t.Fatal("No upstream found")
		}
		rows.Close()
		assert.Equal(t, "qnib/featuretest", repo)
		assert.Equal(t, "arm64", tag)
		////// FETCH MANIFEST_LIST via Collection
		/// Direct DB
		req := types.Request{
			Repo:    fmt.Sprintf("%s/%s", testRepo, testImage),
			Tag:     testTag,
			Profile: "generic",
		}
		_, found, err := h.mag.SQlite.GetTypesManifestListByRequest(req)
		assert.NoError(t, err)
		assert.True(t, found)
	*/

}

func Test_UploadManifest_FTarm(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	testTag := "latest"
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	err := h.mag.Minio.EmptyAll()
	assert.NoError(t, err)
	err = h.mag.SQlite.EmptyData(true)
	assert.NoError(t, err)
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	///// LAYER0
	l0bytes := types.FT_arm_Layer0Byte()
	l0ref := types.FT_arm_Layer0Reference()
	// POST with Data
	wL0, rL0 := createNewHTTPTest(http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", l0bytes)

	h.BlobUploadPostHandler(&wL0, rL0)
	assert.Equal(t, http.StatusAccepted, wL0.Code)
	// Fetch Upload from DB
	gotLoc := wL0.Header().Get("Location")
	q := fmt.Sprintf(`SELECT uuid,uploaded_by FROM main.blob_uploads WHERE location="%s";`, gotLoc)
	rows, err := h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	var uuid, uploadedBy string
	for rows.Next() {
		err = rows.Scan(&uuid, &uploadedBy)
		assert.NoError(t, err)
	}
	assert.Equal(t, "test", uploadedBy)
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuid), gotLoc)
	// Next we do a PATCH request
	wL0Patch, rL0Patch := createNewHTTPTest(http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuid,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  "1-1",
		"Content-Length": fmt.Sprintf("%d", len(l0bytes)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", l0bytes)
	h.BlobUploadPatchHandler(&wL0Patch, rL0Patch)
	assert.Equal(t, http.StatusAccepted, wL0Patch.Code)
	q = fmt.Sprintf(`SELECT uuid FROM main.blob_uploads WHERE digest="%s";`, l0ref.Digest)
	rows, err = h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	var gotUuid string
	for rows.Next() {
		err = rows.Scan(&gotUuid)
		assert.NoError(t, err)
	}
	assert.Equal(t, uuid, gotUuid)
	// Final PUT request
	startPut := time.Now()
	wL0Put, rL0Put := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuid,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest), nil)
	h.BlobUploadPutHandler(&wL0Put, rL0Put)
	endPut := time.Now()
	assert.Equal(t, http.StatusCreated, wL0Put.Code)
	gotPutLoc := wL0Put.Header().Get("Location")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuid), gotPutLoc)
	q = fmt.Sprintf(`SELECT finished_at FROM main.blob_uploads WHERE digest="%s";`, l0ref.Digest)
	rows, err = h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	var gotFinished time.Time
	for rows.Next() {
		err = rows.Scan(&gotFinished)
		assert.NoError(t, err)
	}
	assert.Greater(t, gotFinished, startPut)
	assert.Greater(t, endPut, gotFinished)
	///// LAYER1
	l1bytes := types.FT_arm_Layer1Byte()
	l1ref := types.FT_arm_Layer1Reference()
	_ = l1ref
	// POST with Data
	wL1, rL1 := createNewHTTPTest(http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", l1bytes)
	h.BlobUploadPostHandler(&wL1, rL1)
	assert.Equal(t, http.StatusAccepted, wL1.Code)
	// Fetch Upload from DB
	gotLocL1 := wL1.Header().Get("Location")
	q = fmt.Sprintf(`SELECT uuid, uploaded_by FROM main.blob_uploads WHERE location="%s";`, gotLocL1)
	rows, err = h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	var uuidL1, uploadedByL1 string
	for rows.Next() {
		err = rows.Scan(&uuidL1, &uploadedByL1)
		assert.NoError(t, err)
	}
	assert.Equal(t, "test", uploadedByL1)
	assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/uploads/%s", testRepo, testImage, uuidL1), gotLocL1)
	// Next we do a PATCH request
	wL1Patch, rL1Patch := createNewHTTPTest(http.MethodPatch, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuidL1,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  "1-1",
		"Content-Length": fmt.Sprintf("%d", len(l1bytes)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", l1bytes)
	h.BlobUploadPatchHandler(&wL1Patch, rL1Patch)
	assert.Equal(t, http.StatusAccepted, wL1Patch.Code)
	// Final PUT request
	wL1Put, rL1Put := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuidL1,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l1ref.Digest), nil)
	h.BlobUploadPutHandler(&wL1Put, rL1Put)
	assert.Equal(t, http.StatusCreated, wL1Put.Code)
	gotPutLocL1 := wL0Put.Header().Get("Location")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuid), gotPutLocL1)
	/////// CONFIG
	cfgBytes := types.FT_arm_ConfigByte()
	cfg := types.FT_arm_Config()
	// POST
	wCfgPost, rCfgPost := createNewHTTPTest(http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", cfgBytes)
	h.BlobUploadPostHandler(&wCfgPost, rCfgPost)
	assert.Equal(t, http.StatusAccepted, wCfgPost.Code)
	// Fetch Upload from DB
	gotLocCfg := wCfgPost.Header().Get("Location")
	q = fmt.Sprintf(`SELECT uuid, blob_type, uploaded_by FROM main.blob_uploads WHERE location="%s";`, gotLocCfg)
	rows, err = h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	var uuidCfg, uploadedByCfg, blobType string
	for rows.Next() {
		err = rows.Scan(&uuidCfg, &blobType, &uploadedByCfg)
		assert.NoError(t, err)
	}
	rows.Close()
	assert.Equal(t, "", blobType)
	assert.Equal(t, "test", uploadedByCfg)
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuidCfg), gotLocCfg)
	/// PATCH with config
	wCfgPatch, rCfgPatch := createNewHTTPTest(http.MethodPatch, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuidCfg,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  "1-1",
		"Content-Length": fmt.Sprintf("%d", len(cfgBytes)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", cfgBytes)
	h.BlobUploadPatchHandler(&wCfgPatch, rCfgPatch)
	assert.Equal(t, http.StatusAccepted, wCfgPatch.Code)
	q = fmt.Sprintf(`SELECT blob_type FROM main.blob_uploads WHERE location="%s";`, gotLocCfg)
	rows, err = h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&blobType)
		assert.NoError(t, err)
	}
	rows.Close()
	assert.Equal(t, "config", blobType)
	// Final PUT request
	wCfgPut, rCfgPut := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":  testRepo,
		"image": testImage,
		"uuid":  uuidCfg,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", cfg.Digest), nil)
	h.BlobUploadPutHandler(&wCfgPut, rCfgPut)
	assert.Equal(t, http.StatusCreated, wCfgPut.Code)
	gotPutLocCfg := wCfgPut.Header().Get("Location")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/featuretest/blobs/uploads/%s", uuidCfg), gotPutLocCfg)
	/// Manifest
	mani := types.FT_arm_Manifest()
	wManiPut, rManiPut := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	assert.Equal(t, http.StatusCreated, wManiPut.Code)
	// Check DB
	// Upstream
	q = fmt.Sprintf(`SELECT id,repo,tag FROM main.upstreams WHERE digest="%s";`, mani.Digest)
	rows, err = h.mag.SQlite.Query(q)
	assert.NoError(t, err)
	defer rows.Close()
	var id int
	var repo, tag string
	if rows.Next() {
		err = rows.Scan(&id, &repo, &tag)
		assert.NoError(t, err)
	} else {
		t.Fatal("No upstream found")
	}
	rows.Close()
	assert.Equal(t, "qnib/featuretest", repo)
	//assert.Equal(t, "latest-linux.arm64", tag)
	assert.Equal(t, "latest", tag)
	////// FETCH MANIFEST_LIST via Collection
	/// Direct DB
	req := types.Request{
		Repo:    fmt.Sprintf("%s/%s", testRepo, testImage),
		Tag:     testTag,
		Profile: "generic",
	}
	_, found, err := h.mag.SQlite.GetTypesManifestListByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	//assert.Len(t, ml.Manifests, 1)
	//assert.Equal(t, mani.Digest, ml.Manifests[0].Digest)
	/// via HTTPRequest
	_, rMLGet := createNewHTTPTest(http.MethodGet, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)

	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(rMLGet)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Len(t, ml.Manifests, 1)
	assert.Equal(t, mani.Digest, ml.Manifests[0].Digest)
	/// Fetch Manifest
	_, rManiGet := createNewHTTPTest(http.MethodGet, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": mani.Digest,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	gotMani, found, err := h.mag.Manifest_GetByHTTPRequest(rManiGet)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, mani.Digest, gotMani.Digest)
	// Fetch Manifest by tag
	_, rManiTagGet := createNewHTTPTest(http.MethodGet, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	gotTagMani, foundTag, err := h.mag.Manifest_GetByHTTPRequest(rManiTagGet)
	assert.NoError(t, err)
	assert.True(t, foundTag)
	assert.Equal(t, mani.Digest, gotTagMani.Digest)
}

func Test_Manifest_OCITest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	//bs.SetSqlite("Desktop/metahub.db")
	err := h.mag.Minio.EmptyAll()
	assert.NoError(t, err)
	err = h.mag.SQlite.EmptyData(true)
	assert.NoError(t, err)
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
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
	wCfgPost, rCfgPost := createNewHTTPTest(http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  "1-1",
		"Content-Length": fmt.Sprintf("%d", len(cfg.Data)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", cfg.Data)
	h.BlobUploadPostHandler(&wCfgPost, rCfgPost)
	assert.Equal(t, http.StatusCreated, wCfgPost.Code)
	/// Manifest
	mani := types.Manifest{
		Data: []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:b9720327d8aaf6b8896095a1630f3828c3912177a4e961171c203081a1b86de9",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJUbmVLZFFYSFhJQ3NTWHdSIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
				"newUnspecifiedField": "aGVsbG8gd29ybGQ="
			},
			"layers": [
				{
					"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
					"digest": "sha256:48acff1d91752e957527c1b5416e7376d910bcacf01b9441175f8c270e35c183",
					"size": 168,
					"data": null,
					"newUnspecifiedField": null
				}
			]
		}`),
	}

	wManiPut, rManiPut := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	assert.Equal(t, http.StatusCreated, wManiPut.Code)
	maniDigest := wManiPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, "sha256:f6db1c89670168c92e83c9954f680129f08d3e5b0e97d3951a07553c005139a7", maniDigest)
	// Remove
	/*
		wManiDel, rManiDel := createNewHTTPTest(http.MethodDelete, map[string]string{
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
		err = h.mag.Manifest_DeleteByHTTPRequest(rManiDel)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, wManiDel.Code)
	*/
}

func Test_Manifest_OCITest_NoLayers(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	//bs.SetSqlite("Desktop/metahub.db")
	err := h.mag.Minio.EmptyAll()
	assert.NoError(t, err)
	err = h.mag.SQlite.EmptyData(true)
	assert.NoError(t, err)
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
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
	wCfgPost, rCfgPost := createNewHTTPTest(http.MethodPost, map[string]string{
		"repo":  testRepo,
		"image": testImage,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization":  fmt.Sprintf("Bearer %s", token),
		"Content-Type":   "application/octet-stream",
		"Content-Range":  "1-1",
		"Content-Length": fmt.Sprintf("%d", len(cfg.Data)),
	}, "/{repo}/{image}/blobs/uploads/{uuid}", cfg.Data)
	h.BlobUploadPostHandler(&wCfgPost, rCfgPost)
	assert.Equal(t, http.StatusCreated, wCfgPost.Code)
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

	wManiPut, rManiPut := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": testTag,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/manifests/{reference}", mani.Data)
	h.ManifestPutHandler(&wManiPut, rManiPut)
	assert.Equal(t, http.StatusCreated, wManiPut.Code)
	maniDigest := wManiPut.Header().Get("Docker-Content-Digest")
	assert.Equal(t, expManiDigest.String(), maniDigest)
	assert.Equal(t, "http://example.com/v2/oci/conformance/manifests/tagtest0", wManiPut.Header().Get("Location"))
}

func Test_Manifest_Remove(t *testing.T) {
	testRepo := "qnib"
	testImage := "featuretest"
	//testTag := "latest"
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	err := h.mag.Minio.EmptyAll()
	assert.NoError(t, err)
	err = h.mag.SQlite.EmptyData(true)
	assert.NoError(t, err)
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:arm64")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	maniDigest, err := br.Get("manifest:qnib/featuretest:arm64#digest")
	assert.NoError(t, err)
	// Remove
	_, rManiDel := createNewHTTPTest(http.MethodDelete, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": maniDigest,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	err = h.mag.Manifest_DeleteByHTTPRequest(rManiDel)
	assert.NoError(t, err)
	_, rManiDel2 := createNewHTTPTest(http.MethodDelete, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": maniDigest,
	}, map[string]string{
		"accountName": token,
		"userName":    "test",
		"profileName": "generic",
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}, "/{repo}/{image}/manifests/{reference}", nil)
	err = h.mag.Manifest_DeleteByHTTPRequest(rManiDel2)
	assert.Error(t, err)
}
