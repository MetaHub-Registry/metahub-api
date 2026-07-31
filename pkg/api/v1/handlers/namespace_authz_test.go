package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-api/pkg/config"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestRepoFromOCIPath(t *testing.T) {
	cases := map[string]string{
		"/v2/acme/web/manifests/latest":   "acme/web",
		"/v2/acme/web/blobs/sha256:abc":   "acme/web",
		"/v2/user/alice/app/manifests/1":  "user/alice/app",
		"/v2/oci/tags/list":               "oci",
		"/v2/":                            "",
		"/v2/single/blobs/uploads/uuid-1": "single",
	}
	for path, want := range cases {
		assert.Equal(t, want, repoFromOCIPath(path), path)
	}
}

func TestProjectGroups(t *testing.T) {
	g := projectGroups("acme", "web")
	assert.Equal(t, "mh-acme", g.orgMember)
	assert.Equal(t, "mh-acme-admin", g.orgAdmin)
	assert.Equal(t, "mh-acme-web", g.projMember)
	assert.Equal(t, "mh-acme-web-admin", g.projAdmin)
}

func TestCanPull(t *testing.T) {
	personal := nsInfo{class: nsPersonal, owner: "alice"}
	priv := nsInfo{class: nsProject, org: "acme", project: "web", visibility: types.VisibilityPrivate}
	pub := nsInfo{class: nsProject, org: "acme", project: "web", visibility: types.VisibilityPublic}
	sys := nsInfo{class: nsSystem}

	// personal: owner or admin only
	assert.True(t, canPull(personal, "alice", roleUser, nil))
	assert.False(t, canPull(personal, "bob", roleUser, nil))
	assert.True(t, canPull(personal, "bob", roleAdmin, nil))
	// private project: any org/project group; non-member denied
	assert.True(t, canPull(priv, "u", roleUser, []string{"mh-acme-web"}))
	assert.True(t, canPull(priv, "u", roleUser, []string{"mh-acme"}))
	assert.True(t, canPull(priv, "u", roleUser, []string{"mh-acme-web-admin"}))
	assert.False(t, canPull(priv, "u", roleUser, []string{"mh-other"}))
	// public project + system: any authenticated user
	assert.True(t, canPull(pub, "u", roleUser, nil))
	assert.True(t, canPull(sys, "u", roleUser, nil))
}

func TestCanPush(t *testing.T) {
	personal := nsInfo{class: nsPersonal, owner: "alice"}
	proj := nsInfo{class: nsProject, org: "acme", project: "web"}
	sys := nsInfo{class: nsSystem}

	assert.True(t, canPush(personal, "alice", roleUser, nil))
	assert.False(t, canPush(personal, "bob", roleUser, nil))
	// project: only project-admin / org-admin / global-admin push
	assert.True(t, canPush(proj, "u", roleUser, []string{"mh-acme-web-admin"}))
	assert.True(t, canPush(proj, "u", roleUser, []string{"mh-acme-admin"}))
	assert.False(t, canPush(proj, "u", roleUser, []string{"mh-acme-web"})) // member can't push
	assert.False(t, canPush(proj, "u", roleUser, nil))
	// system: only global-admin
	assert.False(t, canPush(sys, "u", roleUser, nil))
	assert.True(t, canPush(sys, "u", roleAdmin, nil))
}

func TestClassifyNamespace(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["acme/web"] = types.Project{TeamSlug: "acme", ProjectSlug: "web", Visibility: types.VisibilityPublic}
	h := Handlers{mag: magazine.Magazine{Database: fake}}
	ctx := context.Background()

	assert.Equal(t, nsProject, h.classifyNamespace(ctx, "acme/web/img").class)
	assert.Equal(t, types.VisibilityPublic, h.classifyNamespace(ctx, "acme/web/img").visibility)
	assert.Equal(t, nsPersonal, h.classifyNamespace(ctx, "user/alice/app").class)
	assert.Equal(t, "alice", h.classifyNamespace(ctx, "user/alice/app").owner)
	assert.Equal(t, nsSystem, h.classifyNamespace(ctx, "unknown/x/y").class)
}

// gateReq builds a /v2 request with an injected OCIAuthContext.
func gateReq(method, path string, auth *OCIAuthContext) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if auth != nil {
		r = r.WithContext(contextWithOCIAuth(r.Context(), auth))
	}
	return r
}

func okNext() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestOCIReadGate(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["acme/web"] = types.Project{TeamSlug: "acme", ProjectSlug: "web", Visibility: types.VisibilityPrivate}
	base := Handlers{mag: magazine.Magazine{Database: fake}}

	// switch OFF: private project readable by anyone (legacy ungated reads)
	off := base
	w := httptest.NewRecorder()
	off.OCIReadAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("GET", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "bob", Role: roleUser}))
	assert.Equal(t, http.StatusOK, w.Code, "switch off => ungated")

	// switch ON
	on := base.WithRegistry(config.RegistryConfig{EnforcePullAuthz: true})
	// non-member of a private project -> 403
	w = httptest.NewRecorder()
	on.OCIReadAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("GET", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "bob", Role: roleUser}))
	assert.Equal(t, http.StatusForbidden, w.Code)
	// member -> 200
	w = httptest.NewRecorder()
	on.OCIReadAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("GET", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "bob", Role: roleUser, Groups: []string{"mh-acme-web"}}))
	assert.Equal(t, http.StatusOK, w.Code)
	// personal owner -> 200, other -> 403
	w = httptest.NewRecorder()
	on.OCIReadAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("GET", "/v2/user/alice/app/manifests/1",
		&OCIAuthContext{Username: "alice", Role: roleUser}))
	assert.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	on.OCIReadAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("GET", "/v2/user/alice/app/manifests/1",
		&OCIAuthContext{Username: "bob", Role: roleUser}))
	assert.Equal(t, http.StatusForbidden, w.Code)
	// a write method passes the read gate untouched
	w = httptest.NewRecorder()
	on.OCIReadAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("PUT", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "bob", Role: roleUser}))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOCIWriteGate_Ownership(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["acme/web"] = types.Project{TeamSlug: "acme", ProjectSlug: "web", Visibility: types.VisibilityPrivate}
	base := Handlers{mag: magazine.Magazine{Database: fake}}

	// switch OFF: admin-only writes (legacy)
	w := httptest.NewRecorder()
	base.OCIWriteAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("PUT", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "bob", Role: roleUser}))
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = httptest.NewRecorder()
	base.OCIWriteAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("PUT", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "admin", Role: roleAdmin}))
	assert.Equal(t, http.StatusOK, w.Code)

	// switch ON: ownership/role
	on := base.WithRegistry(config.RegistryConfig{EnforceWriteOwnership: true})
	// project-admin pushes
	w = httptest.NewRecorder()
	on.OCIWriteAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("PUT", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "u", Role: roleUser, Groups: []string{"mh-acme-web-admin"}}))
	assert.Equal(t, http.StatusOK, w.Code)
	// plain project member cannot push
	w = httptest.NewRecorder()
	on.OCIWriteAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("PUT", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "u", Role: roleUser, Groups: []string{"mh-acme-web"}}))
	assert.Equal(t, http.StatusForbidden, w.Code)
	// personal owner pushes; a GET is never gated by the write middleware
	w = httptest.NewRecorder()
	on.OCIWriteAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("PUT", "/v2/user/alice/app/manifests/1",
		&OCIAuthContext{Username: "alice", Role: roleUser}))
	assert.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	on.OCIWriteAuthMiddleware()(okNext()).ServeHTTP(w, gateReq("GET", "/v2/acme/web/manifests/latest",
		&OCIAuthContext{Username: "bob", Role: roleUser}))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEnforceManifestQuota_PersonalProvision(t *testing.T) {
	fake := newFakeProjDB()
	h := Handlers{mag: magazine.Magazine{Database: fake}}.
		WithRegistry(config.RegistryConfig{EnforceWriteOwnership: true, DefaultUserQuotaBytes: 5 << 30})
	ctx := context.Background()
	buf := manifestBuf(t) // config 10 + layer 100 = 110 bytes incoming
	req := types.Request{Repo: "user/alice/app"}

	// first push auto-provisions user/alice (5 GiB) and is well under quota
	require.NoError(t, h.enforceManifestQuota(ctx, req, buf))
	p, found, _ := fake.ProjectGet(ctx, types.PersonalTeamSlug, "alice")
	require.True(t, found, "personal namespace provisioned on first push")
	assert.Equal(t, types.KindPersonal, p.Kind)
	assert.Equal(t, int64(5<<30), p.QuotaBytes)

	// shrink the quota below incoming -> 413-class quotaExceededError
	fake.projects["user/alice"] = types.Project{TeamSlug: "user", ProjectSlug: "alice", QuotaBytes: 50, Kind: types.KindPersonal}
	fake.usage["user/alice"] = 0
	err := h.enforceManifestQuota(ctx, req, buf)
	require.Error(t, err)
	_, isQuota := err.(*quotaExceededError)
	assert.True(t, isQuota)

	// switch OFF: personal namespace is not provisioned/enforced (exempt)
	off := Handlers{mag: magazine.Magazine{Database: newFakeProjDB()}}
	assert.NoError(t, off.enforceManifestQuota(ctx, types.Request{Repo: "user/carol/x"}, buf))
}

func TestVisibilityAdminAPI(t *testing.T) {
	fake := newFakeProjDB()
	h := Handlers{mag: magazine.Magazine{Database: fake, Config: magazine.MagConfig{LogicalPoolBytes: 100000}}}

	// create with visibility=public
	w := httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", `{"team_slug":"acme","project_slug":"web","quota_bytes":500,"visibility":"public"}`, nil))
	require.Equal(t, http.StatusCreated, w.Code)
	var resp projectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, types.VisibilityPublic, resp.Visibility)

	// invalid visibility on create -> 400
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", `{"team_slug":"a","project_slug":"b","quota_bytes":1,"visibility":"bogus"}`, nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// PUT visibility -> 200 and round-trips
	params := map[string]string{"team": "acme", "project": "web"}
	w = httptest.NewRecorder()
	h.ProjectSetVisibilityHandler(w, adminReq("PUT", "/projects/acme/web/visibility", `{"visibility":"private"}`, params))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, types.VisibilityPrivate, resp.Visibility)

	// bad value -> 400
	w = httptest.NewRecorder()
	h.ProjectSetVisibilityHandler(w, adminReq("PUT", "/projects/acme/web/visibility", `{"visibility":"x"}`, params))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// missing project -> 404
	w = httptest.NewRecorder()
	h.ProjectSetVisibilityHandler(w, adminReq("PUT", "/projects/acme/none/visibility", `{"visibility":"public"}`, map[string]string{"team": "acme", "project": "none"}))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// non-admin -> 403
	w = httptest.NewRecorder()
	h.ProjectSetVisibilityHandler(w, nonAdminReq("PUT", "/projects/acme/web/visibility", `{"visibility":"public"}`))
	assert.Equal(t, http.StatusForbidden, w.Code)
}
