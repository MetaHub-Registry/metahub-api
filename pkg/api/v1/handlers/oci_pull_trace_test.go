package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	gCtx "github.com/gorilla/context"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// These tests exercise the pull-path spans added for perf tracing
// (BlobHandler.openBlobReader/streamBlob, tryBlobGetRedirect.presign,
// ManifestHandler.resolveModelCentricRuntime). They drive the real handlers so
// the instrumented branches actually execute; the assertions double as
// behavioural coverage of stream-vs-redirect and the model-centric resolve.

// uploadBlobForPull POSTs then does a monolithic PUT?digest of a blob so it
// lands in MinIO with a :Blob node but NO upstream node — mirrors
// TestUploadUploadMonolithicStream. An upload-seeded blob (no upstream) lets a
// subsequent GET fall through the upstream-redirect gate so we can steer it into
// either the presign branch or the streaming branch. Returns (digest, bytes).
func uploadBlobForPull(t *testing.T, ctx context.Context, token, testRepo, testImage, testProfile string) (string, []byte) {
	t.Helper()
	l0bytes := types.FTarmLayer0Byte()
	l0ref := types.FTarmLayer0Reference()
	// POST to open the upload.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/{image}/blobs/uploads/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPostHandler(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
	uuid := w.Header().Get("Docker-Upload-UUID")
	// Monolithic PUT: whole body + ?digest, no PATCH → streaming store.
	wPut := httptest.NewRecorder()
	putURL := fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest)
	rPut := httptest.NewRequest(http.MethodPut, putURL, bytes.NewReader(l0bytes))
	rctxPut := chi.NewRouteContext()
	rctxPut.URLParams.Add("repo", testRepo)
	rctxPut.URLParams.Add("image", testImage)
	rctxPut.URLParams.Add("uuid", uuid)
	rPut = rPut.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctxPut))
	gCtx.Set(rPut, "accountName", token)
	gCtx.Set(rPut, "userName", testRepo)
	gCtx.Set(rPut, "profileName", testProfile)
	rPut.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	rPut.Header.Set("Content-Length", fmt.Sprint(len(l0bytes)))
	h.BlobUploadPutHandler(wPut, rPut)
	assert.Equal(t, http.StatusCreated, wPut.Code)
	return l0ref.Digest.String(), l0bytes
}

func newBlobGet(ctx context.Context, repo, image, reference, token string) (*httptest.ResponseRecorder, *http.Request) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/{repo}/{image}/blobs/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", repo)
	rctx.URLParams.Add("image", image)
	rctx.URLParams.Add("reference", reference)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return w, r
}

// TestBlobHandlerStreamsWhenRedirectDisabled drives BlobHandler down the
// constant-memory streaming branch (open reader + io.Copy) by turning off the
// blob-bucket redirect. Covers BlobHandler.openBlobReader + BlobHandler.streamBlob.
func TestBlobHandlerStreamsWhenRedirectDisabled(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
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
	dig, want := uploadBlobForPull(t, ctx, token, "qnib", "alpine", "generic")

	// Flip the shared harness redirect OFF (read-modify-write; restore on exit).
	// Must not t.Parallel() — h is a package global.
	key := minio.GetBlobBucketKey()
	cfg := h.mag.Minio.BucketConfig[key]
	orig := cfg.Redirect
	cfg.Redirect = false
	h.mag.Minio.BucketConfig[key] = cfg
	defer func() {
		cfg.Redirect = orig
		h.mag.Minio.BucketConfig[key] = cfg
	}()

	w, r := newBlobGet(ctx, "qnib", "alpine", dig, token)
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, fmt.Sprint(len(want)), w.Header().Get("Content-Length"))
	assert.Equal(t, want, w.Body.Bytes())
}

// TestBlobHandlerPresignRedirect drives BlobHandler into the MinIO presign
// branch (redirect ON by default in the harness; an upload-seeded blob has no
// upstream node so the upstream-redirect gate falls through). Covers
// tryBlobGetRedirect.presign.
func TestBlobHandlerPresignRedirect(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
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
	dig, _ := uploadBlobForPull(t, ctx, token, "qnib", "alpine", "generic")

	// Ensure redirect is ON (harness default) — set explicitly to be robust
	// against test-ordering side effects, and restore.
	key := minio.GetBlobBucketKey()
	cfg := h.mag.Minio.BucketConfig[key]
	orig := cfg.Redirect
	cfg.Redirect = true
	h.mag.Minio.BucketConfig[key] = cfg
	defer func() {
		cfg.Redirect = orig
		h.mag.Minio.BucketConfig[key] = cfg
	}()

	w, r := newBlobGet(ctx, "qnib", "alpine", dig, token)
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "X-Amz-Signature")
}

// TestResolveModelCentricRuntimeNotFound executes resolveModelCentricRuntime for
// a models/<org>/<name> repo whose model has no composition (ModelDetailGet
// returns not-found), so the whole function body runs and returns ok=false.
// Covers ManifestHandler.resolveModelCentricRuntime. Called directly (it is an
// unexported method in this package) to keep the assertion deterministic and
// avoid the downstream pullthrough network path.
func TestResolveModelCentricRuntimeNotFound(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	req := types.Request{Repo: "models/qnib/minicpm5", Tag: "q8-0"}
	runtimeRepo, modelName, quant, ok := h.resolveModelCentricRuntime(ctx, req)
	assert.False(t, ok)
	assert.Empty(t, runtimeRepo)
	assert.Empty(t, modelName)
	assert.Empty(t, quant)

	// A non-model repo returns early (guard) without touching the DB.
	_, _, _, ok2 := h.resolveModelCentricRuntime(ctx, types.Request{Repo: "qnib/alpine", Tag: "latest"})
	assert.False(t, ok2)
}
