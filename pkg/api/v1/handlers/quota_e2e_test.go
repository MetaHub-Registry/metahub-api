package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// quota_e2e_test.go exercises the per-project storage-quota guard end-to-end
// against the REAL neo4j backend wired into the package-level `h` (see
// handlers_test.go TestMain) — real ProjectCreate/UpdateQuota/UsageBytes/
// BlobDigests/Delete + the real enforceManifestQuota. The guard runs before
// StoreManifestUpload, so no blob/profile bootstrap is needed: a project with a
// tiny quota rejects an incoming manifest larger than it.
func TestQuotaGuardE2E(t *testing.T) {
	ctx := context.Background()
	team, project := "e2equota", "p"
	defer h.mag.Database.ProjectDelete(ctx, team, project)

	// manifestBuf (from project_admin_test.go) = config 10B + layer 100B = 110B.
	buf := manifestBuf(t)
	req := types.Request{Repo: team + "/" + project + "/img"}

	// No :Project yet -> exempt (existing/system content behaviour).
	require.NoError(t, h.enforceManifestQuota(ctx, req, buf))

	// Create a project with a 50-byte quota: the 110B push exceeds it -> 413.
	_, err := h.mag.Database.ProjectCreate(ctx, team, project, 50)
	require.NoError(t, err)
	err = h.enforceManifestQuota(ctx, req, buf)
	require.Error(t, err)
	_, isQuota := err.(*quotaExceededError)
	assert.True(t, isQuota, "expected quotaExceededError, got %v", err)

	// Raise the quota above the incoming size -> now allowed.
	require.NoError(t, h.mag.Database.ProjectUpdateQuota(ctx, team, project, 1000))
	require.NoError(t, h.enforceManifestQuota(ctx, req, buf))

	// A repo without a two-segment project namespace is exempt.
	require.NoError(t, h.enforceManifestQuota(ctx, types.Request{Repo: "single"}, buf))

	// Usage is still 0 (nothing actually stored — the guard rejects before store).
	usage, err := h.mag.Database.ProjectUsageBytes(ctx, team, project)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage)
}
