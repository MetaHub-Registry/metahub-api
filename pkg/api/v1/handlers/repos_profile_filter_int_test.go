package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
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

// TestRepoGetAll_UnfilteredManifestCount documents the baseline: with
// no profile, each leaf's ManifestCount is the full tag count. The
// profile-narrowing path (profileFilteredManifestCount) only runs when
// a collection IS returned for the profile, which requires a
// Slot→OF_PROFILE→Profile binding the bootstrap fixture does not
// create — so the per-profile narrowing is covered by the pure unit
// test TestCountDigestsWithProfile and verified live on dev. Here we
// only assert the unfiltered counts are intact (regression guard that
// the override is gated behind a non-empty profile).
func TestRepoGetAll_UnfilteredManifestCount(t *testing.T) {
	ctx := context.Background()
	_, _, teardown := bootstrapProfileFilterFixture(t, ctx)
	defer teardown()

	raw, _ := json.Marshal(map[string]string{"repo": "qnib/featuretest"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collections",
		bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	h.RepoGetAll(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp CollectionsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Next, "expected tag leaves for qnib/featuretest")

	// The generic tag binds both manifests; without a profile filter its
	// count must be the full total (2), proving the override is skipped.
	gen, ok := resp.Next["generic"]
	require.True(t, ok, "expected a 'generic' tag leaf")
	assert.Equal(t, int64(2), gen.ManiCount,
		"unfiltered ManifestCount must be the full tag total")
}

// TestProfileFilteredManifestCount_Direct exercises the
// profileFilteredManifestCount wrapper directly against the bootstrapped
// graph. The fixture binds manifests to a collection but does not create
// the Slot→OF_PROFILE→Profile path ManifestGetProfileLabels traverses,
// so the resolved label set is empty and the count is 0 for any profile.
// This still covers the wrapper's digest-collection + DB-call + delegate
// path (the per-profile matching arithmetic is covered exhaustively by
// the pure TestCountDigestsWithProfile).
func TestProfileFilteredManifestCount_Direct(t *testing.T) {
	ctx := context.Background()
	colID, tagID, teardown := bootstrapProfileFilterFixture(t, ctx)
	defer teardown()

	col, err := h.mag.Database.CollectionListingGetByID(ctx, colID, tagID)
	require.NoError(t, err)
	require.NotEmpty(t, col.Manifests, "fixture must have manifests")

	l := logrus.WithField("test", "profileFilteredManifestCount")
	_, span := tracer.Start(ctx, "TestProfileFilteredManifestCount_Direct")
	defer span.End()

	got := h.profileFilteredManifestCount(ctx, l, span, col, "cpu#zen2")
	assert.Equal(t, int64(0), got,
		"fixture manifests resolve to no profile labels -> count 0")

	// Empty collection short-circuits to 0 without a DB call.
	empty := types.CollectionListing{Manifests: map[string]types.Manifest{}}
	assert.Equal(t, int64(0),
		h.profileFilteredManifestCount(ctx, l, span, empty, "cpu#zen2"))
}
