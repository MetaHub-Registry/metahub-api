package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

const (
	pkgNameProfiles = "handlers/profiles.go"
)

// ProfilesGetAll handles GET /api/v1/profiles.
// It returns all profiles stored in the Neo4j graph including their linked rules.
// The endpoint is public (no authentication required).
func (h Handlers) ProfilesGetAll(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfiles,
		"func":    "ProfilesGetAll",
	})
	ctx := r.Context()
	_ = l

	profiles, err := h.mag.Database.ProfileGetAll(ctx)
	if err != nil {
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}

	_ = h.writeJSON(w, http.StatusOK, envelope{
		"profiles": profiles,
	})
}

// ProfileGetOne handles GET /api/v1/profiles/{label}.
// Returns a single profile's detail including rules, collection count, and manifest count.
func (h Handlers) ProfileGetOne(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameProfiles,
		"func":    "ProfileGetOne",
	})
	ctx := r.Context()

	label := chi.URLParam(r, "label")
	if label == "" {
		_ = h.writeJSON(w, http.StatusBadRequest, envelope{"error": "profile label required"})
		return
	}
	l = l.WithField("label", label)
	l.Info("ProfileGetOne")

	ps, found, err := h.mag.Database.ProfileGetByLabelWithStats(ctx, label)
	if err != nil {
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	if !found {
		_ = h.writeJSON(w, http.StatusNotFound, envelope{"error": "profile not found"})
		return
	}

	rules := toProfileRuleResps(ps.Rules)
	resp := ProfileDetailResponse{
		ID:              ps.ID,
		Name:            ps.Name,
		Tag:             ps.Tag,
		Label:           ps.Label,
		Rules:           rules,
		CollectionCount: ps.CollectionCount,
		ManifestCount:   ps.ManifestCount,
	}
	_ = h.writeJSON(w, http.StatusOK, resp)
}

// toProfileRuleResps converts metalib ProfileRule slice to response DTOs.
func toProfileRuleResps(rules []types.ProfileRule) []ProfileRuleResp {
	resps := make([]ProfileRuleResp, 0, len(rules))
	for _, r := range rules {
		resps = append(resps, ProfileRuleResp{
			ID:    r.ID,
			Type:  r.Type,
			Op:    r.Op,
			Key:   r.Key,
			Value: r.Value,
		})
	}
	return resps
}
