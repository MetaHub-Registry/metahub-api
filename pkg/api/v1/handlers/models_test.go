package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestModelsGetAll_Empty verifies GET /api/v1/models wires through to
// ModelListingGetAll and returns a 200 envelope with an empty catalog when no
// model/<name> manipulations exist. The populated data path is covered by
// metalib's TestModelListingGetAll.
func TestModelsGetAll_Empty(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/models", nil)
	h.ModelsGetAll(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Models []mlibTypes.ModelListing `json:"models"`
		Error  string                   `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Error)
	assert.Empty(t, resp.Models)
}

// TestModelsGetOne_NotFound verifies GET /api/v1/models/{ref} wires through to
// ModelDetailGet and answers 404 when no model/<name> manipulation exists. The
// populated card payload is covered by metalib's TestModelDetailGet.
func TestModelsGetOne_NotFound(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ref", "does-not-exist")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/models/does-not-exist", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ModelsGetOne(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "model not found", resp["error"])
}

// TestModelsGetOne_OrgQualifiedEncodedRef verifies that an org-qualified model
// name (models/<org>/<name>) — which the UI sends as a single %2F-encoded path
// segment (qnib%2Fminicpm5) — is url-decoded before lookup, so it resolves the
// model/qnib/minicpm5 manipulation instead of 404ing on the literal `%`.
func TestModelsGetOne_OrgQualifiedEncodedRef(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	// Seed an org-qualified model manipulation, as `repackage --name qnib/minicpm5` wires it.
	_, err = h.mag.Database.Execute(`
		CREATE (mp:Manipulation {label: "model/qnib/minicpm5"})
		CREATE (mp)-[:HAS]->(:ManipuLayer {label: "model/qnib/minicpm5", rule: "", select: ""})-[:UPSTREAM]->(up:Upstream {reg: "reg", repo: "models/qnib/minicpm5-layer", tag: "q8-0", digest: "sha256:orgq8"})
		CREATE (m:Manifest {digest: "sha256:orgq8"})-[:ORIGINAL]->(up)
		CREATE (m)-[:HAS_LAYER]->(:Blob {mediaType: "application/vnd.oci.image.layer.v1.tar", size: 1153532416, type: "layer"})
	`)
	require.NoError(t, err)

	// The UI encodes '/' → %2F; chi hands the handler the still-escaped value.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ref", "qnib%2Fminicpm5")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/models/qnib%2Fminicpm5", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ModelsGetOne(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Model mlibTypes.ModelDetail `json:"model"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "qnib/minicpm5", resp.Model.Name)
}

// TestRuntimesGetAll_Empty verifies GET /api/v1/runtimes returns a 200 envelope
// with an empty registry when no runtimes are registered.
func TestRuntimesGetAll_Empty(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	token, err := br.Get("user:test#bearer")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	_, r := utils.CreateNewHTTPTest(ctx, http.MethodGet, map[string]string{}, map[string]string{},
		map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)},
		"/api/v1/runtimes", nil)
	h.RuntimesGetAll(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Runtimes []mlibTypes.Runtime `json:"runtimes"`
		Error    string              `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Error)
	assert.Empty(t, resp.Runtimes)
}

// TestModelsGetCompositions_NotFound verifies GET
// /api/v1/models/{ref}/compositions answers 404 for an unknown model.
func TestModelsGetCompositions_NotFound(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("ref", "does-not-exist")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/models/does-not-exist/compositions", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ModelsGetCompositions(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "model not found", resp["error"])
}

// TestModelsGetOne_MissingRef verifies the shared loadModelDetail helper answers
// 400 "model ref required" when the {ref} path param is empty, before any
// backend call. Covers the missing-ref branch for both ModelsGetOne and
// ModelsGetCompositions in one place.
func TestModelsGetOne_MissingRef(t *testing.T) {
	rctx := chi.NewRouteContext() // no "ref" param registered
	req := httptest.NewRequest(http.MethodGet, "/api/v1/models/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ModelsGetOne(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "model ref required", resp["error"])
}
