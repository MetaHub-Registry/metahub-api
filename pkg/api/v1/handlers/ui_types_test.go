package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestToUIManifest_FieldMapping(t *testing.T) {
	annos, err := types.NewAnnotationsFromMap(map[string]string{
		"cpu:skylake":  "true",
		"cpu:icelake":  "true",
		"custom:label": "value",
	})
	require.NoError(t, err)

	md := types.Manifest{
		Digest:    "sha256:abc123",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      12345,
		Platform: types.Platform{
			Arch:    "amd64",
			OS:      "linux",
			Variant: "v8",
		},
		Upstream: types.Upstream{
			Reg:  "docker.io",
			Repo: "library/alpine",
			Tag:  "3.23",
		},
		Annotations: annos,
	}

	ui := toUIManifest(md)

	assert.Equal(t, "sha256:abc123", ui.Digest)
	assert.Equal(t, "application/vnd.oci.image.manifest.v1+json", ui.MediaType)
	assert.Equal(t, int64(12345), ui.Size)
	assert.Equal(t, "docker.io", ui.Registry)
	assert.Equal(t, "linux", ui.Platform.OS)
	assert.Equal(t, "amd64", ui.Platform.Architecture)
	assert.Equal(t, "v8", ui.Platform.Variant)
	assert.Contains(t, ui.Platform.Features, "cpu:skylake")
	assert.Contains(t, ui.Platform.Features, "cpu:icelake")
	assert.Len(t, ui.Platform.Features, 2, "only cpu: annotations with value 'true' are features")
	assert.Equal(t, "value", ui.Annotations["custom:label"])
}

func TestToUIManifest_JSONFieldNames(t *testing.T) {
	md := types.Manifest{
		Digest:    "sha256:abc123",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      100,
		Platform: types.Platform{
			Arch: "arm64",
			OS:   "linux",
		},
		Upstream: types.Upstream{
			Reg: "quay.io",
		},
	}

	ui := toUIManifest(md)
	b, err := json.Marshal(ui)
	require.NoError(t, err)

	body := string(b)
	// Must use snake_case "media_type", not camelCase "mediaType"
	assert.Contains(t, body, `"media_type"`)
	assert.NotContains(t, body, `"mediaType"`)
	// Must have "registry" at top level
	assert.Contains(t, body, `"registry"`)
	// Must have platform.architecture
	assert.Contains(t, body, `"architecture"`)
}

func TestToUICollectionDetail_ManifestsIsArray(t *testing.T) {
	col := types.CollectionListing{
		Repo: "alpine",
		Tag:  "3.23",
		Manifests: map[string]types.Manifest{
			"key1": {
				Digest:    "sha256:aaa",
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Size:      100,
				Platform:  types.Platform{Arch: "amd64", OS: "linux"},
			},
			"key2": {
				Digest:    "sha256:bbb",
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Size:      200,
				Platform:  types.Platform{Arch: "arm64", OS: "linux"},
			},
		},
	}

	ui := toUICollectionDetail(col)
	assert.Equal(t, "alpine", ui.Name)
	assert.Equal(t, "3.23", ui.Tag)
	assert.Len(t, ui.Manifests, 2)

	// Verify it serialises as a JSON array
	b, err := json.Marshal(ui)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &raw))

	// "manifests" must be an array, not an object
	var arr []json.RawMessage
	err = json.Unmarshal(raw["manifests"], &arr)
	require.NoError(t, err, "manifests must serialise as a JSON array")
	assert.Len(t, arr, 2)

	// "name" field must be present (not "repo")
	_, hasName := raw["name"]
	assert.True(t, hasName, "UICollectionDetail must serialise with 'name' field")
	_, hasRepo := raw["repo"]
	assert.False(t, hasRepo, "UICollectionDetail must NOT have 'repo' field")
}

func TestToUICollectionDetail_EmptyManifests(t *testing.T) {
	col := types.CollectionListing{
		Repo:      "test/empty",
		Tag:       "latest",
		Manifests: map[string]types.Manifest{},
	}

	ui := toUICollectionDetail(col)
	assert.Equal(t, "test/empty", ui.Name)
	assert.Equal(t, "latest", ui.Tag)
	assert.NotNil(t, ui.Manifests, "manifests must be non-nil (empty slice)")
	assert.Len(t, ui.Manifests, 0)

	b, err := json.Marshal(ui)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"manifests":[]`, "empty manifests must serialise as []")
}

func TestOrasBreadCrumbs(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		expected  int
		lastCrumb string
	}{
		{"empty repo", "", 0, ""},
		{"single segment", "alpine", 1, "alpine"},
		{"two segments", "qnib/featuretest", 2, "featuretest"},
		{"three segments", "a/b/c", 3, "c"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			crumbs := orasBreadCrumbs(tc.repo)
			assert.Len(t, crumbs, tc.expected)
			if tc.expected > 0 {
				last := crumbs[len(crumbs)-1]
				assert.Equal(t, 0, last.Active, "last breadcrumb must have active == 0")
				assert.Equal(t, tc.lastCrumb, last.Crumb)
				assert.Equal(t, tc.repo, last.Link)

				// All non-last crumbs must have active == 1
				for i := 0; i < len(crumbs)-1; i++ {
					assert.Equal(t, 1, crumbs[i].Active, "non-last breadcrumb must have active == 1")
				}
			}
		})
	}
}

func TestExtractFeatures(t *testing.T) {
	tests := []struct {
		name     string
		annos    map[string]string
		expected int
	}{
		{"no annotations", nil, 0},
		{"no cpu features", map[string]string{"foo": "bar"}, 0},
		{"cpu feature true", map[string]string{"cpu:skylake": "true"}, 1},
		{"cpu feature false", map[string]string{"cpu:skylake": "false"}, 0},
		{"mixed", map[string]string{"cpu:skylake": "true", "cpu:icelake": "true", "other": "true"}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			features := extractFeatures(tc.annos)
			assert.Len(t, features, tc.expected)
		})
	}
}
