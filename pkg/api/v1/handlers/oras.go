package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/StudioSol/set"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/internal/utils"
	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
	mlibUtils "github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameOras = "handlers/oras.go"
)

// OrasNext holds navigation metadata for a single entry in the ORAS repo tree.
type OrasNext struct {
	Repo          string `json:"repo"`
	ArtifactType  string `json:"artifact_type"`
	ArtifactCount int64  `json:"artifact_count"`
}

// orasUpdateNext adds or merges a listing into the next-navigation map.
// When the query repo depth matches the listing depth the entry is keyed by artifact type (leaf);
// when the listing is deeper it is keyed by the next path segment (intermediate node).
// Returns 1 if the entry is a leaf (slug_end), 0 otherwise.
func orasUpdateNext(next map[string]OrasNext, listing mlibTypes.OrasListing, repo string, repoSlug []string) int {
	listingSlug := strings.Split(listing.Repo, "/")
	switch {
	case len(repoSlug) == len(listingSlug):
		next[listing.ArtifactType] = OrasNext{
			Repo:          listing.Repo,
			ArtifactType:  listing.ArtifactType,
			ArtifactCount: listing.ArtifactCount,
		}
		return 1
	case len(repoSlug) < len(listingSlug):
		seg := strings.Split(strings.TrimPrefix(strings.TrimPrefix(listing.Repo, repo), "/"), "/")[0]
		if seg != "" {
			existing := next[seg]
			next[seg] = OrasNext{Repo: seg, ArtifactCount: existing.ArtifactCount + listing.ArtifactCount}
		}
	}
	return 0
}

// orasAppendBreadCrumbs extends breadCrumbs with any new matching path segments from listing.Repo.
func orasAppendBreadCrumbs(breadCrumbs []utils.BreadCrumb, reps *set.LinkedHashSetString, listing mlibTypes.OrasListing, repoSlug []string) []utils.BreadCrumb {
	for i, seg := range strings.Split(listing.Repo, "/") {
		if i+1 > len(repoSlug) || seg != repoSlug[i] || reps.InArray(seg) {
			continue
		}
		reps.Add(seg)
		breadCrumbs = append(breadCrumbs, utils.BreadCrumb{
			Crumb:  seg,
			Active: 1,
			Link:   strings.Join(reps.AsSlice(), "/"),
			Repo:   strings.Join(reps.AsSlice(), "/"),
			IDX:    i,
		})
	}
	return breadCrumbs
}

// OrasGetAll handles GET /api/v1/oras
// It accepts an optional ?repo= query parameter and returns a navigation tree of ORAS artifact repos.
func (h Handlers) OrasGetAll(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := mlibUtils.InitLogging(r.Context(), tracer, map[string]any{
		"package": pkgNameOras,
		"func":    "OrasGetAll",
	})
	defer span.End()

	repo := r.URL.Query().Get("repo")
	l = l.WithField("repo", repo)
	l.Infof("OrasGetAll: repo=%q", repo)

	listings, err := h.mag.Database.OrasArtifactGetByRepoLike(ctx, repo)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "OrasArtifactGetByRepoLike failed")
		h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}

	repoSlug := []string{}
	if repo != "" {
		repoSlug = strings.Split(repo, "/")
	}

	next := map[string]OrasNext{}
	reps := set.NewLinkedHashSetString()
	breadCrumbs := []utils.BreadCrumb{}
	slugEnd := 0

	for _, listing := range listings {
		slugEnd |= orasUpdateNext(next, listing, repo, repoSlug)
		breadCrumbs = orasAppendBreadCrumbs(breadCrumbs, reps, listing, repoSlug)
	}

	if len(breadCrumbs) > 0 {
		breadCrumbs[len(breadCrumbs)-1].Active = 0
	}

	logrus.Debugf("OrasGetAll: repo=%q next=%d slugEnd=%d", repo, len(next), slugEnd)
	_ = h.writeJSON(w, http.StatusOK, envelope{
		"repos":        reps.AsSlice(),
		"bread_crumbs": breadCrumbs,
		"next":         next,
		"slug_end":     slugEnd,
	})
}

// orasBreadCrumbs builds breadcrumb navigation entries from a repo path (e.g. "qnib/featuretest").
func orasBreadCrumbs(repo string) []utils.BreadCrumb {
	if repo == "" {
		return []utils.BreadCrumb{}
	}
	segments := strings.Split(repo, "/")
	crumbs := make([]utils.BreadCrumb, 0, len(segments))
	for i, seg := range segments {
		link := strings.Join(segments[:i+1], "/")
		active := 1
		if i == len(segments)-1 {
			active = 0
		}
		crumbs = append(crumbs, utils.BreadCrumb{
			Crumb:  seg,
			Active: active,
			Link:   link,
			Repo:   link,
			IDX:    i,
		})
	}
	return crumbs
}

// OrasGetOne handles GET /api/v1/oras/artifact
// It accepts ?repo=&artifact_type= or ?id= query parameters.
func (h Handlers) OrasGetOne(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := mlibUtils.InitLogging(r.Context(), tracer, map[string]any{
		"package": pkgNameOras,
		"func":    "OrasGetOne",
	})
	defer span.End()

	q := r.URL.Query()
	idStr := q.Get("id")
	repo := q.Get("repo")
	artifactType := q.Get("artifact_type")

	if idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			h.writeJSON(w, http.StatusBadRequest, envelope{"error": "invalid id parameter"})
			return
		}
		artifact, err := h.mag.Database.OrasArtifactGetByID(ctx, id)
		if err != nil {
			mlibUtils.ErrorHandler(l, span, err, "OrasArtifactGetByID failed")
			h.writeJSON(w, http.StatusNotFound, envelope{"error": "artifact not found"})
			return
		}
		breadCrumbs := orasBreadCrumbs(artifact.Repo)
		_ = h.writeJSON(w, http.StatusOK, envelope{"artifact": artifact, "bread_crumbs": breadCrumbs})
		return
	}

	if repo == "" || artifactType == "" {
		h.writeJSON(w, http.StatusBadRequest, envelope{"error": "repo and artifact_type, or id, are required"})
		return
	}

	l = l.WithFields(logrus.Fields{"repo": repo, "artifact_type": artifactType})
	l.Infof("OrasGetOne: repo=%q artifact_type=%q", repo, artifactType)

	artifact, err := h.mag.Database.OrasArtifactGetByName(ctx, repo, artifactType)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "OrasArtifactGetByName failed")
		h.writeJSON(w, http.StatusNotFound, envelope{"error": "artifact not found"})
		return
	}

	breadCrumbs := orasBreadCrumbs(repo)
	_ = h.writeJSON(w, http.StatusOK, envelope{"artifact": artifact, "bread_crumbs": breadCrumbs})
}
