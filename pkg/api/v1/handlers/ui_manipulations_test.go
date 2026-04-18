package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestToUIManipulations_EmptyReturnsNonNilSlice ensures the converter
// emits a non-nil empty slice so the JSON payload is deterministic.
func TestToUIManipulations_EmptyReturnsNonNilSlice(t *testing.T) {
	out := toUIManipulations(nil)
	assert.NotNil(t, out)
	assert.Len(t, out, 0)
}

// TestToUIManipulation_LegacyFlatLayers populates `layers` on the DTO when
// only the legacy flat form is present.
func TestToUIManipulation_LegacyFlatLayers(t *testing.T) {
	mp := types.Manipulation{
		ID:    42,
		Label: "legacy#1",
		ManipuLayers: []types.ManipuLayer{
			{Rule: "arch:arm64=true"},
			{Rule: "arch:amd64=true"},
		},
	}
	ui := toUIManipulation(mp)
	assert.Equal(t, int64(42), ui.ID)
	assert.Equal(t, "legacy#1", ui.Label)
	assert.Nil(t, ui.Default, "legacy manipulation has no default block")
	assert.Empty(t, ui.Variants, "legacy manipulation has no variants")
	require.Len(t, ui.Layers, 2)
	assert.Equal(t, "arch:arm64=true", ui.Layers[0].Rule)
}

// TestToUIManipulation_ExtendedFullShape roundtrips a manipulation with
// default + variants + commands into its DTO form.
func TestToUIManipulation_ExtendedFullShape(t *testing.T) {
	mp := types.Manipulation{
		ID:    7,
		Label: "extended#1",
		Default: types.ManipuConfig{
			Env:        map[string]string{"FOO": "bar"},
			Workdir:    "/opt/ws",
			User:       "1000:1000",
			Volumes:    []string{"/data"},
			Entrypoint: &types.ManipuCommand{Op: types.ManipuCmdOpReplace, Value: []string{"/bin/entry"}},
			Cmd:        &types.ManipuCommand{Op: types.ManipuCmdOpAppend, Value: []string{"--loud"}},
		},
		Variants: []types.ManipuVariant{
			{
				Rule: "arch:arm64=true",
				Config: types.ManipuConfig{
					Env: map[string]string{"ARCH": "arm64"},
				},
			},
		},
	}
	ui := toUIManipulation(mp)
	require.NotNil(t, ui.Default)
	assert.Equal(t, "/opt/ws", ui.Default.Workdir)
	assert.Equal(t, "bar", ui.Default.Env["FOO"])
	require.NotNil(t, ui.Default.Entrypoint)
	assert.Equal(t, "replace", ui.Default.Entrypoint.Op)
	require.NotNil(t, ui.Default.Cmd)
	assert.Equal(t, "append", ui.Default.Cmd.Op)
	require.Len(t, ui.Variants, 1)
	assert.Equal(t, "arch:arm64=true", ui.Variants[0].Rule)
	assert.Equal(t, "arm64", ui.Variants[0].Config.Env["ARCH"])
	assert.Empty(t, ui.Layers, "extended manipulation should not surface legacy layers")
}

// TestToUIManipulation_ZeroCommandPointersDropped ensures that a pointer
// to a zero-valued ManipuCommand is treated the same as nil.
func TestToUIManipulation_ZeroCommandPointersDropped(t *testing.T) {
	mp := types.Manipulation{
		Label: "zcmd#1",
		Default: types.ManipuConfig{
			Workdir:    "/x",
			Entrypoint: &types.ManipuCommand{}, // zero
			Cmd:        nil,
		},
	}
	ui := toUIManipulation(mp)
	require.NotNil(t, ui.Default)
	assert.Nil(t, ui.Default.Entrypoint)
	assert.Nil(t, ui.Default.Cmd)
}

// TestToUIManipulation_JSONShape verifies the emitted JSON has the
// snake_case fields the UI expects and omits zero-valued optionals.
func TestToUIManipulation_JSONShape(t *testing.T) {
	mp := types.Manipulation{
		ID:    1,
		Label: "json#1",
		Default: types.ManipuConfig{
			Workdir: "/opt",
		},
	}
	ui := toUIManipulation(mp)
	b, err := json.Marshal(ui)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"label":"json#1"`)
	assert.Contains(t, s, `"default":{"workdir":"/opt"}`)
	// Omit empty.
	assert.NotContains(t, s, `"variants":null`)
	assert.NotContains(t, s, `"layers":null`)
}

// TestUIManifest_JSONIncludesConfigLayersWhenPresent asserts the
// serialized shape when config + layers are populated.
func TestUIManifest_JSONIncludesConfigLayersWhenPresent(t *testing.T) {
	m := UIManifest{
		Digest: "sha256:abc",
		Config: &UIDescriptor{Digest: "sha256:cfg", MediaType: "application/vnd.oci.image.config.v1+json", Size: 123},
		Layers: []UIDescriptor{
			{Digest: "sha256:lyr1", MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Size: 999},
		},
		Profiles: []string{},
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"config":{"digest":"sha256:cfg"`)
	assert.Contains(t, s, `"layers":[{"digest":"sha256:lyr1"`)
}
