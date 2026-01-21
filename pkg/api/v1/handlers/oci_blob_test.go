package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	gCtx "github.com/gorilla/context"
	"github.com/sirupsen/logrus"

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

func TestBlobHandlerDirectPost(t *testing.T) {
	debug := 0
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
	if err != nil {
		t.Fatal(err)
	}
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs, br)
	testRepo := "qnib"
	testImage := "test"
	testProfile := "generic"
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	w, r := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
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
	wDat, rDat := utils.CreateNewHTTPTest(ctx, http.MethodPost, map[string]string{
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

func TestBlobHandlerLayerTest(t *testing.T) {
	ctx := context.Background()
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobHandlerLayerTest",
	})
	defer span.End()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	if debug > 0 {
		bs.PostTestSleep = debug
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

	// Prepare
	if err := h.mag.ComputeAllManifests(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.mag.ComputeManifestlists(ctx); err != nil {
		t.Fatal(err)
	}
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	l0Arm64Dig, err := br.Get("layer:0:ft_amd64#digest")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Looking for layer digest:", utils.ShortenDigest(l0Arm64Dig))
	/// HEAD
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/blobs/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", l0Arm64Dig)
	rctx.URLParams.Add("digest", l0Arm64Dig)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Method = http.MethodHead
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, w.Body.Len())
	// // GET (with data)
	wGet := httptest.NewRecorder()
	rGet := httptest.NewRequest(http.MethodGet, "/{repo}/{image}/blobs/{reference}", nil)
	rctxGet := chi.NewRouteContext()
	rctxGet.URLParams.Add("repo", "qnib")
	rctxGet.URLParams.Add("image", "featuretest")
	rctxGet.URLParams.Add("reference", l0Arm64Dig)
	rctxGet.URLParams.Add("digest", l0Arm64Dig)
	rGet = rGet.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctxGet))
	rGet.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobHandler(wGet, rGet)
	assert.Equal(t, http.StatusTemporaryRedirect, wGet.Code)
	assert.GreaterOrEqual(t, wGet.Body.Len(), 100)
	assert.LessOrEqual(t, wGet.Body.Len(), 412)
	span.End()
}

func TestBlobHandlerRedirectLayerTest(t *testing.T) {
	ctx := context.Background()
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobHandlerLayerTest",
	})
	defer span.End()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	if debug > 0 {
		bs.PostTestSleep = debug
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
	testRepo := "qnib"
	//testProfile := "generic"
	if err := h.mag.ComputeAllManifests(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.mag.ComputeManifestlists(ctx); err != nil {
		t.Fatal(err)
	}
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/blobs/{reference}", nil)
	rctx := chi.NewRouteContext()
	ftArm64L0Dig, err := br.Get("layer:0:ft_arm64#digest")
	if err != nil {
		t.Fatal(err)
	}
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

func TestUploadUploadLayer0Sequence(t *testing.T) {
	ctx := context.Background()
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
	testRepo := "qnib"
	br.Add("repo", testRepo)
	br.Add("user", "admin")
	testProfile := "generic"
	br.Add("profile", testProfile)
	testImage := "alpine"
	br.Add("image", testImage)

	// POST to get UUID
	if w, r, e := getHttpTestLayer0UploadPostHandler(&br); e != nil {
		t.Fatal(e)
	} else {
		h.BlobUploadPostHandler(w, r)
		assert.Equal(t, http.StatusAccepted, w.Code)
		// Fetch Upload from DB
		gotLoc := w.Header().Get("Location")
		uuid := w.Header().Get("Docker-Upload-UUID")
		br.Add("uuid", uuid)
		assert.Regexp(t, fmt.Sprintf(`/v2/qnib/alpine/blobs/uploads/%s$`, uuid), gotLoc)
	}
	// Next we do a PATCH request
	if w, r, e := getHttpTestLayer0UploadPatchHandler(&br); e != nil {
		t.Fatal(e)
	} else {
		h.BlobUploadPatchHandler(w, r)
		assert.Equal(t, http.StatusAccepted, w.Code)
	}
	// Final PUT request
	if w, r, e := getHttpTestLayer0UploadPutHandler(&br); e != nil {
		t.Fatal(e)
	} else {
		h.BlobUploadPutHandler(w, r)
		assert.Equal(t, http.StatusCreated, w.Code)
		gotPutLoc := w.Header().Get("Location")
		uuid, err := br.Get("uuid")
		if err != nil {
			t.Fatal(err)
		}
		assert.Regexp(t, fmt.Sprintf(`/v2/%s/%s/blobs/uploads/%s$`, testRepo, testImage, uuid), gotPutLoc)
	}
	// CrossMount
	cmImage := "alpine-crossmount"
	br.Add("cross-mount-image", cmImage)
	if w, r, e := getHttpTestLayer0UploadPostHandlerMount(&br); e != nil {
		t.Fatal(e)
	} else {
		r.Header.Set("userRole", "admin")
		h.BlobUploadPostHandler(w, r)
		assert.Equal(t, http.StatusAccepted, w.Code)
		// Fetch Upload from DB
		gotLoc := w.Header().Get("Location")
		uuid := w.Header().Get("Docker-Upload-UUID")
		br.Add("uuid-mount", uuid)
		assert.Regexp(t, fmt.Sprintf(`/v2/%s/blobs/uploads/%s$`, cmImage, uuid), gotLoc)
	}
	// Remove
	// l0ref := types.FT_arm_Layer0Reference()
	// br.Add("reference", string(l0ref.Digest))
	// if w, r, e := getHttpTestLayer0UploadDeleteHandler(&br); e != nil {
	// 	t.Fatal(e)
	// } else {
	// 	h.BlobDelHandler(w, r)
	// 	assert.Equal(t, http.StatusAccepted, w.Code)
	// }
}

func TestUploadUploadBlobSequenceSplit(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	//bs.SetSqlite("Desktop/metahub.db")
	ctx := context.Background()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	_ = br
	defer TeardownTest(bs, br)
	/// Vars
	testRepo := "qnib"
	br.Add("repo", testRepo)
	testProfile := "generic"
	br.Add("profile", testProfile)
	testImage := "alpine"
	br.Add("image", testImage)
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	// POST with Data
	w := httptest.NewRecorder()
	l0bytes := types.FT_arm_Layer0Byte()
	l0bytePart1 := l0bytes[:len(l0bytes)/2]
	l0bytePart2 := l0bytes[len(l0bytes)/2:]
	l0ref := types.FT_arm_Layer0Reference()
	_ = l0ref
	r := httptest.NewRequest(http.MethodPost, "/{image}/blobs/uploads/", bytes.NewReader(l0bytes))
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
	rPatchPart1 := httptest.NewRequest(http.MethodPatch, "/{repo}/{image}/blobs/uploads/{uuid}", bytes.NewReader(l0bytePart1))
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
	rPut := httptest.NewRequest(http.MethodPut, putUrl, bytes.NewReader(l0bytes))
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

// BlobUploadGetHandler
func TestBlobUploadGetHandler(t *testing.T) {
	ctx := context.Background()
	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "TestBlobUploadGetHandler.Setup",
	})
	defer span.End()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:arm64")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	if debug > 0 {
		bs.PostTestSleep = debug
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
	span.End()
	defer TeardownTest(bs, br)
	testRepo := "qnib"
	br.Add("repo", testRepo)
	br.Add("user", "admin")
	testProfile := "generic"
	br.Add("profile", testProfile)
	testImage := "alpine"
	br.Add("image", testImage)
	br.Add("uuid", uuid.New().String())
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	if debug > 0 {
		logrus.SetLevel(logrus.DebugLevel)
	}
	_ = token
	t.Run("404NotFound", func(t *testing.T) {
		ctx = context.Background()
		_, ctx, span = utils.InitLogging(ctx, tracer, map[string]any{
			"func": "TestBlobUploadGetHandler.404NotFound",
		})
		defer span.End()
		// POST to get UUID
		if w, r, e := getHttpTestLayer0UploadPostHandler(&br); e != nil {
			t.Fatal(e)
		} else {
			h.BlobUploadGetHandler(w, r)
			assert.Equal(t, http.StatusNotFound, w.Code)
		}
		span.End()
	})
	t.Run("200ok", func(t *testing.T) {
		ctx = context.Background()
		_, ctx, span = utils.InitLogging(ctx, tracer, map[string]any{
			"func": "TestBlobUploadGetHandler.200ok",
		})
		defer span.End()
		uid, err := br.Get("uuid")
		if err != nil {
			t.Fatal(err)
		}
		q := fmt.Sprintf(`CREATE (bu:BlobUpload {location:"/{repo}/{image}/blobs/uploads/", upload_user:"admin", digest:"dig123",uuid:"%s", created_at:datetime(), updated_at:datetime()})`, uid)
		if _, err := h.mag.Database.Execute(q); err != nil {
			t.Fatal(err)
		}
		// POST to get UUID
		if w, r, e := getHttpTestLayer0UploadPostHandler(&br); e != nil {
			t.Fatal(e)
		} else {
			h.BlobUploadGetHandler(w, r)
			assert.Equal(t, http.StatusNoContent, w.Code)
		}
	})
}
