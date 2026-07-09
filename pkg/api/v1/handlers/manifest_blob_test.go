package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestPickBlobLayerDigest(t *testing.T) {
	// The model-config layer is preferred over any other layer, regardless of order.
	layers := []mlibTypes.Layer{
		{MediaType: "application/vnd.oci.image.layer.v1.tar", Digest: "sha256:first"},
		{MediaType: modelConfigMediaType, Digest: "sha256:config"},
	}
	assert.Equal(t, "sha256:config", pickBlobLayerDigest(layers))

	// No model-config layer → the first layer is used.
	layers = []mlibTypes.Layer{
		{MediaType: "application/vnd.oci.image.layer.v1.tar", Digest: "sha256:first"},
		{MediaType: "application/octet-stream", Digest: "sha256:second"},
	}
	assert.Equal(t, "sha256:first", pickBlobLayerDigest(layers))

	// No layers → "".
	assert.Equal(t, "", pickBlobLayerDigest(nil))
	assert.Equal(t, "", pickBlobLayerDigest([]mlibTypes.Layer{}))
}

func TestManifestBlobContentGet(t *testing.T) {
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

	t.Run("400 when repo/ref missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/manifest/blob-content", nil)
		h.ManifestBlobContentGet(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("400 when only repo given", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/manifest/blob-content?repo=models/x", nil)
		h.ManifestBlobContentGet(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not served for a nonexistent manifest", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/manifest/blob-content?repo=models/nonexistent&ref=nope", nil)
		h.ManifestBlobContentGet(w, r)
		// found=false → 404; a backend error → 500. Either exercises the
		// not-served path (never 200 for a bogus manifest).
		assert.Contains(t, []int{http.StatusNotFound, http.StatusInternalServerError}, w.Code)
	})
}
