package handlers

import (
	"net/http"
	"strings"

	"github.com/StudioSol/set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/utils"
	"github.com/metahub-registry/metahub-core/pkg/types"
	mlibUtils "github.com/metahub-registry/metahub-core/pkg/utils"
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
func (h Handlers) RepoGetOne(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := mlibUtils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "RepoGetOne",
	})
	defer span.End()
	type Payload struct {
		RepoName string `json:"repo_name"`
		Repo     string `json:"repo"`
		Tag      string `json:"tag"`
		TagName  string `json:"tag_name"`
		ColID    int64  `json:"collection_id"`
		TagID    int64  `json:"tag_id"`
	}
	pl := Payload{}
	breadCrumbs := []utils.BreadCrumb{}
	err := h.readJSON(w, r, &pl)
	if err != nil {
		logrus.Errorf("RepoGetOne (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	if pl.RepoName == "" && pl.Repo != "" {
		pl.RepoName = pl.Repo
	}
	if pl.TagName == "" && pl.Tag != "" {
		pl.TagName = pl.Tag
	}
	mlibUtils.Info(l, span, "Payload: %s:%s", pl.RepoName, pl.TagName)
	var col types.CollectionListing
	if pl.RepoName != "" && pl.TagName != "" {
		l = l.WithFields(logrus.Fields{
			"repo_name": pl.RepoName,
			"tag_name":  pl.TagName,
		})
		l.Info("RepoGetOne()")
		col, err = h.mag.Database.CollectionListingGetByName(ctx, pl.RepoName, pl.TagName)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
	} else {
		l = l.WithFields(logrus.Fields{
			"collection_id": pl.ColID,
			"tag_id":        pl.TagID,
		})
		l.Info("RepoGetOne()")
		col, err = h.mag.Database.CollectionListingGetByID(ctx, pl.ColID, pl.TagID)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
	}
	mlibUtils.Debug(l, span, " -> %s:%s", col.Repo, col.Tag)
	// Artifact filtering (non-image config media types) is applied at the database layer
	// in metalib (ManifestDataGetByCollectionTag). No handler-level filtering is needed.
	// Create Breadcrumbs for the collection
	reps := set.NewLinkedHashSetString()
	for idx, r := range strings.Split(col.Repo, "/") {
		if !reps.InArray(r) {
			reps.Add(r)
			bc := utils.BreadCrumb{
				Crumb:  r,
				Active: 1,
				Link:   strings.Join(reps.AsSlice(), "/"),
				Repo:   strings.Join(reps.AsSlice(), "/"),
				Tag:    col.Tag,
				IDX:    idx,
			}
			breadCrumbs = append(breadCrumbs, bc)
		}
	}
	/*
		manis, err := h.mag.Database.ManifestDataGetByCollectionTag(ctx, pl.ColID, pl.TagID)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
		for _, mani := range manis {
			key := fmt.Sprintf("%s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
			col.Manifests[key] = mani
		}
	*/
	// Mark the last breadcrumb as active (non-clickable current position)
	if len(breadCrumbs) > 0 {
		breadCrumbs[len(breadCrumbs)-1].Active = 0
	}
	mlibUtils.Debug(l, span, "Found collection '%s:%s' with %d manifests", col.Repo, col.Tag, len(col.Manifests))
	uiDetail := toUICollectionDetail(col)
	// Resolve profile labels for each manifest.
	if len(uiDetail.Manifests) > 0 {
		digests := make([]string, len(uiDetail.Manifests))
		for i, m := range uiDetail.Manifests {
			digests[i] = m.Digest
		}
		profileMap, pErr := h.mag.Database.ManifestGetProfileLabels(ctx, digests)
		if pErr != nil {
			mlibUtils.Debug(l, span, "ManifestGetProfileLabels failed (non-fatal): %v", pErr)
		} else {
			for i := range uiDetail.Manifests {
				if labels, ok := profileMap[uiDetail.Manifests[i].Digest]; ok {
					uiDetail.Manifests[i].Profiles = labels
				}
			}
		}
	}
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
