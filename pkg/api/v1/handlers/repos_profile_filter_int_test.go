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

// Integration tests for the `profile` filter field added to RepoGetOne
// (POST /api/v1/collection) by add-collection-detail-profile-filter.
//
// These tests verify the wire-level handler honours the field. The
// pure filter logic is covered in repos_profile_filter_test.go.

// bootstrapProfileFilterFixture builds the same featuretest graph as
// the existing TestCollectionGetOne_UIResponseShape: amd64 + arm64
// manifests for qnib/featuretest, bound to the `generic` profile via
// rules/generic/linux.yaml. Returns the resolved collection_id + tag_id
// for the `generic` tag.
func bootstrapProfileFilterFixture(t *testing.T, ctx context.Context) (int64, int64, func()) {
	t.Helper()
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

	colID, err := br.GetInt64("collection:ft#id")
	require.NoError(t, err)
	tagID, err := br.GetInt64("tag:generic#id")
	require.NoError(t, err)

	return colID, tagID, func() { TeardownTest(bs, br) }
}

func postRepoGetOne(t *testing.T, body ReqPayload) CollectionResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collection",
		bytes.NewBuffer(body.ToByte()))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetOne(w, req)
	require.Equal(t, http.StatusOK, w.Code, "RepoGetOne returned %d: %s",
		w.Code, w.Body.String())
	var resp CollectionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// TestRepoGetOne_ProfileFilter_AllManifestsWhenAbsent is the backward-
// compatibility guard: a request without a `profile` field must return
// every manifest pinned to the tag (the prior contract).
//
// NOTE on this fixture: the bootstrap path used here does not create the
// Slot→OF_PROFILE→Profile edges that ManifestGetProfileLabels traverses,
// so each manifest's resolved `profiles` slice is empty. That makes this
// fixture suitable for proving (a) the unfiltered path is unaffected and
// (b) ANY non-empty profile filter drops everything (since no manifest
// can match an empty profile set) — i.e. that the filter is wired into
// the handler. The positive narrowing case (one matching manifest kept)
// is covered exhaustively by the pure unit tests in
// repos_profile_filter_test.go, which exercise filterManifestsByProfile
// against hand-built UIManifest slices with populated Profiles.
func TestRepoGetOne_ProfileFilter_AllManifestsWhenAbsent(t *testing.T) {
	ctx := context.Background()
	colID, tagID, teardown := bootstrapProfileFilterFixture(t, ctx)
	defer teardown()

	resp := postRepoGetOne(t, ReqPayload{ColID: colID, TagID: tagID})
	assert.Equal(t, "qnib/featuretest", resp.Collection.Name)
	assert.NotEmpty(t, resp.Collection.Manifests,
		"absent profile must return manifests (backwards-compat)")
}

// TestRepoGetOne_ProfileFilter_EmptyTreatedAsAbsent asserts that an
// explicit empty / whitespace-only `profile` is treated identically to
// an omitted field: the full manifest set is returned (no filter). This
// guards the normalizeProfile() contract at the wire level.
func TestRepoGetOne_ProfileFilter_EmptyTreatedAsAbsent(t *testing.T) {
	ctx := context.Background()
	colID, tagID, teardown := bootstrapProfileFilterFixture(t, ctx)
	defer teardown()

	absent := postRepoGetOne(t, ReqPayload{ColID: colID, TagID: tagID})
	require.NotEmpty(t, absent.Collection.Manifests,
		"precondition: omitted profile returns manifests")

	for _, p := range []string{"", "   ", "\t"} {
		got := postRepoGetOne(t, ReqPayload{ColID: colID, TagID: tagID, Profile: p})
		assert.Equal(t, len(absent.Collection.Manifests),
			len(got.Collection.Manifests),
			"empty/whitespace profile %q must behave like no filter", p)
	}
}

// TestRepoGetOne_ProfileFilter_FilterIsWired asserts the handler actually
// applies the profile filter: with a non-empty `profile` and a fixture
// whose manifests carry an empty profiles slice, the result must narrow
// to zero manifests (vs. the non-empty unfiltered result). This proves
// the field is parsed and the filter loop runs — the inverse of the
// "absent" backward-compat test.
func TestRepoGetOne_ProfileFilter_FilterIsWired(t *testing.T) {
	ctx := context.Background()
	colID, tagID, teardown := bootstrapProfileFilterFixture(t, ctx)
	defer teardown()

	unfiltered := postRepoGetOne(t, ReqPayload{ColID: colID, TagID: tagID})
	require.NotEmpty(t, unfiltered.Collection.Manifests,
		"precondition: unfiltered call returns manifests")

	filtered := postRepoGetOne(t, ReqPayload{
		ColID:   colID,
		TagID:   tagID,
		Profile: "generic",
	})
	assert.Empty(t, filtered.Collection.Manifests,
		"a non-empty profile filter must drop manifests whose resolved "+
			"profiles slice does not include the label (here: empty set)")
}

// TestRepoGetOne_ProfileFilter_NoMatchReturnsEmptyArray asserts that a
// profile no manifest matches yields 200 with `collection.manifests: []`
// rather than 404. This is the documented behaviour (see the
// collections-api spec, scenario "Profile filter with no matches").
func TestRepoGetOne_ProfileFilter_NoMatchReturnsEmptyArray(t *testing.T) {
	ctx := context.Background()
	colID, tagID, teardown := bootstrapProfileFilterFixture(t, ctx)
	defer teardown()

	w := httptest.NewRecorder()
	body := ReqPayload{
		ColID:   colID,
		TagID:   tagID,
		Profile: "cpu#strix-halo-nonexistent",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collection",
		bytes.NewBuffer(body.ToByte()))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetOne(w, req)
	require.Equal(t, http.StatusOK, w.Code,
		"unknown profile must NOT 404, must return 200 with empty manifests")

	// The envelope must still be valid (name/tag/breadcrumbs/repos present).
	var resp CollectionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "qnib/featuretest", resp.Collection.Name)
	assert.Empty(t, resp.Collection.Manifests,
		"unknown profile must produce empty manifests array")
	assert.NotEmpty(t, resp.BCrumbs,
		"breadcrumbs must remain populated even when filter drops all manifests")

	// JSON wire shape: manifests must be `[]`, not `null`.
	assert.Contains(t, w.Body.String(), `"manifests":[]`,
		"empty manifests must serialise as [] not null")
}
