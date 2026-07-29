package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// nonAdminReq builds a request whose context carries a non-admin identity, so
// requireAdmin() returns 403.
func nonAdminReq(method, target, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	ctx := contextWithClaims(r.Context(), &OIDCClaims{Groups: []string{"some-other-group"}})
	return r.WithContext(ctx)
}

func TestProjectListHandler(t *testing.T) {
	fake := newFakeProjDB()
	h := Handlers{mag: magazine.Magazine{Database: fake}}

	// non-admin -> 403
	w := httptest.NewRecorder()
	h.ProjectListHandler(w, nonAdminReq("GET", "/projects", ""))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// admin, empty -> 200 []
	w = httptest.NewRecorder()
	h.ProjectListHandler(w, adminReq("GET", "/projects", "", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var empty []projectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &empty))
	assert.Empty(t, empty)

	// admin, two projects -> 200 with both
	fake.projects["a/b"] = types.Project{TeamSlug: "a", ProjectSlug: "b", QuotaBytes: 10}
	fake.projects["c/d"] = types.Project{TeamSlug: "c", ProjectSlug: "d", QuotaBytes: 20}
	fake.usage["a/b"] = 5
	w = httptest.NewRecorder()
	h.ProjectListHandler(w, adminReq("GET", "/projects", "", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var got []projectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestProjectUsageHandler(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["t/p"] = types.Project{TeamSlug: "t", ProjectSlug: "p", QuotaBytes: 900}
	fake.usage["t/p"] = 300
	h := Handlers{mag: magazine.Magazine{Database: fake}}
	params := map[string]string{"team": "t", "project": "p"}

	// non-admin -> 403
	w := httptest.NewRecorder()
	h.ProjectUsageHandler(w, nonAdminReq("GET", "/projects/t/p/usage", ""))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// missing project -> 404
	w = httptest.NewRecorder()
	h.ProjectUsageHandler(w, adminReq("GET", "/projects/t/none/usage", "", map[string]string{"team": "t", "project": "none"}))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// admin, existing -> 200 with usage + quota
	w = httptest.NewRecorder()
	h.ProjectUsageHandler(w, adminReq("GET", "/projects/t/p/usage", "", params))
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]int64
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(300), resp["usage_bytes"])
	assert.Equal(t, int64(900), resp["quota_bytes"])

	// admin, ?by=user -> 200 with the per-pusher split + quota
	fake.userUsage["t/p"] = map[string]float64{"alice": 250, "bob": 250}
	w = httptest.NewRecorder()
	h.ProjectUsageHandler(w, adminReq("GET", "/projects/t/p/usage?by=user", "", params))
	require.Equal(t, http.StatusOK, w.Code)
	var uresp struct {
		Usages     map[string]float64 `json:"usages"`
		QuotaBytes int64              `json:"quota_bytes"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uresp))
	assert.Equal(t, 250.0, uresp.Usages["alice"])
	assert.Equal(t, 250.0, uresp.Usages["bob"])
	assert.Equal(t, int64(900), uresp.QuotaBytes)
}

func TestProjectCreateHandler_Validation(t *testing.T) {
	fake := newFakeProjDB()
	h := Handlers{mag: magazine.Magazine{Database: fake, Config: magazine.MagConfig{LogicalPoolBytes: 100000}}}

	// malformed JSON -> 400
	w := httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", `{not json`, nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// missing required fields -> 400
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", `{"team_slug":"t"}`, nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// negative quota -> 400
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", `{"team_slug":"t","project_slug":"p","quota_bytes":-1}`, nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// idempotent: create then re-create returns 200 (existing) not 201
	body := `{"team_slug":"t","project_slug":"p","quota_bytes":500}`
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", body, nil))
	require.Equal(t, http.StatusCreated, w.Code)
	w = httptest.NewRecorder()
	h.ProjectCreateHandler(w, adminReq("POST", "/projects", body, nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProjectQuotaUpdateHandler_Validation(t *testing.T) {
	fake := newFakeProjDB()
	fake.projects["t/p"] = types.Project{TeamSlug: "t", ProjectSlug: "p", QuotaBytes: 400}
	h := Handlers{mag: magazine.Magazine{Database: fake, Config: magazine.MagConfig{LogicalPoolBytes: 100000}}}
	params := map[string]string{"team": "t", "project": "p"}

	// non-admin -> 403
	w := httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, nonAdminReq("PUT", "/projects/t/p/quota", `{"quota_bytes":500}`))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// malformed body -> 400
	w = httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, adminReq("PUT", "/projects/t/p/quota", `{bad`, params))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// negative quota -> 400
	w = httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, adminReq("PUT", "/projects/t/p/quota", `{"quota_bytes":-5}`, params))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// not found -> 404
	w = httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, adminReq("PUT", "/projects/t/none/quota", `{"quota_bytes":10}`, map[string]string{"team": "t", "project": "none"}))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// lower quota within pool -> 200 (no pool check on shrink)
	w = httptest.NewRecorder()
	h.ProjectQuotaUpdateHandler(w, adminReq("PUT", "/projects/t/p/quota", `{"quota_bytes":100}`, params))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProjectGetHandler_NonAdmin(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{Database: newFakeProjDB()}}
	w := httptest.NewRecorder()
	h.ProjectGetHandler(w, nonAdminReq("GET", "/projects/t/p", ""))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestProjectDeleteHandler_NonAdmin(t *testing.T) {
	h := Handlers{mag: magazine.Magazine{Database: newFakeProjDB()}}
	w := httptest.NewRecorder()
	h.ProjectDeleteHandler(w, nonAdminReq("DELETE", "/projects/t/p", ""))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// adminFromContext returns false when no claims are present at all.
func TestAdminFromContext_NoClaims(t *testing.T) {
	assert.False(t, adminFromContext(context.Background()))
}
