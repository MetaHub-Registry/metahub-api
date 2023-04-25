package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gCtx "github.com/gorilla/context"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	l1 = types.Layer{
		Digest:    "sha256:1234567890",
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      1234,
		Data:      []byte("1234567890"),
	}
)

func Test_BlobHandler_DirectPost(t *testing.T) {
	_, l0byte := types.FT_arm_BlobUpload_Layer0()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	//bs.AddBootstrap("layer:qnib/test")
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	testRepo := "qnib"
	testImage := "test"
	testProfile := "generic"
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	w, r := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": l1.Digest,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}", nil)
	/// POST w/o data to initiaite upload
	h.BlobUploadPostHandler(&w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("Code: %d, expected %d", w.Code, http.StatusAccepted)
	}
	assert.Equal(t, 0, w.Body.Len())
	// POST With data
	wDat, rDat := createNewHTTPTest(http.MethodPut, map[string]string{
		"repo":      testRepo,
		"image":     testImage,
		"reference": l1.Digest,
	}, map[string]string{
		"accountName": token,
		"userName":    testRepo,
		"profileName": testProfile,
	}, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
		"Content-Type":  "application/vnd.docker.distribution.manifest.v2+json",
	}, "/{repo}/{image}/blobs/{reference}", l0byte)
	h.BlobUploadPostHandler(&wDat, rDat)
	if wDat.Code != http.StatusCreated {
		t.Fatalf("Code: %d, expected %d", wDat.Code, http.StatusCreated)
	}
	assert.Equal(t, 0, wDat.Body.Len())
}

func Test_BlobHandler_LayerTest(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("layer:0:ft_arm64")
	//bs.PostTestSleep = 60
	br, err := SetupTest(ctx, bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/blobs/{reference}", nil)
	ftArm64L0Dig, err := br.Get("layer:0:ft_arm64#digest")
	assert.NoError(t, err)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", ftArm64L0Dig)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	_, found, err := h.mag.ConfigBlob_GetByHTTPRequest(ctx, r)
	assert.NoError(t, err)
	assert.False(t, found)
	/// HEAD
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, w.Body.Len())
	// GET (with data)
	r.Method = http.MethodGet
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, w.Body.Len(), 411)
	assert.LessOrEqual(t, w.Body.Len(), 412)
}

func Test_BlobHandler_Redirect_LayerTest(t *testing.T) {
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
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_arm64")
	//bs.PostTestSleep = 300
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	assert.NoError(t, err)
	_ = br
	defer TeardownTest(bs)
	/// Vars
	testRepo := "qnib"
	//testProfile := "generic"
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/blobs/{reference}", nil)
	rctx := chi.NewRouteContext()
	ftArm64L0Dig, err := br.Get("layer:0:ft_arm64#digest")
	assert.NoError(t, err)
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", ftArm64L0Dig)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	// GET (with redirect)
	r.Method = http.MethodGet
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
}

func Test_Upload_UploadBlobSequence(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	//bs.PostTestSleep = 300
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	assert.NoError(t, err)
	_ = br
	defer TeardownTest(bs)
	/// Vars
	testRepo := "qnib"
	testProfile := "generic"
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	// POST with Data
	w := httptest.NewRecorder()
	l0bytes := types.FT_arm_Layer0Byte()
	l0ref := types.FT_arm_Layer0Reference()
	r := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/", bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", "alpine")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPostHandler(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
	// Fetch Upload from DB
	gotLoc := w.Header().Get("Location")
	uuid := w.Header().Get("Docker-Upload-UUID")
	assert.Regexp(t, fmt.Sprintf(`/v2/qnib/alpine/blobs/uploads/%s$`, uuid), gotLoc)
	// Next we do a PATCH request
	wPatch := httptest.NewRecorder()
	rPatch := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/{uuid}", bytes.NewReader(l0bytes))
	rctxPatch := chi.NewRouteContext()
	rctxPatch.URLParams.Add("repo", testRepo)
	rctxPatch.URLParams.Add("image", "alpine")
	rctxPatch.URLParams.Add("uuid", uuid)
	rPatch = rPatch.WithContext(context.WithValue(rPatch.Context(), chi.RouteCtxKey, rctxPatch))
	gCtx.Set(rPatch, "accountName", token)
	gCtx.Set(rPatch, "userName", testRepo)
	gCtx.Set(rPatch, "profileName", testProfile)
	rPatch.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	rPatch.Header.Set("Content-Type", "application/octet-stream")
	rPatch.Header.Set("Content-Range", fmt.Sprintf("0-%d", len(l0bytes)-1))
	rPatch.Header.Set("Content-Length", fmt.Sprintf("%d", len(l0bytes)))
	h.BlobUploadPatchHandler(wPatch, rPatch)
	assert.Equal(t, http.StatusAccepted, wPatch.Code)
	// Final PUT request
	wPut := httptest.NewRecorder()
	putUrl := fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest)
	rPut := httptest.NewRequest(http.MethodPost, putUrl, bytes.NewReader(l0bytes))
	rctxPut := chi.NewRouteContext()
	rctxPut.URLParams.Add("repo", testRepo)
	rctxPut.URLParams.Add("image", "alpine")
	rctxPut.URLParams.Add("uuid", uuid)
	rPut = rPut.WithContext(context.WithValue(rPut.Context(), chi.RouteCtxKey, rctxPut))
	gCtx.Set(rPut, "accountName", token)
	gCtx.Set(rPut, "userName", testRepo)
	gCtx.Set(rPut, "profileName", testProfile)
	rPut.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPutHandler(wPut, rPut)
	assert.Equal(t, http.StatusCreated, wPut.Code)
	gotPutLoc := wPut.Header().Get("Location")
	assert.Regexp(t, fmt.Sprintf(`/v2/qnib/alpine/blobs/uploads/%s$`, uuid), gotPutLoc)
}

func Test_Upload_UploadBlobSequence_Split(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	//bs.SetSqlite("Desktop/metahub.db")
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	assert.NoError(t, err)
	_ = br
	defer TeardownTest(bs)
	/// Vars
	testRepo := "qnib"
	testProfile := "generic"
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	// POST with Data
	w := httptest.NewRecorder()
	l0bytes := types.FT_arm_Layer0Byte()
	l0bytePart1 := l0bytes[:len(l0bytes)/2]
	l0bytePart2 := l0bytes[len(l0bytes)/2:]
	l0ref := types.FT_arm_Layer0Reference()
	_ = l0ref
	r := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/", bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", "alpine")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPostHandler(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
	// Fetch Upload from DB
	gotLoc := w.Header().Get("Location")
	uuid := w.Header().Get("Docker-Upload-UUID")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/alpine/blobs/uploads/%s", uuid), gotLoc)
	// Next we do a PATCH request with Part1
	wPatchPart1 := httptest.NewRecorder()
	rPatchPart1 := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/{uuid}", bytes.NewReader(l0bytePart1))
	rctxPatch := chi.NewRouteContext()
	rctxPatch.URLParams.Add("repo", testRepo)
	rctxPatch.URLParams.Add("image", "alpine")
	rctxPatch.URLParams.Add("uuid", uuid)
	rPatchPart1 = rPatchPart1.WithContext(context.WithValue(rPatchPart1.Context(), chi.RouteCtxKey, rctxPatch))
	gCtx.Set(rPatchPart1, "accountName", token)
	gCtx.Set(rPatchPart1, "userName", testRepo)
	gCtx.Set(rPatchPart1, "profileName", testProfile)
	rPatchPart1.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	rPatchPart1.Header.Set("Content-Type", "application/octet-stream")
	rPatchPart1.Header.Set("Content-Range", fmt.Sprintf("0-%d", len(l0bytePart1)-1))
	rPatchPart1.Header.Set("Content-Length", fmt.Sprintf("%d", len(l0bytePart1)))
	h.BlobUploadPatchHandler(wPatchPart1, rPatchPart1)
	assert.Equal(t, http.StatusAccepted, wPatchPart1.Code)
	// Next we do a PATCH request with Part2
	wPatchPart2 := httptest.NewRecorder()
	rPatchPart2 := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/{uuid}", bytes.NewReader(l0bytePart2))
	rPatchPart2 = rPatchPart2.WithContext(context.WithValue(rPatchPart2.Context(), chi.RouteCtxKey, rctxPatch))
	gCtx.Set(rPatchPart2, "accountName", token)
	gCtx.Set(rPatchPart2, "userName", testRepo)
	gCtx.Set(rPatchPart2, "profileName", testProfile)
	rPatchPart2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	rPatchPart2.Header.Set("Content-Type", "application/octet-stream")
	rPatchPart2.Header.Set("Content-Range", fmt.Sprintf("%d-%d", len(l0bytePart1), len(l0bytePart1)+len(l0bytePart1)))
	rPatchPart2.Header.Set("Content-Length", fmt.Sprintf("%d", len(l0bytePart2)))
	h.BlobUploadPatchHandler(wPatchPart2, rPatchPart2)
	if wPatchPart2.Code != http.StatusAccepted {
		t.Fatalf("Code: %d, expected %d", wPatchPart2.Code, http.StatusAccepted)
	}
	// Final PUT request
	wPut := httptest.NewRecorder()
	putUrl := fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest)
	rPut := httptest.NewRequest(http.MethodPost, putUrl, bytes.NewReader(l0bytes))
	rctxPut := chi.NewRouteContext()
	rctxPut.URLParams.Add("repo", testRepo)
	rctxPut.URLParams.Add("image", "alpine")
	rctxPut.URLParams.Add("uuid", uuid)
	rPut = rPut.WithContext(context.WithValue(rPut.Context(), chi.RouteCtxKey, rctxPut))
	gCtx.Set(rPut, "accountName", token)
	gCtx.Set(rPut, "userName", testRepo)
	gCtx.Set(rPut, "profileName", testProfile)
	rPut.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPutHandler(wPut, rPut)
	assert.Equal(t, http.StatusCreated, wPut.Code)
	gotPutLoc := wPut.Header().Get("Location")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/alpine/blobs/uploads/%s", uuid), gotPutLoc)
}

func createNewHTTPTest(meth string, p, c, h map[string]string, url string, b []byte) (w httptest.ResponseRecorder, r *http.Request) {
	w = *httptest.NewRecorder()
	r = httptest.NewRequest(meth, url, bytes.NewReader(b))
	rctx := chi.NewRouteContext()
	for k, v := range p {
		rctx.URLParams.Add(k, v)
	}
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	for k, v := range c {
		gCtx.Set(r, k, v)
	}
	for k, v := range h {
		r.Header.Set(k, v)
	}
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(b)))
	return
}
