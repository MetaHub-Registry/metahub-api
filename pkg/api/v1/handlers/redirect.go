package handlers

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	defURL = "https://frontend.metahub-registry.org"
)

func (h Handlers) RedirectToWWW(w http.ResponseWriter, r *http.Request) {
	fr := h.mag.Config.RedirectDoc
	if fr == "" {
		logrus.Warnf("RedirectDoc is empty, use default '%s'", defURL)
		fr = defURL
	}
	http.Redirect(w, r, fr, http.StatusTemporaryRedirect)
}
