package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// fakeProjDB implements only the Project* methods of magazine.Database (the rest
// are promoted from the embedded nil interface and panic if called — the tests
// never touch them).
type fakeProjDB struct {
	magazine.Database
	projects  map[string]types.Project
	usage     map[string]int64
	digests   map[string][]string
	userUsage map[string]map[string]float64
}

func newFakeProjDB() *fakeProjDB {
	return &fakeProjDB{
		projects:  map[string]types.Project{},
		usage:     map[string]int64{},
		digests:   map[string][]string{},
		userUsage: map[string]map[string]float64{},
	}
}

func pkey(team, project string) string { return team + "/" + project }

func (f *fakeProjDB) ProjectCreate(_ context.Context, team, project string, quota int64) (types.Project, error) {
	p := types.Project{TeamSlug: team, ProjectSlug: project, QuotaBytes: quota, CreatedAt: 1,
		Visibility: types.VisibilityPrivate, Kind: types.KindProject}
	f.projects[pkey(team, project)] = p
	return p, nil
}
func (f *fakeProjDB) ProjectGet(_ context.Context, team, project string) (types.Project, bool, error) {
	p, ok := f.projects[pkey(team, project)]
	return p, ok, nil
}
func (f *fakeProjDB) ProjectUpdateQuota(_ context.Context, team, project string, quota int64) error {
	p := f.projects[pkey(team, project)]
	p.QuotaBytes = quota
	f.projects[pkey(team, project)] = p
	return nil
}
func (f *fakeProjDB) ProjectDelete(_ context.Context, team, project string) (bool, error) {
	k := pkey(team, project)
	_, ok := f.projects[k]
	delete(f.projects, k)
	return ok, nil
}
func (f *fakeProjDB) ProjectList(_ context.Context) ([]types.Project, error) {
	out := make([]types.Project, 0, len(f.projects))
	for _, p := range f.projects {
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeProjDB) ProjectQuotaSum(_ context.Context) (int64, error) {
	var s int64
	for _, p := range f.projects {
		s += p.QuotaBytes
	}
	return s, nil
}
func (f *fakeProjDB) ProjectUsageBytes(_ context.Context, team, project string) (int64, error) {
	return f.usage[pkey(team, project)], nil
}
func (f *fakeProjDB) ProjectUserUsageBytes(_ context.Context, team, project string) (map[string]float64, error) {
	return f.userUsage[pkey(team, project)], nil
}
func (f *fakeProjDB) ProjectBlobDigests(_ context.Context, team, project string) ([]string, error) {
	return f.digests[pkey(team, project)], nil
}
func (f *fakeProjDB) BlobRefCount(_ context.Context, _ string) (int64, error) { return 0, nil }
func (f *fakeProjDB) ProjectSetVisibility(_ context.Context, team, project, vis string) (bool, error) {
	k := pkey(team, project)
	p, ok := f.projects[k]
	if !ok {
		return false, nil
	}
	p.Visibility = vis
	f.projects[k] = p
	return true, nil
}
func (f *fakeProjDB) PersonalNamespaceGetOrCreate(_ context.Context, username string, q int64) (types.Project, error) {
	k := pkey(types.PersonalTeamSlug, username)
	if p, ok := f.projects[k]; ok {
		return p, nil
	}
	p := types.Project{TeamSlug: types.PersonalTeamSlug, ProjectSlug: username, QuotaBytes: q,
		Kind: types.KindPersonal, Owner: username, Visibility: types.VisibilityPrivate}
	f.projects[k] = p
	return p, nil
}

func adminCtx(ctx context.Context) context.Context {
	return contextWithClaims(ctx, &OIDCClaims{Groups: []string{groupMetahubAdmin}})
}

func adminReq(method, target, body string, params map[string]string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	ctx := adminCtx(r.Context())
	if params != nil {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}
	return r.WithContext(ctx)
}

func TestProjectFromRepo(t *testing.T) {
	cases := []struct {
		repo             string
		team, project    string
		ok               bool
	}{
		{"acme/web/img", "acme", "web", true},
		{"acme/web", "acme", "web", true},
		{"single", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		team, project, ok := projectFromRepo(c.repo)
		assert.Equal(t, c.ok, ok, c.repo)
		assert.Equal(t, c.team, team, c.repo)
		assert.Equal(t, c.project, project, c.repo)
	}
}

func manifestBuf(t *testing.T) []byte {
	m := v1.Manifest{
		Config: v1.Descriptor{Digest: "sha256:cfg", Size: 10},
		Layers: []v1.Descriptor{{Digest: "sha256:lyr", Size: 100}},
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

func TestEnforceManifestQuota(t *testing.T) {
	fake := newFakeProjDB()
	h := Handlers{mag: magazine.Magazine{Database: fake}}
	ctx := context.Background()
	buf := manifestBuf(t)
	req := types.Request{Repo: "team/proj/img"}

	// exempt: no :Project for the namespace
	require.NoError(t, h.enforceManifestQuota(ctx, req, buf))

	// project exists, well under quota -> allowed
	fake.projects["team/proj"] = types.Project{TeamSlug: "team", ProjectSlug: "proj", QuotaBytes: 1000}
	require.NoError(t, h.enforceManifestQuota(ctx, req, buf))

	// over quota: usage 950 + incoming 110 > 1000 -> quotaExceededError
	fake.usage["team/proj"] = 950
	err := h.enforceManifestQuota(ctx, req, buf)
	require.Error(t, err)
	_, isQuota := err.(*quotaExceededError)
	assert.True(t, isQuota)

	// dedup: both incoming blobs already counted for the project -> delta 0 -> allowed
	fake.digests["team/proj"] = []string{"sha256:cfg", "sha256:lyr"}
	require.NoError(t, h.enforceManifestQuota(ctx, req, buf))
}

func TestProjectCreateHandler_AdminGateAndPoolCap(t *testing.T) {
	fake := newFakeProjDB()
	h := Handlers{mag: magazine.Magazine{Database: fake, Config: magazine.MagConfig{LogicalPoolBytes: 1000}}}
	body := `{"team_slug":"t","project_slug":"p","quota_bytes":500}`

	// non-admin -> 403
	r := httptest.NewRequest("POST", "/projects", strings.NewReader(body)).
		WithContext(contextWithClaims(context.Background(), &OIDCClaims{Groups: []string{"other"}}))
	w := httptest.NewRecorder()
	h.ProjectCreateHandler(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// admin, within pool -> 201
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", body, nil))
	assert.Equal(t, http.StatusCreated, w.Code)

	// admin, over logical pool (existing 500 + 600 > 1000) -> 409
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", `{"team_slug":"t2","project_slug":"p","quota_bytes":600}`, nil))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestProjectGetAndDeleteHandlers(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["t/p"] = types.Project{TeamSlug: "t", ProjectSlug: "p", QuotaBytes: 500}
	fake.usage["t/p"] = 123
	h := Handlers{mag: magazine.Magazine{Database: fake}}

	// GET existing -> 200 with usage
	w := httptest.NewRecorder()
	h.ProjectGetHandler(w, adminReq("GET", "/projects/t/p", "", map[string]string{"team": "t", "project": "p"}))
	require.Equal(t, http.StatusOK, w.Code)
	var resp projectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "t/p", resp.Namespace)
	assert.Equal(t, int64(123), resp.UsageBytes)

	// GET missing -> 404
	w = httptest.NewRecorder()
	h.ProjectGetHandler(w, adminReq("GET", "/projects/t/none", "", map[string]string{"team": "t", "project": "none"}))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// DELETE existing -> 204
	w = httptest.NewRecorder()
	h.ProjectDeleteHandler(w, adminReq("DELETE", "/projects/t/p", "", map[string]string{"team": "t", "project": "p"}))
	assert.Equal(t, http.StatusNoContent, w.Code)

	// DELETE again -> 404
	w = httptest.NewRecorder()
	h.ProjectDeleteHandler(w, adminReq("DELETE", "/projects/t/p", "", map[string]string{"team": "t", "project": "p"}))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProjectQuotaUpdateHandler_PoolCap(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["t/p"] = types.Project{TeamSlug: "t", ProjectSlug: "p", QuotaBytes: 400}
	h := Handlers{mag: magazine.Magazine{Database: fake, Config: magazine.MagConfig{LogicalPoolBytes: 1000}}}
	params := map[string]string{"team": "t", "project": "p"}

	// raise within pool (400 -> 900, sum 900 <= 1000) -> 200
	w := httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, adminReq("PUT", "/projects/t/p/quota", `{"quota_bytes":900}`, params))
	assert.Equal(t, http.StatusOK, w.Code)

	// raise beyond pool (900 -> 1200 > 1000) -> 409
	w = httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, adminReq("PUT", "/projects/t/p/quota", `{"quota_bytes":1200}`, params))
	assert.Equal(t, http.StatusConflict, w.Code)
}
