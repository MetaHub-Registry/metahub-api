package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/StudioSol/set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/utils"
)

type Next struct {
	Repo      string `json:"repo"`
	Tag       string `json:"tag"` // Tag is added when repo slug is exhausted
	ID        int64  `json:"id"`  // ID is added when repo slug is exhausted
	ColCount  int    `json:"collection_count"`
	ManiCount int64  `json:"manifest_count"`
}

func (h Handlers) CollectionGetOne(w http.ResponseWriter, r *http.Request) {
	type Payload struct {
		ColID int64 `json:"collection_id"`
	}
	pl := Payload{}
	breadCrumbs := []utils.BreadCrumb{}
	err := h.readJSON(w, r, &pl)
	if err != nil {
		logrus.Errorf("CollectionGetOne (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	col, err := h.mag.SQlite.Collection.GetCollectionByID(pl.ColID)
	logrus.Infof("CollectionGetOne(%d): %s:%s", pl.ColID, col.Repo, col.Tag.Tag)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	// Create Breadcrumbs for the collection
	reps := set.NewLinkedHashSetString()
	for _, r := range strings.Split(col.Repo, "/") {
		if !reps.InArray(r) {
			reps.Add(r)
			bc := utils.BreadCrumb{
				Crumb:  r,
				Active: 1,
				Link:   fmt.Sprintf("/collections/%s", strings.Join(reps.AsSlice(), "/")),
			}
			breadCrumbs = append(breadCrumbs, bc)
		}
	}
	manis, err := h.mag.SQlite.Manifest.GetByCollectionId(pl.ColID)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	for _, mani := range manis {
		key := fmt.Sprintf("%s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
		col.Manifests[key] = mani
	}
	payload := envelope{
		"repos":        reps,
		"bread_crumbs": breadCrumbs,
		"collection":   col,
	}
	_ = h.writeJSON(w, http.StatusOK, payload)
}

func (h Handlers) CollectionGetAll(w http.ResponseWriter, r *http.Request) {
	type Payload struct {
		Repo string `json:"repo"`
	}
	pl := Payload{}
	err := h.readJSON(w, r, &pl)
	if err != nil {
		logrus.Errorf("CollectionGetAll (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	logrus.Infof("CollectionGetAll: '%s'", pl.Repo)
	repoSlug := strings.Split(pl.Repo, "/")
	cols, err := h.mag.SQlite.Collection.GetCollectionsByRepoLike(pl.Repo)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	slugEnd := 0
	next := map[string]Next{}
	reps := set.NewLinkedHashSetString()
	breadCrumbs := []utils.BreadCrumb{}
	for _, col := range cols {
		colSlug := strings.Split(col.Repo, "/")
		switch {
		case len(repoSlug) == len(colSlug):
			// Add Tags to next with the complete n
			next[col.Tag.Tag] = Next{
				Repo:      col.Repo,
				Tag:       col.Tag.Tag,
				ID:        col.ID,
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
