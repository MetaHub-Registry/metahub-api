package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/StudioSol/set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/utils"
	"github.com/metahub-registry/metahub-core/pkg/types"
	mlibUtils "github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameCol = "handlers/collections.go"
)

type Next struct {
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`           // Tag is added when repo slug is exhausted
	ColID     int64  `json:"collection_id"` // ID is added when repo slug is exhausted
	TagID     int64  `json:"tag_id"`        // ID is added when repo slug is exhausted
	ColCount  int    `json:"collection_count"`
	ManiCount int64  `json:"manifest_count"`
}

func (h Handlers) CollectionGetOne(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := mlibUtils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameCol,
		"func":    "CollectionGetOne",
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
		logrus.Errorf("CollectionGetOne (body: %v, payload:%v): %v", r.Body, pl, err)
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
		l.Info("CollectionGetOne()")
		col, err = h.mag.Database.CollectionListing_Get_ByName(ctx, pl.RepoName, pl.TagName)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
	} else {
		l = l.WithFields(logrus.Fields{
			"collection_id": pl.ColID,
			"tag_id":        pl.TagID,
		})
		l.Info("CollectionGetOne()")
		col, err = h.mag.Database.CollectionListing_Get_ByID(ctx, pl.ColID, pl.TagID)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
	}
	mlibUtils.Debug(l, span, " -> %s:%s", col.Repo, col.Tag)
	// Create Breadcrumbs for the collection
	reps := set.NewLinkedHashSetString()
	for idx, r := range strings.Split(col.Repo, "/") {
		if !reps.InArray(r) {
			reps.Add(r)
			bc := utils.BreadCrumb{
				Crumb:  r,
				Active: 1,
				Link:   fmt.Sprintf("/collections/%s", strings.Join(reps.AsSlice(), "/")),
				Repo:   strings.Join(reps.AsSlice(), "/"),
				Tag:    col.Tag,
				IDX:    idx,
			}
			breadCrumbs = append(breadCrumbs, bc)
		}
	}
	/*
		manis, err := h.mag.Database.ManifestData_GetByCollectionTag(ctx, pl.ColID, pl.TagID)
		if err != nil {
			h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
			return
		}
		for _, mani := range manis {
			key := fmt.Sprintf("%s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
			col.Manifests[key] = mani
		}
	*/
	mlibUtils.Debug(l, span, "Found collection '%s:%s' with %d manifests", col.Repo, col.Tag, len(col.Manifests))
	payload := envelope{
		"repos":        reps.AsSlice(),
		"bread_crumbs": breadCrumbs,
		"collection":   col,
	}
	_ = h.writeJSON(w, http.StatusOK, payload)
}

func (h Handlers) CollectionGetAll(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": "api/collections.go",
		"func":    "CollectionGetAll",
	})
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "CollectionGetAll")
	defer span.End()
	type Payload struct {
		Repo string `json:"repo"`
	}
	pl := Payload{}
	err := h.readJSON(w, r, &pl)
	if err != nil {
		l.Errorf("CollectionGetAll (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	l = l.WithFields(logrus.Fields{
		"repo": pl.Repo,
	})
	l.Infof("CollectionGetAll: '%s'", pl.Repo)
	repoSlug := strings.Split(pl.Repo, "/")
	cols, err := h.mag.Database.CollectionListingGetByRepoLike(ctx, pl.Repo)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	l.Warnf("CollectionGetAll: '%s' -> %d", pl.Repo, len(cols))
	slugEnd := 0
	next := map[string]Next{}
	reps := set.NewLinkedHashSetString()
	breadCrumbs := []utils.BreadCrumb{}
	for _, col := range cols {
		colSlug := strings.Split(col.Repo, "/")
		switch {
		case len(repoSlug) == len(colSlug):
			// Add Tags to next with the complete n
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
			n := strings.TrimPrefix(col.Repo, pl.Repo)
			n = strings.TrimPrefix(n, "/")
			n = strings.Split(n, "/")[0]
			if n != "" {
				if _, ok := next[n]; !ok {
					next[n] = Next{
						Repo:      n,
						ColCount:  1,
						ManiCount: col.ManifestCount,
					}
				} else {
					next[n] = Next{
						Repo:      n,
						ColCount:  next[n].ColCount + 1,
						ManiCount: next[n].ManiCount + col.ManifestCount,
					}
				}
			}

		}
		for i, r := range strings.Split(col.Repo, "/") {
			if i+1 <= len(repoSlug) && r == repoSlug[i] {
				if !reps.InArray(r) {
					reps.Add(r)
					bc := utils.BreadCrumb{
						Crumb:  r,
						Active: 1,
						Link:   fmt.Sprintf("/collections/%s", strings.Join(reps.AsSlice(), "/")),
						Repo:   strings.Join(reps.AsSlice(), "/"),
						Tag:    "",
						IDX:    i,
					}
					breadCrumbs = append(breadCrumbs, bc)
				}
			}
		}
	}
	if len(breadCrumbs) > 0 {
		// Set the last one inactive
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
