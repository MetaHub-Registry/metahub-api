package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// quota.go implements the logical per-project storage-quota guard enforced at
// manifest link (openspec change add-registry-project-quotas, task 3). A project
// owns the two-segment namespace "<team>/<project>" (the leading two repo path
// segments). Usage is the sum of DISTINCT blob sizes reachable from the project's
// manifests; a push is rejected (413) when it would take that logical usage above
// the project's quota. Namespaces with no :Project node are exempt (existing /
// system content).

// projectFromRepo returns (team, project, ok) from a repo path's first two
// segments. ok is false when the repo has fewer than two non-empty segments.
func projectFromRepo(repo string) (team, project string, ok bool) {
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// quotaExceededError signals the push would take the project over its quota.
type quotaExceededError struct {
	Team, Project       string
	Usage, Delta, Quota int64
}

func (e *quotaExceededError) Error() string {
	return fmt.Sprintf("project %s/%s storage quota exceeded: usage %d + incoming %d > quota %d bytes",
		e.Team, e.Project, e.Usage, e.Delta, e.Quota)
}

// enforceManifestQuota is the push guard. It parses the incoming image manifest,
// determines the incoming blobs NOT already counted for the project (dedup within
// the project), and returns a *quotaExceededError when usage + that delta exceeds
// the quota. Returns nil when allowed, or exempt (no project for the namespace,
// or a non-image-manifest payload such as an index).
func (h Handlers) enforceManifestQuota(ctx context.Context, req types.Request, buf []byte) error {
	team, project, ok := projectFromRepo(req.Repo)
	if !ok {
		return nil
	}
	proj, found, err := h.mag.Database.ProjectGet(ctx, team, project)
	if err != nil {
		return err
	}
	if !found {
		return nil // no :Project node -> exempt
	}

	// Collect the incoming config + layer descriptors. Index/list payloads carry
	// no direct blobs; their sub-manifests are guarded when pushed.
	var m v1.Manifest
	if e := json.Unmarshal(buf, &m); e != nil {
		return nil
	}
	incoming := map[string]int64{}
	if m.Config.Digest != "" {
		incoming[m.Config.Digest.String()] = m.Config.Size
	}
	for _, layer := range m.Layers {
		incoming[layer.Digest.String()] = layer.Size
	}
	if len(incoming) == 0 {
		return nil
	}

	digs, err := h.mag.Database.ProjectBlobDigests(ctx, team, project)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(digs))
	for _, d := range digs {
		existing[d] = struct{}{}
	}

	var delta int64
	for dig, size := range incoming {
		if _, seen := existing[dig]; seen {
			continue // already counted for this project
		}
		if size > 0 {
			delta += size
		}
	}
	if delta == 0 {
		return nil // everything referenced is already counted -> free
	}

	usage, err := h.mag.Database.ProjectUsageBytes(ctx, team, project)
	if err != nil {
		return err
	}
	if usage+delta > proj.QuotaBytes {
		return &quotaExceededError{Team: team, Project: project, Usage: usage, Delta: delta, Quota: proj.QuotaBytes}
	}
	return nil
}

// writeQuota413 emits an OCI-style DENIED error with HTTP 413.
func writeQuota413(w http.ResponseWriter, qe *quotaExceededError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]string{{"code": "DENIED", "message": qe.Error()}},
	})
}
