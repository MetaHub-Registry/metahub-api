package pullthrough

import (
	"context"

	"github.com/opencontainers/go-digest"
	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	mhtypes "github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
)

// MagazineStore adapts magazine.Magazine to the cache's Store seam: it stats
// MetaHub storage and streams upstream layers through metalib's constant-memory
// PullLayer path.
type MagazineStore struct {
	Mag magazine.Magazine
}

// Exists reports whether the digest's bytes are already in MetaHub storage. A
// stat failure is treated as "not stored" rather than an error: the download
// path surfaces any real failure, and the stat is only an optimisation.
func (s MagazineStore) Exists(ctx context.Context, dig string) (bool, error) {
	if _, err := s.Mag.Minio.BlobStatObj(ctx, dig); err != nil {
		return false, nil
	}
	return true, nil
}

// Cache streams the upstream layer for job into MetaHub storage and returns the
// stored size. job.Registry is the upstream registry's EffectiveURL (the value
// metalib expects in UpstreamTriple.Reg) and job.Repo is the prefix-stripped
// upstream repo path.
func (s MagazineStore) Cache(ctx context.Context, job Job) (int64, error) {
	up := upstream.UpstreamTriple{
		Reg:       job.Registry,
		Repo:      job.Repo,
		Tag:       job.Tag,
		Digest:    digest.Digest(job.Digest),
		MediaType: job.MediaType,
	}
	if err := s.Mag.PullLayer(ctx, up); err != nil {
		return 0, err
	}
	if obj, err := s.Mag.Minio.BlobStatObj(ctx, job.Digest); err == nil {
		return obj.Size, nil
	}
	return 0, nil
}

// MagazinePolicies adapts the metalib registry config to the cache's
// PolicyResolver seam.
type MagazinePolicies struct {
	Mag magazine.Magazine
}

// PolicyFor resolves the pull-through policy for a registry key, matching on
// either the EffectiveURL (network endpoint) or the slug alias. Eligibility
// mirrors metalib's pullThroughEligible: anonymous pull allowed AND opted in.
func (p MagazinePolicies) PolicyFor(registry string) (RegistryPolicy, bool) {
	for _, reg := range p.Mag.Config.Registries {
		if reg.EffectiveURL() != registry && reg.EffectiveName() != registry {
			continue
		}
		return RegistryPolicy{
			Enabled:         reg.AllowAnon && reg.PullThrough.Enabled,
			Cache:           reg.PullThrough.Cache,
			MaxCacheBytes:   reg.PullThrough.MaxCacheBytes,
			Workers:         reg.PullThrough.Workers,
			ClientRedirect:  reg.PullThrough.ClientRedirect,
			RefreshInterval: reg.PullThrough.RefreshInterval,
		}, true
	}
	return RegistryPolicy{}, false
}

// MaxWorkers returns the largest per-registry Workers override across enabled
// pull-through registries (0 when none is set), so the process-wide pool is
// sized to the busiest registry.
func (p MagazinePolicies) MaxWorkers() int {
	max := 0
	for _, reg := range p.Mag.Config.Registries {
		if !(reg.AllowAnon && reg.PullThrough.Enabled) {
			continue
		}
		if reg.PullThrough.Workers > max {
			max = reg.PullThrough.Workers
		}
	}
	return max
}

// MagazineRefresher adapts magazine.Magazine to the cache's Refresher seam: it
// re-discovers the upstream (repo, tag) via HeadManifestNotFound, which is
// idempotent, rewires the Slot when the upstream digest moved, and re-stamps
// last_refreshed. An unreachable upstream returns an error (the client was
// already served stale by the intercept).
type MagazineRefresher struct {
	Mag magazine.Magazine
}

// Refresh re-checks the upstream for job's (repo, tag).
func (r MagazineRefresher) Refresh(ctx context.Context, job RefreshJob) error {
	ctx, span := tracer.Start(ctx, "pullthrough.refresh.upstream")
	defer span.End()
	req := mhtypes.Request{Repo: job.Repo, Tag: job.Tag, Profile: job.Profile}
	if _, _, err := r.Mag.HeadManifestNotFound(ctx, req, job.Registry); err != nil {
		return err
	}
	return nil
}
