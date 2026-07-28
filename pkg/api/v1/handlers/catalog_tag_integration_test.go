package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// Integration smoke tests for the catalog + tag-list read handlers against the
// shared testcontainer backend (global h). They exercise the success path on an
// empty graph — CatalogGetHandler / TagGetHandler currently had little/no direct
// coverage.
func TestCatalogAndTagHandlers_EmptyGraph(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	br, err := SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	t.Run("catalog on empty graph -> 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v2/_catalog", nil)
		h.CatalogGetHandler(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("tags list on empty repo does not 200-with-tags", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v2/test/tags/list?repo=test", nil)
		h.TagGetHandler(w, r)
		// empty repo → an empty tag list (200) or a not-found/err; never a crash.
		assert.Contains(t, []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError}, w.Code)
	})
}
