package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/StudioSol/set"
	"github.com/sirupsen/logrus"
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
		Repo string `json:"repo"`
		Tag  string `json:"tag"`
	}
	pl := Payload{}
	err := h.readJSON(w, r, &pl)
	if err != nil {
		logrus.Errorf("CollectionGetOne (body: %v, payload:%v): %v", r.Body, pl, err)
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	col, err := h.models.Collection.GetCollectionByRepoTag(pl.Repo, pl.Tag)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	manis, err := h.models.Manifest.GetByCollectionId(col.ID)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	for _, mani := range manis {
		key := fmt.Sprintf("%s/%s:%s", mani.Upstream.Reg, mani.Upstream.Repo, mani.Upstream.Tag)
		col.Manifests[key] = mani
	}
	reps := strings.Split(pl.Repo, "/")
	reps = append(reps, pl.Tag)
	payload := envelope{
		"repos":      reps,
		"collection": col,
	}
	_ = h.writeJSON(w, http.StatusOK, payload)
}

func (h Handlers) CollectionGetAll(w http.ResponseWriter, r *http.Request) {
	logrus.Info("CollectionGetAll")
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
	repoSlug := strings.Split(pl.Repo, "/")
	cols, err := h.models.Collection.GetCollectionByRepo(pl.Repo)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	slugEnd := 0
	reps := set.NewLinkedHashSetString()
	next := map[string]Next{}
	type BreadCrumb struct {
		Crumb  string `json:"crumb"`
		Active int    `json:"active"`
		Link   string `json:"link"`
	}
	breadCrumbs := []BreadCrumb{}
	for _, col := range cols {
		colSlug := strings.Split(col.Repo, "/")
		switch {
		case len(repoSlug) == len(colSlug):
			logrus.Infof("We ARE at the end of the collection search: \ncolSlug: %v\nrepoSlug: %v", colSlug, repoSlug)
			// Add Tags to next with the complete n
			next[col.Tag] = Next{
				Repo:      col.Repo,
				Tag:       col.Tag,
				ID:        col.ID,
				ColCount:  1,
				ManiCount: col.ManifestCount,
			}
			slugEnd = 1
		case len(repoSlug) < len(colSlug):
			logrus.Infof("We ARE NOT at the end of the collection search: \ncolSlug: %v\nrepoSlug: %v", colSlug, repoSlug)
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
					bc := BreadCrumb{
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
