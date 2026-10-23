package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/distribution/distribution/manifest/schema2"
	gCtx "github.com/gorilla/context"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	//nolint:unused // fixture retained for the temporarily disabled manifest tests
	testManifestBytes = []byte(`{
		"schemaVersion": 2,
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "sha256:56da786a44839ae967f901c3b3119f534608053ac3ae8843e454e0ea0fb9aef5",
			"size": 129,
			"data": "ewoJImF1dGhvciI6ICJ6OHJPNlhscFo3LVRUTkhyIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
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
	}`)
	testManifestWithoutLayersBytes = []byte(`{
			"schemaVersion": 2,
			"config": {
				"mediaType": "application/vnd.oci.image.config.v1+json",
				"digest": "sha256:eb3844bc2b6019324d2542315ed7135d9609c1b25251c4174044c8ffa39f1f8d",
				"size": 129,
				"data": "ewoJImF1dGhvciI6ICJCRHQxd3RpUVZpZk5BMEpoIiwKCSJhcmNoaXRlY3R1cmUiOiAiYW1kNjQiLAoJIm9zIjogImxpbnV4IiwKCSJyb290ZnMiOiB7CgkJInR5cGUiOiAibGF5ZXJzIiwKCQkiZGlmZl9pZHMiOiBbXQoJfQp9",
				"newUnspecifiedField": "aGVsbG8gd29ybGQ="
			},
			"layers": []
		}`)
)

//nolint:unused // referenced by a temporarily disabled test
func getHttpTestManifestDeleteByDigest(ctx context.Context, br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, maniDigest, testImage string
	if testRepo, err = br.Get("repo"); err != nil {
		return w, r, err
	}
	if maniDigest, err = br.Get("digest"); err != nil {
		return w, r, err
	}
	if testImage, err = br.Get("image"); err != nil {
		return w, r, err
	}
	token, err := br.Get("user:test#bearer")
	if err != nil {
		return w, r, err
	}
	wVal, r := utils.CreateNewHTTPTest(ctx, http.MethodDelete, map[string]string{
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
	w = &wVal
	return w, r, err
}

func getHttpTestManifestUploadPutOnlyHandlerWithoutLayers(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testTag string
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
	if testTag, err = br.Get("tag"); err != nil {
		return w, r, err
	}
	w = httptest.NewRecorder()
	putUrl := "/{repo}/{image}/manifests/{reference}"
	r = httptest.NewRequest(http.MethodPut, putUrl, bytes.NewReader(testManifestWithoutLayersBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("reference", testTag)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Content-Type", schema2.MediaTypeManifest)
	return w, r, nil
}

// Populate registry with test manifest (with single put)
//nolint:unused // referenced by a temporarily disabled test
func getHttpTestManifestUploadPutOnlyHandler(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testTag, testMediaType string
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
	if testTag, err = br.Get("tag"); err != nil {
		return w, r, err
	}
	if testMediaType, err = br.Get("mediaType"); err != nil {
		return w, r, err
	}
	w = httptest.NewRecorder()
	putUrl := "/{repo}/{image}/manifests/{reference}"
	r = httptest.NewRequest(http.MethodPut, putUrl, bytes.NewReader(testManifestBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("reference", testTag)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Content-Type", testMediaType)
	return w, r, nil
}

//nolint:unused // referenced by a temporarily disabled test
func getHttpTestManifestByDigest(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testRef, testMethod string
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
	if testRef, err = br.Get("reference"); err != nil {
		return w, r, err
	}
	if testMethod, err = br.Get("method"); err != nil {
		return w, r, err
	}

	w = httptest.NewRecorder()
	url := "/{repo}/{image}/manifests/{reference}"
	r = httptest.NewRequest(testMethod, url, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("reference", testRef)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Content-Type", schema2.MediaTypeManifest)
	return w, r, nil
}

func getHttpTestManifestHeadByDigest(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testDigest string
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
	if testDigest, err = br.Get("digest"); err != nil {
		return w, r, err
	}
	w = httptest.NewRecorder()
	url := "/{repo}/{image}/manifests/{reference}"
	r = httptest.NewRequest(http.MethodHead, url, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("reference", testDigest)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Content-Type", schema2.MediaTypeManifest)
	return w, r, nil
}

func getHttpTestManifestHeadByTag(br *utils.BootstrapResult) (w *httptest.ResponseRecorder, r *http.Request, err error) {
	var testRepo, testProfile, testImage, testTag string
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
	if testTag, err = br.Get("tag"); err != nil {
		return w, r, err
	}
	w = httptest.NewRecorder()
	url := "/{repo}/{image}/manifests/{reference}"
	r = httptest.NewRequest(http.MethodHead, url, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("repo", testRepo)
	rctx.URLParams.Add("image", testImage)
	rctx.URLParams.Add("reference", testTag)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	gCtx.Set(r, "accountName", token)
	gCtx.Set(r, "userName", testRepo)
	gCtx.Set(r, "profileName", testProfile)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Content-Type", schema2.MediaTypeManifest)
	return w, r, nil
}
