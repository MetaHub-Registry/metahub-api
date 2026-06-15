package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for filterManifestsByProfile. Pure function — no Neo
// dependency. The integration counterpart that exercises the wire
// field lives in repos_test.go (TestCollectionGetOne_* family) and
// runs against a bootstrapped graph.

// normalizeProfile: an empty / whitespace-only profile must collapse to
// "" so it is treated as "no profile given"; a real label is trimmed.
func TestNormalizeProfile(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"spaces", "   ", ""},
		{"tab", "\t", ""},
		{"newline", "\n", ""},
		{"plain label", "cpu#zen2", "cpu#zen2"},
		{"padded label", "  cpu#zen2  ", "cpu#zen2"},
		{"generic", "generic", "generic"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, normalizeProfile(c.in))
		})
	}
}

// normalise() must apply the profile normalisation as part of payload
// canonicalisation, so RepoGetOne's `if profile != ""` guard sees "".
func TestRepoGetOnePayload_NormaliseProfile(t *testing.T) {
	pl := repoGetOnePayload{RepoName: "qnib/ftest", TagName: "latest", Profile: "   "}
	pl.normalise()
	assert.Equal(t, "", pl.Profile, "whitespace profile must normalise to empty (no filter)")

	pl2 := repoGetOnePayload{RepoName: "qnib/ftest", TagName: "latest", Profile: " cpu#zen2 "}
	pl2.normalise()
	assert.Equal(t, "cpu#zen2", pl2.Profile, "padded label must be trimmed")
}

func TestFilterManifestsByProfile_EmptyProfileReturnsAll(t *testing.T) {
	// Guard for the backwards-compat path: when no profile is set,
	// the handler must short-circuit before reaching the filter.
	// The helper itself ALSO short-circuits as a defence in depth.
	in := []UIManifest{
		{Digest: "sha256:a", Profiles: []string{"generic"}},
		{Digest: "sha256:b", Profiles: []string{"cpu#zen2"}},
		{Digest: "sha256:c", Profiles: nil},
	}
	out := filterManifestsByProfile(in, "")
	require.Len(t, out, 3)
	// Same backing array on the short-circuit path — explicitly fine,
	// the caller does not mutate the slice header in that branch.
	assert.Equal(t, in[0].Digest, out[0].Digest)
	assert.Equal(t, in[2].Digest, out[2].Digest)
}

func TestFilterManifestsByProfile_NarrowsToMatching(t *testing.T) {
	in := []UIManifest{
		{Digest: "sha256:a", Profiles: []string{"generic"}},
		{Digest: "sha256:b", Profiles: []string{"cpu#zen2", "generic"}},
		{Digest: "sha256:c", Profiles: []string{"cpu#zen3"}},
	}
	out := filterManifestsByProfile(in, "cpu#zen2")
	require.Len(t, out, 1)
	assert.Equal(t, "sha256:b", out[0].Digest)
	// Spec: filter is removal-only, the kept manifest's Profiles slice
	// is unchanged.
	assert.Equal(t, []string{"cpu#zen2", "generic"}, out[0].Profiles)
}

func TestFilterManifestsByProfile_PreservesOrder(t *testing.T) {
	in := []UIManifest{
		{Digest: "sha256:b", Profiles: []string{"generic"}},
		{Digest: "sha256:a", Profiles: []string{"generic", "cpu#zen2"}},
		{Digest: "sha256:c", Profiles: []string{"generic"}},
		{Digest: "sha256:d", Profiles: []string{"generic", "cpu#zen2"}},
	}
	out := filterManifestsByProfile(in, "generic")
	require.Len(t, out, 4)
	assert.Equal(t, "sha256:b", out[0].Digest)
	assert.Equal(t, "sha256:a", out[1].Digest)
	assert.Equal(t, "sha256:c", out[2].Digest)
	assert.Equal(t, "sha256:d", out[3].Digest)
}

func TestFilterManifestsByProfile_NoMatchReturnsEmptyNotNil(t *testing.T) {
	// Empty slice (NOT nil) so the JSON envelope serialises as `[]`,
	// not `null` — required by the existing
	// "Ensure all manifests have a non-nil Profiles slice" pattern
	// further down in RepoGetOne.
	in := []UIManifest{
		{Digest: "sha256:a", Profiles: []string{"generic"}},
		{Digest: "sha256:b", Profiles: []string{"cpu#zen2"}},
	}
	out := filterManifestsByProfile(in, "cpu#strix-halo")
	require.NotNil(t, out, "must return [] not nil for clean JSON serialisation")
	assert.Empty(t, out)
}

func TestFilterManifestsByProfile_NilProfilesIsExcluded(t *testing.T) {
	// A manifest with no resolved Profiles (e.g. ManifestGetProfileLabels
	// returned no labels for that digest) cannot match any non-empty
	// profile filter.
	in := []UIManifest{
		{Digest: "sha256:a", Profiles: nil},
		{Digest: "sha256:b", Profiles: []string{}},
		{Digest: "sha256:c", Profiles: []string{"generic"}},
	}
	out := filterManifestsByProfile(in, "generic")
	require.Len(t, out, 1)
	assert.Equal(t, "sha256:c", out[0].Digest)
}

func TestFilterManifestsByProfile_EmptyInput(t *testing.T) {
	out := filterManifestsByProfile([]UIManifest{}, "cpu#zen2")
	assert.NotNil(t, out)
	assert.Empty(t, out)
}
