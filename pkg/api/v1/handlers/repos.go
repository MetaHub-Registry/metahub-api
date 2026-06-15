package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/StudioSol/set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/utils"
	"github.com/metahub-registry/metahub-core/pkg/types"
	mlibUtils "github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameCol = "handlers/repos.go"
)

type Next struct {
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`           // Tag is added when repo slug is exhausted
	ColID     int64  `json:"collection_id"` // ID is added when repo slug is exhausted
	TagID     int64  `json:"tag_id"`        // ID is added when repo slug is exhausted
	ColCount  int    `json:"collection_count"`
	ManiCount int64  `json:"manifest_count"`
}

// RepoGetOne returns details for a single repo+tag combination.
// Formerly CollectionGetOne; renamed as part of the Path Z Collection→Repo transition.
//
// Profile filter:
// When the request body carries a non-empty `profile` field, the handler
// drops every `UIManifest` whose resolved `Profiles` slice does not include
// that label, AFTER the per-manifest `ManifestGetProfileLabels` decoration
// runs. This is symmetric with RepoGetAll's `profile` filter (which runs
// at the database layer via `CollectionListingGetByRepoLikeForProfile`);
// the difference is scope — RepoGetAll narrows which collections appear,
// RepoGetOne narrows which manifests inside a single collection appear.
// The post-decoration loop is cheap (a tag has ≤ a handful of manifests
// in practice) and keeps the metalib query surface unchanged.
// repoGetOnePayload is the request body for RepoGetOne. A collection is
// addressed EITHER by (repo_name, tag_name) OR by (collection_id,
// tag_id); the legacy `repo`/`tag` aliases are normalised onto the
// `*_name` fields. `profile` is an optional manifest filter (see the
// RepoGetOne doc comment).
type repoGetOnePayload struct {
	RepoName string `json:"repo_name"`
	Repo     string `json:"repo"`
	Tag      string `json:"tag"`
	TagName  string `json:"tag_name"`
	ColID    int64  `json:"collection_id"`
	TagID    int64  `json:"tag_id"`
	Profile  string `json:"profile"`
}

// normalise folds the legacy `repo`/`tag` aliases onto the canonical
// `repo_name`/`tag_name` fields.
func (pl *repoGetOnePayload) normalise() {
	if pl.RepoName == "" && pl.Repo != "" {
		pl.RepoName = pl.Repo
	}
	if pl.TagName == "" && pl.Tag != "" {
		pl.TagName = pl.Tag
	}
}

// resolveCollection loads the CollectionListing addressed by the
// payload — by name when both repo_name and tag_name are present, else
// by numeric IDs.
func (h Handlers) resolveCollection(ctx context.Context, pl repoGetOnePayload) (types.CollectionListing, error) {
	if pl.RepoName != "" && pl.TagName != "" {
		return h.mag.Database.CollectionListingGetByName(ctx, pl.RepoName, pl.TagName)
	}
	return h.mag.Database.CollectionListingGetByID(ctx, pl.ColID, pl.TagID)
}

// buildCollectionBreadcrumbs derives the breadcrumb trail and the
// de-duplicated repo-segment set from a collection's repo path. The
// final breadcrumb is marked active==0 (current, non-clickable).
func buildCollectionBreadcrumbs(col types.CollectionListing) ([]utils.BreadCrumb, *set.LinkedHashSetString) {
	reps := set.NewLinkedHashSetString()
	breadCrumbs := []utils.BreadCrumb{}
	for idx, seg := range strings.Split(col.Repo, "/") {
		if reps.InArray(seg) {
			continue
		}
		reps.Add(seg)
		breadCrumbs = append(breadCrumbs, utils.BreadCrumb{
			Crumb:  seg,
			Active: 1,
			Link:   strings.Join(reps.AsSlice(), "/"),
			Repo:   strings.Join(reps.AsSlice(), "/"),
			Tag:    col.Tag,
			IDX:    idx,
		})
	}
	if len(breadCrumbs) > 0 {
		breadCrumbs[len(breadCrumbs)-1].Active = 0
	}
	return breadCrumbs, reps
}

// decorateManifestProfiles resolves which profile label(s) serve each
// manifest and stamps them onto the UIManifest.Profiles slice. Non-fatal:
// a lookup failure leaves Profiles empty and logs at debug level.
func (h Handlers) decorateManifestProfiles(ctx context.Context, l *logrus.Entry, span trace.Span, manis []UIManifest) {
	if len(manis) == 0 {
		return
	}
	digests := make([]string, len(manis))
	for i, m := range manis {
		digests[i] = m.Digest
	}
	profileMap, pErr := h.mag.Database.ManifestGetProfileLabels(ctx, digests)
	if pErr != nil {
		mlibUtils.Debug(l, span, "ManifestGetProfileLabels failed (non-fatal): %v", pErr)
		return
	}
	for i := range manis {
		if labels, ok := profileMap[manis[i].Digest]; ok {
			manis[i].Profiles = labels
		}
	}
}

func (h Handlers) RepoGetOne(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := mlibUtils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "RepoGetOne",
	})
	defer span.End()
	pl := repoGetOnePayload{}
	if err := h.readJSON(w, r, &pl); err != nil {
		logrus.Errorf("RepoGetOne (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	pl.normalise()
	mlibUtils.Info(l, span, "Payload: %s:%s", pl.RepoName, pl.TagName)
	col, err := h.resolveCollection(ctx, pl)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	mlibUtils.Debug(l, span, " -> %s:%s", col.Repo, col.Tag)
	// Artifact filtering (non-image config media types) is applied at the database layer
	// in metalib (ManifestDataGetByCollectionTag). No handler-level filtering is needed.
	breadCrumbs, reps := buildCollectionBreadcrumbs(col)
	mlibUtils.Debug(l, span, "Found collection '%s:%s' with %d manifests", col.Repo, col.Tag, len(col.Manifests))
	uiDetail := toUICollectionDetail(col)
	h.decorateManifestProfiles(ctx, l, span, uiDetail.Manifests)
	// Profile filter (post-decoration, pre-hydration). Drops every
	// manifest whose resolved Profiles slice does not include the
	// requested label. See the package comment on RepoGetOne for the
	// rationale. Order-preserving.
	if pl.Profile != "" {
		uiDetail.Manifests = filterManifestsByProfile(uiDetail.Manifests, pl.Profile)
	}
	// Hydrate layer descriptors per manifest. Non-fatal: per-manifest
	// failures leave that manifest's layers empty and continue. Runs
	// AFTER the profile filter so dropped manifests don't trigger a
	// (wasted) Neo4j round-trip.
	h.hydrateManifestLayers(ctx, col, uiDetail.Manifests)
	// Ensure all manifests have a non-nil Profiles slice for JSON serialization.
	for i := range uiDetail.Manifests {
		if uiDetail.Manifests[i].Profiles == nil {
			uiDetail.Manifests[i].Profiles = []string{}
		}
	}
	payload := envelope{
		"repos":        reps.AsSlice(),
		"bread_crumbs": breadCrumbs,
		"collection":   uiDetail,
	}
	_ = h.writeJSON(w, http.StatusOK, payload)
}

// filterManifestsByProfile keeps only the UIManifest entries whose
// Profiles slice contains the requested profile label. Order-preserving.
// When `profile` is empty the input is returned unchanged (callers should
// gate this with `if profile != ""` to avoid an unnecessary copy).
//
// The filter is the post-decoration counterpart to
// CollectionListingGetByRepoLikeForProfile (which filters at the database
// layer for RepoGetAll). For RepoGetOne the manifest set is already
// bounded (≤ a handful per tag) and the `Profiles` slice has just been
// computed via ManifestGetProfileLabels, so filtering in-process is
// both cheaper than a second Neo4j round-trip and behaviourally identical
// to the UI's prior `manifestInProfile` client-side filter (commit
// 5ea7678 in metahub-ui).
func filterManifestsByProfile(uiManis []UIManifest, profile string) []UIManifest {
	if profile == "" {
		return uiManis
	}
	kept := make([]UIManifest, 0, len(uiManis))
	for _, m := range uiManis {
		for _, p := range m.Profiles {
			if p == profile {
				kept = append(kept, m)
				break
			}
		}
	}
	return kept
}

// hydrateManifestLayers populates each `UIManifest.Layers` slice with the
// layer blob descriptors for the corresponding metalib `Manifest`. The
// lookup is by digest so the UI slice order is preserved. Failures log
// debug-level and continue; no manifest is dropped.
func (h Handlers) hydrateManifestLayers(ctx context.Context, col types.CollectionListing, uiManis []UIManifest) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameCol,
		"func":    "hydrateManifestLayers",
	})
	// Build digest -> metalib Manifest.ID map for a single pass lookup.
	byDigest := make(map[string]int64, len(col.Manifests))
	for _, m := range col.Manifests {
		if m.Digest != "" {
			byDigest[m.Digest] = m.ID
		}
	}
	for i := range uiManis {
		id, ok := byDigest[uiManis[i].Digest]
		if !ok || id == 0 {
			continue
		}
		layers, err := h.mag.Database.GetLayersForManifestID(ctx, id)
		if err != nil {
			l.Debugf("GetLayersForManifestID(%d) failed (non-fatal): %v", id, err)
			continue
		}
		if len(layers) == 0 {
			continue
		}
		uiManis[i].Layers = make([]UIDescriptor, 0, len(layers))
		for _, lyr := range layers {
			uiManis[i].Layers = append(uiManis[i].Layers, UIDescriptor{
				Digest:    lyr.Digest,
				MediaType: lyr.MediaType,
				Size:      lyr.Size,
			})
		}
	}
}

// accumulateCollectionNext builds the next-navigation map and breadcrumbs from a list of collections.
func accumulateCollectionNext(cols []types.CollectionListing, repoSlug []string, baseRepo string) (next map[string]Next, reps *set.LinkedHashSetString, breadCrumbs []utils.BreadCrumb, slugEnd int) {
	next = map[string]Next{}
	reps = set.NewLinkedHashSetString()
	breadCrumbs = []utils.BreadCrumb{}
	for _, col := range cols {
		colSlug := strings.Split(col.Repo, "/")
		switch {
		case len(repoSlug) == len(colSlug):
			next[col.Tag] = Next{
				Repo:      col.Repo,
				Tag:       col.Tag,
				ColID:     col.ID,
				TagID:     col.ColTag.ID,
				ColCount:  1,
				ManiCount: col.ManifestCount,
			}
			slugEnd = 1
		case len(repoSlug) < len(colSlug):
			n := strings.TrimPrefix(col.Repo, baseRepo)
			n = strings.TrimPrefix(n, "/")
			n = strings.Split(n, "/")[0]
			if n != "" {
				if existing, ok := next[n]; !ok {
					next[n] = Next{Repo: n, ColCount: 1, ManiCount: col.ManifestCount}
				} else {
					next[n] = Next{Repo: n, ColCount: existing.ColCount + 1, ManiCount: existing.ManiCount + col.ManifestCount}
				}
			}
		}
		for i, seg := range strings.Split(col.Repo, "/") {
			if i+1 <= len(repoSlug) && seg == repoSlug[i] && !reps.InArray(seg) {
				reps.Add(seg)
				breadCrumbs = append(breadCrumbs, utils.BreadCrumb{
					Crumb:  seg,
					Active: 1,
					Link:   strings.Join(reps.AsSlice(), "/"),
					Repo:   strings.Join(reps.AsSlice(), "/"),
					IDX:    i,
				})
			}
		}
	}
	return
}

// RepoGetAll returns a navigable listing of all repos (with breadcrumb and next-navigation).
// Formerly CollectionGetAll; renamed as part of the Path Z Collection→Repo transition.
func (h Handlers) RepoGetAll(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": "api/repos.go",
		"func":    "RepoGetAll",
	})
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "RepoGetAll")
	defer span.End()
	type Payload struct {
		Repo    string `json:"repo"`
		Profile string `json:"profile"`
	}
	pl := Payload{}
	err := h.readJSON(w, r, &pl)
	if err != nil {
		l.Errorf("RepoGetAll (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	l = l.WithFields(logrus.Fields{"repo": pl.Repo, "profile": pl.Profile})
	l.Infof("RepoGetAll: '%s' (profile: '%s')", pl.Repo, pl.Profile)
	repoSlug := strings.Split(pl.Repo, "/")
	// When a profile is specified, filter collection listings to only those
	// served under that profile. Otherwise fall back to unfiltered.
	cols, err := h.mag.Database.CollectionListingGetByRepoLikeForProfile(ctx, pl.Repo, pl.Profile)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	l.Warnf("RepoGetAll: '%s' -> %d", pl.Repo, len(cols))
	next, reps, breadCrumbs, slugEnd := accumulateCollectionNext(cols, repoSlug, pl.Repo)
	if len(breadCrumbs) > 0 {
		breadCrumbs[len(breadCrumbs)-1].Active = 0
	}
	payload := envelope{
		"repos":        reps.AsSlice(),
		"bread_crumbs": breadCrumbs,
		"next":         next,
		"slug_end":     slugEnd,
	}
	_ = h.writeJSON(w, http.StatusOK, payload)
}
