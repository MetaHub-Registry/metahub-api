package handlers

import (
	"net/http"

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

	_ = h.writeJSON(w, http.StatusOK, envelope{
		"profiles": profiles,
	})
}
