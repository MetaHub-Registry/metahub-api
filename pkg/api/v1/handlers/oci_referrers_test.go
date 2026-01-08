package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestReferrersGetHandlerNotFound(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs)
		t.Fatal(err)
	}

	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer TeardownTest(bs)
	/// Request
	token, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/{image}/referrers/{reference}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", "FAKEDIGEST1234567890abcdef")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.ReferrersGetHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodHead, "/{repo}/{image}/referrers/{reference}", nil)
	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", "FAKEDIGEST1234567890abcdef")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.ReferrersGetHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodHead, "/{repo}/{repo2}/{image}/referrers/{reference}", nil)
	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("repo2", "repo")
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", "FAKEDIGEST1234567890abcdef")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.ReferrersGetHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodHead, "/{repo}/{repo2}/{repo3}/{image}/referrers/{reference}", nil)
	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("repo", "qnib")
	rctx.URLParams.Add("repo2", "repo")
	rctx.URLParams.Add("repo3", "sub")
	rctx.URLParams.Add("image", "featuretest")
	rctx.URLParams.Add("reference", "FAKEDIGEST1234567890abcdef")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	h.ReferrersGetHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// func TestReferrersGetHandlerHead(t *testing.T) {
// 	ctx := context.Background()
// 	_, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
// 		"func": "TestManifestPutOnlyWithLayerDelete",
// 	})
// 	defer span.End()
// 	debug := 1
// 	bs := utils.NewBootstrap()
// 	bs.SetEmptyAll()
// 	bs.SetYAMLPrefix("../../../../misc/yaml")
// 	bs.AddBootstrap("users")
// 	bs.AddBootstrap("platforms:generic")
// 	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
// 	bs.AddBootstrap("config:ft_arm64")
// 	if debug > 0 {
// 		bs.PostTestSleep = debug
// 	}
// 	br, err := SetupTest(ctx, bs)
// 	if err != nil {
// 		TeardownTest(bs)
// 		t.Fatal(err)
// 	}

// 	for _, l := range br.ToLines() {
// 		if debug > 0 {
// 			t.Log(l)
// 		}
// 	}
// 	defer TeardownTest(bs)
// 	// // Prepare
// 	// if err := h.mag.ComputeAllManifests(ctx); err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	// if err := h.mag.ComputeManifestlists(ctx); err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	/// Request
// 	token, err := br.Get("user:test#bearer")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	w := httptest.NewRecorder()
// 	r := httptest.NewRequest(http.MethodHead, "/{repo}/{image}/referrers/{reference}", nil)
// 	maniDigest, err := br.Get("manifest:qnib/featuretest:arm64#digest")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	rctx := chi.NewRouteContext()
// 	rctx.URLParams.Add("repo", "qnib")
// 	rctx.URLParams.Add("image", "featuretest")
// 	rctx.URLParams.Add("reference", maniDigest)
// 	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
// 	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

// 	_, found, err := h.mag.ConfigBlobGetByHTTPRequest(ctx, r)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.False(t, found)
// 	_ = w
// 	// /// HEAD

// 	// h.ReferrersGetHandler(w, r)
// 	// assert.Equal(t, http.StatusNoContent, w.Code)
// 	// // GET (with data)
// 	// r.Method = http.MethodGet
// 	// h.ReferrersGetHandler(w, r)
// 	// assert.Equal(t, http.StatusOK, w.Code)
// 	// assert.GreaterOrEqual(t, w.Body.Len(), 100)
// 	// assert.LessOrEqual(t, w.Body.Len(), 412)
// 	span.End()
// }
