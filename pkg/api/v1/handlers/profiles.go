package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/sirupsen/logrus"
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

	// Emit the canonical public label (slashes mapped to "#") so the listing
	// uses the same single-segment label the UI passes back to
	// GET /api/v1/profiles/{label}.
	for i := range profiles {
		profiles[i].Label = publicProfileLabel(profiles[i].Label)
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

	ps, found, err := h.resolveProfileWithStats(ctx, label)
	if err != nil {
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	if !found {
		_ = h.writeJSON(w, http.StatusNotFound, envelope{"error": "profile not found"})
		return
	}

	// Hydrate linked manipulations. Non-fatal: a failure here logs but
	// does not block the rest of the response.
	mps, mpErr := h.mag.Database.ManipulationsWithConfigByProfileID(ctx, ps.ID)
	if mpErr != nil {
		l.Warnf("ManipulationsWithConfigByProfileID failed (non-fatal): %v", mpErr)
	}

	rules := toProfileRuleResps(ps.Rules)
	resp := ProfileDetailResponse{
		Name: ps.Name,
		Tag:  ps.Tag,
		// Return the canonical public label (e.g. "arch#zen4#latest"), which is
		// the form the API/UI use to reference a profile, not the stored form
		// (e.g. "arch/zen4#latest") that keeps slashes in the name segment.
		Label:           publicProfileLabel(ps.Label),
		Rules:           rules,
		CollectionCount: ps.CollectionCount,
		ManifestCount:   ps.ManifestCount,
		Manipulations:   toUIManipulations(mps),
	}
	_ = h.writeJSON(w, http.StatusOK, resp)
}

// publicProfileLabel converts a stored profile label to its canonical public
// form. Profiles are stored as "name#tag" where the name may itself contain
// slashes (e.g. "arch/zen4#latest"). The public label the API/UI use replaces
// those slashes with "#" so the label is a single URL-path-safe segment
// (e.g. "arch#zen4#latest").
func publicProfileLabel(stored string) string {
	return strings.ReplaceAll(stored, "/", "#")
}

// resolveProfileWithStats resolves a profile by the label from the request.
// It first tries the label as-is (covers already-stored / slash-free labels),
// then falls back to matching the canonical public label (slashes mapped to
// "#") against every stored profile label. This makes single-GET work for the
// "#"-delimited public form even though labels are stored with slashes in the
// name segment.
func (h Handlers) resolveProfileWithStats(ctx context.Context, label string) (types.ProfileStats, bool, error) {
	ps, found, err := h.mag.Database.ProfileGetByLabelWithStats(ctx, label)
	if err != nil || found {
		return ps, found, err
	}
	// Fallback: the requested label is the public "#"-delimited form. Find the
	// stored profile whose public projection equals it, then re-fetch by the
	// stored label so stats/rules hydrate correctly.
	profiles, err := h.mag.Database.ProfileGetAll(ctx)
	if err != nil {
		return ps, false, err
	}
	for _, p := range profiles {
		if publicProfileLabel(p.Label) == label {
			return h.mag.Database.ProfileGetByLabelWithStats(ctx, p.Label)
		}
	}
	return ps, false, nil
}

// toProfileRuleResps converts metalib ProfileRule slice to response DTOs.
func toProfileRuleResps(rules []types.ProfileRule) []ProfileRuleResp {
	resps := make([]ProfileRuleResp, 0, len(rules))
	for _, r := range rules {
		resps = append(resps, ProfileRuleResp{
			Type:  r.Type,
			Op:    r.Op,
			Key:   r.Key,
			Value: r.Value,
		})
	}
	return resps
}
