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
// owns the two-segment namespace "<team>/<project>". Usage is the sum of DISTINCT
// blob sizes reachable from the project's manifests; a push is rejected (413)
// when it would take that logical usage above the project's quota. Namespaces
// with no :Project node are exempt (existing / system content).

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

// incomingBlobs parses an image manifest and returns its config+layer blob
// digests -> size. Returns an empty map for non-image-manifest payloads (e.g. an
// index), whose sub-manifests are guarded when pushed.
func incomingBlobs(buf []byte) map[string]int64 {
	var m v1.Manifest
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil
	}
	out := map[string]int64{}
	if m.Config.Digest != "" {
		out[m.Config.Digest.String()] = m.Config.Size
	}
	for _, layer := range m.Layers {
		out[layer.Digest.String()] = layer.Size
	}
	return out
}

// newBytesDelta returns the total size of incoming blobs NOT already counted for
// the project (dedup within the project by digest); sizes < 0 are ignored.
func newBytesDelta(incoming map[string]int64, existing []string) int64 {
	have := make(map[string]struct{}, len(existing))
	for _, d := range existing {
		have[d] = struct{}{}
	}
	var delta int64
	for dig, size := range incoming {
		if _, seen := have[dig]; seen {
			continue
		}
		if size > 0 {
			delta += size
		}
	}
	return delta
}

// enforceManifestQuota is the push guard: it rejects (returns *quotaExceededError)
// when the incoming manifest's not-yet-counted blobs would take the project's
// logical usage above its quota. Returns nil when allowed, or exempt (no project
// for the namespace, or a non-image-manifest payload).
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
		// Personal namespace (user/<username>): the first push auto-provisions it
		// with the default quota, then it is enforced like any project. Only under
		// write-ownership enforcement; otherwise system/unowned namespaces stay
		// exempt (legacy behavior).
		if h.registry.EnforceWriteOwnership && team == types.PersonalTeamSlug && project != "" {
			proj, err = h.mag.Database.PersonalNamespaceGetOrCreate(ctx, project, h.personalQuotaBytes())
			if err != nil {
				return err
			}
		} else {
			return nil // system/unowned -> exempt
		}
	}
	incoming := incomingBlobs(buf)
	if len(incoming) == 0 {
		return nil
	}
	existing, err := h.mag.Database.ProjectBlobDigests(ctx, team, project)
	if err != nil {
		return err
	}
	delta := newBytesDelta(incoming, existing)
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
	writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
		"errors": []map[string]string{{"code": "DENIED", "message": qe.Error()}},
	})
}
