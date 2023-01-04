package handlers

import "net/http"

func (h Handlers) RedirectToWWW(w http.ResponseWriter, r *http.Request) {
	fr := h.mag.Config.RedirectDoc
	if fr == "" {
		fr = "https://frontend.metahub-registry.org"
	}
	http.Redirect(w, r, fr, http.StatusTemporaryRedirect)
}
