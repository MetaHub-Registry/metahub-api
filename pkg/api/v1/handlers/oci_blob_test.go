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
	ctx = context.Background()
	l1  = types.Layer{
		Digest:    "sha256:1234567890",
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      1234,
		Data:      []byte("1234567890"),
	}
)

func Test_BlobHandler_LayerTest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("layer:qnib/test")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/blobs/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("image", "test")
	rctx.URLParams.Add("reference", l1.Digest)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	/// HEAD
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	if assert.Equal(t, 0, w.Body.Len()) {
		t.Log(w.Body.String())
	}
	// GET (with data)
	r.Method = http.MethodGet
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 10, w.Body.Len())

}

func Test_BlobHandler_Redirect_LayerTest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("layer-redirect:qnib/test")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	_ = br
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/blobs/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("image", "test")
	rctx.URLParams.Add("reference", l1.Digest)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	// GET (with redirect)
	r.Method = http.MethodGet
	h.BlobHandler(w, r)
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
}

func Test_Upload_UploadBlobSequence(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	_ = br
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	// POST with Data
	w := httptest.NewRecorder()
	l0bytes := types.FT_arm_Layer0Byte()
	l0ref := types.FT_arm_Layer0Reference()
	r := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/", bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("image", "alpine")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", "test")
	gCtx.Set(r, "profileName", "generic")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPostHandler(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
	// Fetch Upload from DB
	gotLoc := w.Header().Get("Location")
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
	assert.Equal(t, fmt.Sprintf("/v2/qnib/alpine/blobs/uploads/%s", uuid), gotLoc)
	// Next we do a PATCH request
	wPatch := httptest.NewRecorder()
	rPatch := httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/{uuid}", bytes.NewReader(l0bytes))
	rctxPatch := chi.NewRouteContext()
	rctxPatch.URLParams.Add("repo", "qnib")
	rctxPatch.URLParams.Add("image", "alpine")
	rctxPatch.URLParams.Add("uuid", uuid)
	rPatch = rPatch.WithContext(context.WithValue(rPatch.Context(), chi.RouteCtxKey, rctxPatch))
	gCtx.Set(rPatch, "accountName", token)
	gCtx.Set(rPatch, "userName", "test")
	gCtx.Set(rPatch, "profileName", "generic")
	rPatch.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	rPatch.Header.Set("Content-Type", "application/octet-stream")
	rPatch.Header.Set("Content-Range", "1-1")
	rPatch.Header.Set("Content-Length", fmt.Sprintf("%d", len(l0bytes)))
	h.BlobUploadPatchHandler(wPatch, rPatch)
	assert.Equal(t, http.StatusAccepted, wPatch.Code)
	// Final PUT request
	wPut := httptest.NewRecorder()
	putUrl := fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest)
	rPut := httptest.NewRequest(http.MethodPost, putUrl, bytes.NewReader(l0bytes))
	rctxPut := chi.NewRouteContext()
	rctxPut.URLParams.Add("repo", "qnib")
	rctxPut.URLParams.Add("image", "alpine")
	rctxPut.URLParams.Add("uuid", uuid)
	rPut = rPut.WithContext(context.WithValue(rPut.Context(), chi.RouteCtxKey, rctxPut))
	gCtx.Set(rPut, "accountName", token)
	gCtx.Set(rPut, "userName", "test")
	gCtx.Set(rPut, "profileName", "generic")
	rPut.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.BlobUploadPutHandler(wPut, rPut)
	assert.Equal(t, http.StatusCreated, wPut.Code)
	gotPutLoc := wPut.Header().Get("Location")
	assert.Equal(t, fmt.Sprintf("/v2/qnib/alpine/blobs/uploads/%s", uuid), gotPutLoc)
}

func createNewHTTPTest(p, c, h map[string]string, url string, b []byte) (w httptest.ResponseRecorder, r *http.Request) {
	w = *httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, url, bytes.NewReader(b))
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
	return
}
