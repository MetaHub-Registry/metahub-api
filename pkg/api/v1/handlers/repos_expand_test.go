package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestCollectionGetOne_IncludesConfigAndLayers verifies that the
// `config` and `layers` fields are populated on every manifest returned
// by `POST /api/v1/collection`.
func TestCollectionGetOne_IncludesConfigAndLayers(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.SetYAMLPrefix("../../../../misc/yaml")
	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/generic/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_amd64")
	bs.AddBootstrap("collection:ft_arm64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("computeManifests")
	bs.AddBootstrap("computeManifestlists")

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer TeardownTest(bs, br)

	colID, err := br.GetInt64("collection:ft#id")
	require.NoError(t, err)
	colTagID, err := br.GetInt64("tag:generic#id")
	require.NoError(t, err)

	reqBody := ReqPayload{ColID: colID, TagID: colTagID}
	wGet := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collection", bytes.NewBuffer(reqBody.ToByte()))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetOne(wGet, req)
	require.Equal(t, http.StatusOK, wGet.Code)

	var resp CollectionResponse
	require.NoError(t, json.Unmarshal(wGet.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Collection.Manifests, "collection must have manifests")

	// At least one manifest should carry a populated config descriptor.
	var withConfig, withLayers int
	for _, m := range resp.Collection.Manifests {
		if m.Config != nil && m.Config.Digest != "" {
			withConfig++
			assert.NotEmpty(t, m.Config.MediaType, "config.media_type should be populated")
			assert.Positive(t, m.Config.Size, "config.size should be positive")
		}
		if len(m.Layers) > 0 {
			withLayers++
			assert.NotEmpty(t, m.Layers[0].Digest)
			assert.NotEmpty(t, m.Layers[0].MediaType)
		}
	}
	assert.Positive(t, withConfig, "at least one manifest should have a config descriptor")
	assert.Positive(t, withLayers, "at least one manifest should have layer descriptors")

	// JSON shape check — snake_case keys.
	body := wGet.Body.String()
	assert.Contains(t, body, `"config":{`)
	assert.Contains(t, body, `"layers":[`)
	assert.Contains(t, body, `"media_type":`)
}
