package handlers

import "net/http"

func (h Handlers) RedirectToWWW(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://app.metahub-registry.org", http.StatusTemporaryRedirect)
}
