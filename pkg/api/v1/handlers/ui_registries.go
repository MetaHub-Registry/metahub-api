package handlers

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	pkgNameRegistries = "handlers/ui_registries.go"
)

// registryEntry is the JSON representation of a configured upstream registry.
// It exposes the effective name and URL (hiding the legacy Hostname field)
// and never includes secrets (none exist today, but the DTO keeps hygienic).
type registryEntry struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	AllowAnon bool   `json:"allowAnon"`
	TLS       bool   `json:"tls"`
}

// RegistriesGetAll handles GET /api/v1/registries.
// Returns all configured upstream registries available for pullthrough.
// The endpoint is public (no authentication required).
func (h Handlers) RegistriesGetAll(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameRegistries,
		"func":    "RegistriesGetAll",
	})
	_ = l

	regs := h.mag.GetRegistries()
	entries := make([]registryEntry, 0, len(regs))
	for _, reg := range regs {
		entries = append(entries, registryEntry{
			Name:      reg.EffectiveName(),
			URL:       reg.EffectiveURL(),
			AllowAnon: reg.AllowAnon,
			TLS:       reg.TLS,
		})
	}

	_ = h.writeJSON(w, http.StatusOK, envelope{
		"registries": entries,
	})
}
