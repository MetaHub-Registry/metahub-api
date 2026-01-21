package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	gCtx "github.com/gorilla/context"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func getHttpTestLayer0UploadPutHandler(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testUser string
	if testRepo, err = br.Get("repo"); err != nil {
		return w, r, err
	}
	if testProfile, err = br.Get("profile"); err != nil {
		return w, r, err
	}
	if testImage, err = br.Get("image"); err != nil {
		return w, r, err
	}
	if testUser, err = br.Get("user"); err != nil {
		return w, r, err
	}
	token, err := br.Get(fmt.Sprintf("user:%s#bearer", testUser))
	if err != nil {
		return w, r, err
	}
	uuid, err := br.Get("uuid")
	if err != nil {
		return w, r, err
	}
	l0ref := types.FT_arm_Layer0Reference()
	l0bytes := types.FT_arm_Layer0Byte()
	w = httptest.NewRecorder()
	putUrl := fmt.Sprintf("/{repo}/{image}/blobs/uploads/{uuid}?digest=%s", l0ref.Digest)
	r = httptest.NewRequest(http.MethodPut, putUrl, bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("uuid", uuid)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return w, r, nil
}

func getHttpTestLayer0UploadPatchHandler(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage string
	if testRepo, err = br.Get("repo"); err != nil {
		return w, r, err
	}
	if testProfile, err = br.Get("profile"); err != nil {
		return w, r, err
	}
	if testImage, err = br.Get("image"); err != nil {
		return w, r, err
	}
	token, err := br.Get("user:test#bearer")
	if err != nil {
		return w, r, err
	}
	uuid, err := br.Get("uuid")
	if err != nil {
		return w, r, err
	}
	l0bytes := types.FT_arm_Layer0Byte()
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPatch, "/{repo}/{image}/blobs/uploads/{uuid}", bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("uuid", uuid)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Content-Type", "application/octet-stream")
	r.Header.Set("Content-Range", fmt.Sprintf("0-%d", len(l0bytes)-1))
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(l0bytes)))
	return w, r, nil
}

// Blob Delete
func getHttpTestLayer0UploadGetHandler(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testRef string
	if testRepo, err = br.Get("repo"); err != nil {
		return w, r, err
	}
	if testProfile, err = br.Get("profile"); err != nil {
		return w, r, err
	}
	if testImage, err = br.Get("image"); err != nil {
		return w, r, err
	}
	if testRef, err = br.Get("reference"); err != nil {
		return w, r, err
	}
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		return w, r, err
	} //
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/{repo}/{image}/blobs/uploads/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("reference", testRef)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return w, r, nil
}

// Pull.Setup Populate registry with test blob
func getHttpTestLayer0UploadPostHandler(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testUser, testImage string
	if testRepo, err = br.Get("repo"); err != nil {
		return w, r, err
	}
	if testProfile, err = br.Get("profile"); err != nil {
		return w, r, err
	}
	if testImage, err = br.Get("image"); err != nil {
		return w, r, err
	}
	if testUser, err = br.Get("user"); err != nil {
		return w, r, err
	}
	/// Request
	token, err := br.Get(fmt.Sprintf("user:%s#bearer", testUser))
	if err != nil {
		return w, r, err
	} //
	w = httptest.NewRecorder()
	l0bytes := types.FT_arm_Layer0Byte()
	r = httptest.NewRequest(http.MethodPost, "/{repo}/{image}/blobs/uploads/", bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return w, r, nil
}

// Pull.Cross-mounting of a blob without the from argument should yield session id
func getHttpTestLayer0UploadPostHandlerMount(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testUser, testImage string
	if testProfile, err = br.Get("profile"); err != nil {
		return w, r, err
	}
	if testImage, err = br.Get("cross-mount-image"); err != nil {
		return w, r, err
	}
	if testUser, err = br.Get("user"); err != nil {
		return w, r, err
	}
	/// Request
	token, err := br.Get(fmt.Sprintf("user:%s#bearer", testUser))
	if err != nil {
		return w, r, err
	} //
	w = httptest.NewRecorder()
	l0ref := types.FT_arm_Layer0Reference()
	l0bytes := types.FT_arm_Layer0Byte()
	postUrl := fmt.Sprintf("/{image}/blobs/uploads/?mount=%s", l0ref.Digest)
	r = httptest.NewRequest(http.MethodPost, postUrl, bytes.NewReader(l0bytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("image", testImage)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "userRole", "admin")
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return w, r, nil
}
