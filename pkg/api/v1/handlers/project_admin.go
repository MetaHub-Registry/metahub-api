package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// project_admin.go implements the admin API for registry projects + storage
// quotas (openspec change add-registry-project-quotas, tasks 4 & 5): create /
// list / get / delete a project, set its quota, and read its usage. All handlers
// are admin-role-gated. Quota grants are checked against the no-overcommit
// logical pool budget under a process-level mutex.

// poolGrantMu serializes quota grants so Sigma(project quota) cannot overshoot the
// logical pool budget under concurrent create/update. Process-level — adequate
// for a single-replica deployment; multi-replica would need a DB-level lock.
var poolGrantMu sync.Mutex

const (
	msgProjectNotFound  = "project not found"
	msgFailedGetProject = "failed to get project: %v"
)

type projectCreateRequest struct {
	TeamSlug    string `json:"team_slug"`
	ProjectSlug string `json:"project_slug"`
	QuotaBytes  int64  `json:"quota_bytes"`
}

type quotaUpdateRequest struct {
	QuotaBytes int64 `json:"quota_bytes"`
}

type projectResponse struct {
	TeamSlug    string `json:"team_slug"`
	ProjectSlug string `json:"project_slug"`
	Namespace   string `json:"namespace"`
	QuotaBytes  int64  `json:"quota_bytes"`
	UsageBytes  int64  `json:"usage_bytes"`
	CreatedAt   int64  `json:"created_at"`
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// adminFromContext reports whether the authenticated caller has the admin role
// (Authentik group membership resolved by roleFromGroups).
func adminFromContext(ctx context.Context) bool {
	claims, ok := OIDCClaimsFromContext(ctx)
	if !ok {
		return false
	}
	return roleFromGroups(claims.Groups) == roleAdmin
}

func (h Handlers) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if adminFromContext(r.Context()) {
		return true
	}
	writeJSONError(w, http.StatusForbidden, "admin role required")
	return false
}

// checkPoolCapacity enforces the no-overcommit rule: Sigma(quota) + addBytes must
// fit the configured logical pool budget. A budget of 0 means unlimited. Callers
// hold poolGrantMu.
func (h Handlers) checkPoolCapacity(ctx context.Context, addBytes int64) error {
	pool := h.mag.Config.LogicalPoolBytes
	if pool <= 0 {
		return nil
	}
	sum, err := h.mag.Database.ProjectQuotaSum(ctx)
	if err != nil {
		return err
	}
	if sum+addBytes > pool {
		return fmt.Errorf("logical pool budget exceeded: sum(quota) %d + %d > pool %d bytes", sum, addBytes, pool)
	}
	return nil
}

// projectResp builds the API response for a project, including its live usage.
func (h Handlers) projectResp(ctx context.Context, p types.Project) projectResponse {
	usage, _ := h.mag.Database.ProjectUsageBytes(ctx, p.TeamSlug, p.ProjectSlug)
	return projectResponse{
		TeamSlug:    p.TeamSlug,
		ProjectSlug: p.ProjectSlug,
		Namespace:   p.Namespace(),
		QuotaBytes:  p.QuotaBytes,
		UsageBytes:  usage,
		CreatedAt:   p.CreatedAt,
	}
}

// getProjectParam resolves {team}/{project} from the URL and fetches the project,
// writing 500/404 on failure. ok is false when the caller should stop.
func (h Handlers) getProjectParam(w http.ResponseWriter, r *http.Request) (types.Project, bool) {
	p, found, err := h.mag.Database.ProjectGet(r.Context(), chi.URLParam(r, "team"), chi.URLParam(r, "project"))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf(msgFailedGetProject, err))
		return types.Project{}, false
	}
	if !found {
		writeJSONError(w, http.StatusNotFound, msgProjectNotFound)
		return types.Project{}, false
	}
	return p, true
}

// ProjectCreateHandler handles POST /api/v1/protected/projects.
func (h Handlers) ProjectCreateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}
	var req projectCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TeamSlug == "" || req.ProjectSlug == "" {
		writeJSONError(w, http.StatusBadRequest, "team_slug and project_slug are required")
		return
	}
	if req.QuotaBytes < 0 {
		writeJSONError(w, http.StatusBadRequest, "quota_bytes must be >= 0")
		return
	}
	poolGrantMu.Lock()
	defer poolGrantMu.Unlock()
	// Idempotent create: an existing project is returned unchanged (no new
	// allocation, so no pool impact).
	if existing, found, err := h.mag.Database.ProjectGet(ctx, req.TeamSlug, req.ProjectSlug); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf(msgFailedGetProject, err))
		return
	} else if found {
		writeJSON(w, http.StatusOK, h.projectResp(ctx, existing))
		return
	}
	if err := h.checkPoolCapacity(ctx, req.QuotaBytes); err != nil {
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	p, err := h.mag.Database.ProjectCreate(ctx, req.TeamSlug, req.ProjectSlug, req.QuotaBytes)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create project: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, h.projectResp(ctx, p))
}

// ProjectListHandler handles GET /api/v1/protected/projects.
func (h Handlers) ProjectListHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}
	ps, err := h.mag.Database.ProjectList(ctx)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list projects: %v", err))
		return
	}
	out := make([]projectResponse, 0, len(ps))
	for _, p := range ps {
		out = append(out, h.projectResp(ctx, p))
	}
	writeJSON(w, http.StatusOK, out)
}

// ProjectGetHandler handles GET /api/v1/protected/projects/{team}/{project}.
func (h Handlers) ProjectGetHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	p, ok := h.getProjectParam(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.projectResp(r.Context(), p))
}

// ProjectUsageHandler handles GET /api/v1/protected/projects/{team}/{project}/usage.
func (h Handlers) ProjectUsageHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	p, ok := h.getProjectParam(w, r)
	if !ok {
		return
	}
	usage, err := h.mag.Database.ProjectUsageBytes(r.Context(), p.TeamSlug, p.ProjectSlug)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to compute usage: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"usage_bytes": usage, "quota_bytes": p.QuotaBytes})
}

// ProjectQuotaUpdateHandler handles PUT /api/v1/protected/projects/{team}/{project}/quota.
func (h Handlers) ProjectQuotaUpdateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.requireAdmin(w, r) {
		return
	}
	var req quotaUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.QuotaBytes < 0 {
		writeJSONError(w, http.StatusBadRequest, "quota_bytes must be >= 0")
		return
	}
	poolGrantMu.Lock()
	defer poolGrantMu.Unlock()
	cur, ok := h.getProjectParam(w, r)
	if !ok {
		return
	}
	if delta := req.QuotaBytes - cur.QuotaBytes; delta > 0 {
		if err := h.checkPoolCapacity(ctx, delta); err != nil {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
	}
	if err := h.mag.Database.ProjectUpdateQuota(ctx, cur.TeamSlug, cur.ProjectSlug, req.QuotaBytes); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update quota: %v", err))
		return
	}
	cur.QuotaBytes = req.QuotaBytes
	writeJSON(w, http.StatusOK, h.projectResp(ctx, cur))
}

// ProjectDeleteHandler handles DELETE /api/v1/protected/projects/{team}/{project}.
func (h Handlers) ProjectDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	found, err := h.mag.Database.ProjectDelete(r.Context(), chi.URLParam(r, "team"), chi.URLParam(r, "project"))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete project: %v", err))
		return
	}
	if !found {
		writeJSONError(w, http.StatusNotFound, msgProjectNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
