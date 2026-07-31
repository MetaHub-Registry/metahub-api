package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// namespace_authz.go implements registry namespace authorization
// (openspec change add-registry-namespace-authz): classify a repo path into
// personal / project / system, resolve org/project role by constructing the
// Authentik group names, and gate pull (read) + push (write) accordingly. Gated
// by the two config switches (default OFF -> legacy behavior).

// nsClass is the classification of a repo namespace.
type nsClass int

const (
	nsSystem nsClass = iota
	nsPersonal
	nsProject
)

// nsInfo is the resolved namespace classification for a repo path.
type nsInfo struct {
	class      nsClass
	owner      string // personal: the owning username
	org        string // project: team slug
	project    string // project: project slug
	visibility string // project: private|public
}

// repoFromOCIPath extracts the <repo> namespace from an OCI /v2 request path
// (`/v2/<repo>/(manifests|blobs|tags)/...`). <repo> may be multi-segment.
func repoFromOCIPath(p string) string {
	p = strings.TrimPrefix(p, "/v2/")
	for _, sep := range []string{"/manifests/", "/blobs/", "/tags/"} {
		if i := strings.Index(p, sep); i >= 0 {
			return p[:i]
		}
	}
	return strings.Trim(p, "/")
}

// classifyNamespace classifies repo R deterministically:
//  1. user/<u>/... -> personal (owner u).
//  2. <org>/<project> backed by a :Project -> project (with visibility).
//  3. else -> system/unowned.
func (h Handlers) classifyNamespace(ctx context.Context, repo string) nsInfo {
	segs := strings.Split(repo, "/")
	if len(segs) >= 2 && segs[0] == types.PersonalTeamSlug && segs[1] != "" {
		return nsInfo{class: nsPersonal, owner: segs[1]}
	}
	if len(segs) >= 2 && segs[0] != "" && segs[1] != "" {
		org, project := segs[0], segs[1]
		if p, found, err := h.mag.Database.ProjectGet(ctx, org, project); err == nil && found {
			return nsInfo{class: nsProject, org: org, project: project, visibility: p.Visibility}
		}
	}
	return nsInfo{class: nsSystem}
}

// projGroups holds the four candidate Authentik group names for an org/project,
// constructed (never parsed) so metahub + the site-agent agree by construction.
type projGroups struct {
	orgMember, orgAdmin, projMember, projAdmin string
}

func projectGroups(org, project string) projGroups {
	base := types.GroupPrefix + org        // mh-<org>
	pbase := base + "-" + project          // mh-<org>-<project>
	return projGroups{
		orgMember:  base,
		orgAdmin:   base + "-admin",
		projMember: pbase,
		projAdmin:  pbase + "-admin",
	}
}

// hasAny reports whether groups contains any of names.
func hasAny(groups []string, names ...string) bool {
	set := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		set[g] = struct{}{}
	}
	for _, n := range names {
		if _, ok := set[n]; ok {
			return true
		}
	}
	return false
}

// canPull applies the pull column of the authorization matrix. The caller is
// already authenticated; global admins always pass.
func canPull(info nsInfo, username, role string, groups []string) bool {
	if role == roleAdmin {
		return true
	}
	switch info.class {
	case nsPersonal:
		return username == info.owner
	case nsProject:
		if info.visibility == types.VisibilityPublic {
			return true // any authenticated user
		}
		g := projectGroups(info.org, info.project)
		return hasAny(groups, g.orgMember, g.orgAdmin, g.projMember, g.projAdmin)
	default: // system/unowned
		return true // any authenticated user
	}
}

// canPush applies the push column of the authorization matrix.
func canPush(info nsInfo, username, role string, groups []string) bool {
	if role == roleAdmin {
		return true
	}
	switch info.class {
	case nsPersonal:
		return username == info.owner
	case nsProject:
		g := projectGroups(info.org, info.project)
		return hasAny(groups, g.orgAdmin, g.projAdmin)
	default: // system/unowned
		return false // only global admin
	}
}

// OCIReadAuthMiddleware gates GET/HEAD /v2 content by namespace class + groups
// when registry.enforce_pull_authz is on. OFF -> pass-through (legacy ungated
// reads). It runs AFTER OCIAuthMiddleware, so a valid identity is present; a
// caller without access to the namespace gets 403.
func (h Handlers) OCIReadAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !h.registry.EnforcePullAuthz || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}
			auth, ok := OCIUserFromContext(r.Context())
			if !ok {
				realm := realmForRequest(r)
				det := helper.NewDetail("repository", "", r.Method)
				unauthorized(w, r, realm, det, invalidToken, "authentication required")
				return
			}
			repo := repoFromOCIPath(r.URL.Path)
			info := h.classifyNamespace(r.Context(), repo)
			if !canPull(info, auth.Username, auth.Role, auth.Groups) {
				det := helper.NewDetail("repository", repo, r.Method)
				forbidden(w, r, det, "pull access denied for this namespace")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
