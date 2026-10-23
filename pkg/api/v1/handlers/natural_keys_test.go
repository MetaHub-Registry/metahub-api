package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertNoStorageID(t *testing.T, where string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", where, err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("%s: unmarshal: %v", where, err)
	}
	var walk func(any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, child := range n {
				if k == "id" || (len(k) > 3 && k[len(k)-3:] == "_id") {
					t.Errorf("%s: storage ID field %q in public payload", where, k)
				}
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(decoded)
}

// TestPublicDTOsCarryNoStorageIDs is the response-side guard for the
// natural-key contract. It runs in the integration lane (the handlers
// TestMain needs containers) and covers every DTO that is written to a
// public JSON response.
func TestPublicDTOsCarryNoStorageIDs(t *testing.T) {
	assertNoStorageID(t, "Next", Next{Repo: "qnib/ftest", Tag: "latest"})
	assertNoStorageID(t, "ProfileDetailResponse", ProfileDetailResponse{
		Label: "generic#v1",
		Rules: []ProfileRuleResp{{Type: "anno", Op: "eq", Key: "k", Value: "v"}},
		Manipulations: []UIManipulation{{
			Label: "model/demo",
			Default: &UIManipuConfig{
				Cmd: &UIManipuCommand{Op: "replace", Value: []string{"sh"}},
			},
		}},
	})
	assertNoStorageID(t, "types.OrasArtifact", types.OrasArtifact{
		ID:           42,
		Repo:         "qnib/ftest",
		Digest:       "sha256:abc",
		ArtifactType: "application/vnd.cyclonedx",
	})
	assertNoStorageID(t, "types.Manipulation", types.Manipulation{ID: 42, Label: "model/demo"})
}

// TestNaturalKeyRoundTripAcrossHandlerRestart proves that a collection is
// resolvable by repo+tag with no handler-local or storage-ID state: the same
// payload comes back from a freshly constructed handler (the API restart in
// the GoCD dev pipeline recreates the process; this test recreates the
// handler on the same graph).
func TestNaturalKeyRoundTripAcrossHandlerRestart(t *testing.T) {
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

	body := []byte(`{"repo_name":"qnib/featuretest","tag_name":"generic"}`)
	fetch := func(h Handlers) map[string]json.RawMessage {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/collection", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		h.RepoGetOne(w, req)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
		assertNoStorageID(t, "collection response", json.RawMessage(raw["collection"]))
		return raw
	}

	first := fetch(h)

	// A fresh handler represents a new API process: the same graph, no shared
	// in-memory state, no element ID carried over.
	restarted := NewHandlers("test", h.mag, nil, nil)
	second := fetch(restarted)

	// The collection's manifests are backed by a map, so the array order is
	// not stable. Compare the logical content by sorting on the digest.
	normalise := func(raw json.RawMessage) map[string]any {
		t.Helper()
		var col map[string]any
		require.NoError(t, json.Unmarshal(raw, &col))
		if manis, ok := col["manifests"].([]any); ok {
			sort.Slice(manis, func(i, j int) bool {
				di, _ := manis[i].(map[string]any)["digest"].(string)
				dj, _ := manis[j].(map[string]any)["digest"].(string)
				return di < dj
			})
			col["manifests"] = manis
		}
		return col
	}
	assert.Equal(t, normalise(first["collection"]), normalise(second["collection"]),
		"the resource must be identical when re-resolved by its natural key")
}
