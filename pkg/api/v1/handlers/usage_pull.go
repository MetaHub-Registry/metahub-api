package handlers

import (
	"net/http"

	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-api/pkg/usage"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// resolvePullIdentity returns the effective profile and user for a registry
// pull. The request profile wins; when absent it falls back to the profile on
// the authenticated OCI context. The user is the authenticated username (empty
// for anonymous pulls).
func resolvePullIdentity(r *http.Request, reqProfile string) (profile, user string) {
	profile = reqProfile
	if auth, ok := OCIUserFromContext(r.Context()); ok {
		user = auth.Username
		if profile == "" {
			profile = auth.Profile
		}
	}
	return profile, user
}

// recordPull emits the bounded pull metric plus the detailed VictoriaLogs usage
// event (which image, profile, user) for a completed registry pull. For
// manifest pulls it also records the profile-aware resolution metric. It reads
// the final status from the ResponseWriter, so it is meant to be deferred.
func recordPull(w http.ResponseWriter, r *http.Request, req types.Request, kind string) {
	st := metrics.StatusOf(w)
	profile, user := resolvePullIdentity(r, req.Profile)
	metrics.ObservePull(kind, profile, st)
	if kind == metrics.KindManifest {
		metrics.ObserveManifestResolution(profile, metrics.ResultFromStatus(st))
	}
	usage.RegistryPull(usage.PullEvent{
		Kind: kind, Repo: req.Repo, Tag: req.Tag, Digest: req.Digest,
		Profile: profile, User: user, ClientIP: clientIP(r), Status: st,
	})
}
