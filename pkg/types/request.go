package types

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/context"
)

type Request struct {
	Repo    string
	Tag     string
	Digest  string
	User    string
	Profile string
}

func NewRequest(r *http.Request) Request {
	parRepo, parImg, parRef := extractRepoName(r)
	var tag string
	var dig string
	if strings.HasPrefix(parRef, "sha256:") {
		dig = parRef
	} else {
		tag = parRef
	}
	repo := filepath.Join(parRepo, parImg)
	user := "anonymous"
	profile := "generic"
	vU := context.Get(r, "userName")
	if vU != nil {
		user = vU.(string)
	}
	vP := context.Get(r, "profileName")
	if vP != nil {
		profile = vP.(string)
	}
	return Request{
		Repo:    repo,
		Tag:     tag,
		Digest:  dig,
		User:    user,
		Profile: profile,
	}
}

func extractRepoName(r *http.Request) (repo, img, ref string) {
	repo = chi.URLParam(r, "repo")
	img = chi.URLParam(r, "image")
	ref = chi.URLParam(r, "reference")
	return
}
