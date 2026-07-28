package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// fakeModelsDB implements only the model/runtime read methods of
// magazine.Database used by models.go; the rest panic if called.
type fakeModelsDB struct {
	magazine.Database
	listing     []types.ModelListing
	listErr     error
	runtimes    []types.Runtime
	runtErr     error
	detail      types.ModelDetail
	detailFound bool
	detailErr   error
}

func (f *fakeModelsDB) ModelListingGetAll(context.Context) ([]types.ModelListing, error) {
	return f.listing, f.listErr
}
func (f *fakeModelsDB) RuntimesGetAll(context.Context) ([]types.Runtime, error) {
	return f.runtimes, f.runtErr
}
func (f *fakeModelsDB) ModelDetailGet(_ context.Context, _ string) (types.ModelDetail, bool, error) {
	return f.detail, f.detailFound, f.detailErr
}

func TestModelsGetAll(t *testing.T) {
	// success
	h := Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{}}}
	w := httptest.NewRecorder()
	h.ModelsGetAll(w, httptest.NewRequest("GET", "/models", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "models")

	// backend error -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{listErr: errors.New("boom")}}}
	w = httptest.NewRecorder()
	h.ModelsGetAll(w, httptest.NewRequest("GET", "/models", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRuntimesGetAll(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{}}}
	w := httptest.NewRecorder()
	h.RuntimesGetAll(w, httptest.NewRequest("GET", "/runtimes", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "runtimes")

	h = Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{runtErr: errors.New("boom")}}}
	w = httptest.NewRecorder()
	h.RuntimesGetAll(w, httptest.NewRequest("GET", "/runtimes", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestModelsGetOne(t *testing.T) {
	// missing ref -> 400 (no chi route param, no org)
	h := Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{}}}
	w := httptest.NewRecorder()
	h.ModelsGetOne(w, httptest.NewRequest("GET", "/models", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// not found -> 404
	h = Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{detailFound: false}}}
	w = httptest.NewRecorder()
	h.ModelsGetOne(w, adminReq("GET", "/models/x", "", map[string]string{"ref": "x"}))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// backend error -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{detailErr: errors.New("boom")}}}
	w = httptest.NewRecorder()
	h.ModelsGetOne(w, adminReq("GET", "/models/x", "", map[string]string{"ref": "x"}))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// success -> 200
	h = Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{detailFound: true, detail: types.ModelDetail{Name: "m"}}}}
	w = httptest.NewRecorder()
	h.ModelsGetOne(w, adminReq("GET", "/models/m", "", map[string]string{"ref": "m"}))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "model")
}

func TestModelsGetCompositions(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{detailFound: true, detail: types.ModelDetail{Name: "m"}}}}
	w := httptest.NewRecorder()
	h.ModelsGetCompositions(w, adminReq("GET", "/models/m/compositions", "", map[string]string{"ref": "m"}))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "compositions")

	// org-qualified name via {org}/{name} params (ref reassembled)
	h = Handlers{mag: magazine.Magazine{Database: &fakeModelsDB{detailFound: true}}}
	w = httptest.NewRecorder()
	h.ModelsGetCompositions(w, adminReq("GET", "/models/qnib/minicpm5/compositions", "", map[string]string{"org": "qnib", "name": "minicpm5"}))
	assert.Equal(t, http.StatusOK, w.Code)
}
