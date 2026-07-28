package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// fakeReposDB implements the Database read methods RepoGetAll / RepoGetOne use.
type fakeReposDB struct {
	magazine.Database
	cols       []types.CollectionListing
	colsErr    error
	byName     types.CollectionListing
	byNameErr  error
	byID       types.CollectionListing
	byIDErr    error
	labels     map[string][]string
	labelsErr  error
	layers     []types.Layer
	layersErr  error
}

func (f *fakeReposDB) CollectionListingGetByRepoLikeForProfile(context.Context, string, string) ([]types.CollectionListing, error) {
	return f.cols, f.colsErr
}
func (f *fakeReposDB) CollectionListingGetByName(context.Context, string, string) (types.CollectionListing, error) {
	return f.byName, f.byNameErr
}
func (f *fakeReposDB) CollectionListingGetByID(context.Context, int64, int64) (types.CollectionListing, error) {
	return f.byID, f.byIDErr
}
func (f *fakeReposDB) ManifestGetProfileLabels(context.Context, []string) (map[string][]string, error) {
	return f.labels, f.labelsErr
}
func (f *fakeReposDB) GetLayersForManifestID(context.Context, int64) ([]types.Layer, error) {
	return f.layers, f.layersErr
}

func reposReq(body string) *http.Request {
	return httptest.NewRequest("POST", "/repos", strings.NewReader(body))
}

func TestRepoGetAll(t *testing.T) {
	// empty listing -> 200
	h := Handlers{mag: magazine.Magazine{Database: &fakeReposDB{}}}
	w := httptest.NewRecorder()
	h.RepoGetAll(w, reposReq(`{"repo":"acme"}`))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "bread_crumbs")

	// with collections + profile -> 200 (exercises profileFilteredManifestCount)
	h = Handlers{mag: magazine.Magazine{Database: &fakeReposDB{
		cols: []types.CollectionListing{
			{ID: 1, Repo: "acme/web", Tag: "v1", Manifests: map[string]types.Manifest{}},
			{ID: 2, Repo: "acme/api", Tag: "v2", Manifests: map[string]types.Manifest{}},
		},
	}}}
	w = httptest.NewRecorder()
	h.RepoGetAll(w, reposReq(`{"repo":"acme","profile":"cpu#zen2"}`))
	assert.Equal(t, http.StatusOK, w.Code)

	// DB error -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeReposDB{colsErr: context.DeadlineExceeded}}}
	w = httptest.NewRecorder()
	h.RepoGetAll(w, reposReq(`{"repo":"acme"}`))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// malformed body -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeReposDB{}}}
	w = httptest.NewRecorder()
	h.RepoGetAll(w, reposReq(`{not json`))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRepoGetOne(t *testing.T) {
	// by name -> 200
	h := Handlers{mag: magazine.Magazine{Database: &fakeReposDB{
		byName: types.CollectionListing{ID: 1, Repo: "acme/web", Tag: "v1", Manifests: map[string]types.Manifest{}},
	}}}
	w := httptest.NewRecorder()
	h.RepoGetOne(w, reposReq(`{"repo_name":"acme/web","tag_name":"v1"}`))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "collection")

	// by id -> 200
	h = Handlers{mag: magazine.Magazine{Database: &fakeReposDB{
		byID: types.CollectionListing{ID: 5, Repo: "acme/api", Tag: "v2", Manifests: map[string]types.Manifest{}},
	}}}
	w = httptest.NewRecorder()
	h.RepoGetOne(w, reposReq(`{"collection_id":5,"tag_id":9}`))
	assert.Equal(t, http.StatusOK, w.Code)

	// resolve error -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeReposDB{byNameErr: context.DeadlineExceeded}}}
	w = httptest.NewRecorder()
	h.RepoGetOne(w, reposReq(`{"repo_name":"acme/web","tag_name":"v1"}`))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// malformed body -> 500
	h = Handlers{mag: magazine.Magazine{Database: &fakeReposDB{}}}
	w = httptest.NewRecorder()
	h.RepoGetOne(w, reposReq(`{bad`))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
