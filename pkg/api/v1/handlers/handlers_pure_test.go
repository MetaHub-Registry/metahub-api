package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestQuotaExceededError_Error(t *testing.T) {
	e := &quotaExceededError{Team: "t", Project: "p", Usage: 10, Delta: 5, Quota: 12}
	msg := e.Error()
	assert.Contains(t, msg, "t/p")
	assert.Contains(t, msg, "usage 10")
	assert.Contains(t, msg, "incoming 5")
	assert.Contains(t, msg, "quota 12")
}

func TestWriteQuota413(t *testing.T) {
	w := httptest.NewRecorder()
	writeQuota413(w, &quotaExceededError{Team: "t", Project: "p", Usage: 10, Delta: 5, Quota: 12})
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	errs, ok := body["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errs, 1)
	first := errs[0].(map[string]any)
	assert.Equal(t, "DENIED", first["code"])
}

func TestWriteJSON_ProdAndDev(t *testing.T) {
	// production: compact marshal
	hProd := Handlers{environment: "production"}
	w := httptest.NewRecorder()
	require.NoError(t, hProd.writeJSON(w, http.StatusCreated, envelope{"a": 1}))
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"a":1}`, w.Body.String())

	// development: indented marshal (still valid JSON)
	hDev := Handlers{environment: "development"}
	w = httptest.NewRecorder()
	require.NoError(t, hDev.writeJSON(w, http.StatusOK, envelope{"b": 2}))
	assert.JSONEq(t, `{"b":2}`, w.Body.String())

	// custom headers are copied through
	w = httptest.NewRecorder()
	hdr := http.Header{"X-Test": []string{"yes"}}
	require.NoError(t, hProd.writeJSON(w, http.StatusOK, envelope{"c": 3}, hdr))
	assert.Equal(t, "yes", w.Header().Get("X-Test"))
}

func TestWriteJSON_MarshalError(t *testing.T) {
	// a channel is not JSON-serialisable -> writeJSON returns an error (both envs)
	for _, env := range []string{"production", "development"} {
		h := Handlers{environment: env}
		w := httptest.NewRecorder()
		err := h.writeJSON(w, http.StatusOK, envelope{"bad": make(chan int)})
		assert.Error(t, err, env)
	}
}

func TestRedirectToWWW(t *testing.T) {
	// configured RedirectDoc -> 307 to it
	h := Handlers{mag: magazine.Magazine{Config: magazine.MagConfig{RedirectDoc: "https://docs.example.com"}}}
	w := httptest.NewRecorder()
	h.RedirectToWWW(w, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "https://docs.example.com", w.Header().Get("Location"))

	// empty RedirectDoc -> default
	h2 := Handlers{}
	w = httptest.NewRecorder()
	h2.RedirectToWWW(w, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, defURL, w.Header().Get("Location"))
}

func TestBuildCollectionBreadcrumbs(t *testing.T) {
	col := types.CollectionListing{Repo: "acme/web/app", Tag: "v1"}
	crumbs, reps := buildCollectionBreadcrumbs(col)
	require.NotNil(t, reps)
	require.Len(t, crumbs, 3)
	// final crumb is the active/current one (Active==0)
	assert.Equal(t, 0, crumbs[len(crumbs)-1].Active)
	assert.Equal(t, "acme", crumbs[0].Crumb)
}

func TestAccumulateCollectionNext(t *testing.T) {
	cols := []types.CollectionListing{
		{ID: 1, Repo: "acme/web", Tag: "v1", ManifestCount: 2},
		{ID: 2, Repo: "acme/api", Tag: "v2", ManifestCount: 3},
	}
	// browsing at "acme" (slug len 1) -> children are the next segments
	next, reps, crumbs, slugEnd := accumulateCollectionNext(cols, []string{"acme"}, "acme")
	assert.NotNil(t, next)
	assert.NotNil(t, reps)
	assert.NotNil(t, crumbs)
	assert.GreaterOrEqual(t, len(next), 1)
	_ = slugEnd
}
