package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/metahub-registry/metahub-core/pkg/backends/magazine"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameOciIndex = "api/handler/oci_index.go"
	// genericProfile is the slot key for the un-manipulated/generic index. The
	// core slot code uses the same sentinel because an empty triple component
	// is rejected.
	genericProfile = "generic"
)

// isModelServingRepo reports whether the repo may carry the model-serving stack
// under a profile: model artifacts (models/...) and app/runtime repos (apps/...).
// Plain collections (e.g. qnib/ftest) stay on the generic index, which is the
// gate the e2e checks.
func isModelServingRepo(repo string) bool {
	return strings.HasPrefix(repo, "models/") || strings.HasPrefix(repo, "apps/")
}

// profileForIndex returns the profile an index pull is served under. The
// ManifestHandler rewrites a model-centric pull (models/<name>:<tag>) to its
// runtime repo before dispatching, so keying only on req.Repo would demote
// every model-centric index pull to generic. The requested-model context is
// set by that rewrite and is the reliable model-centric signal.
//
// modelServing marks profiles that carry a hardware.gpu block. Only those are
// gated: a collection pull under a model-serving profile falls back to generic
// (so model/config layers never leak), while a collection under an ordinary
// profile keeps it (e.g. the glow overlay).
func profileForIndex(ctx context.Context, req types.Request, modelServing bool) string {
	if req.Profile == "" {
		return genericProfile
	}
	if modelServing && types.RequestedModelFromContext(ctx) == "" && !isModelServingRepo(req.Repo) {
		return genericProfile
	}
	return req.Profile
}

// profileIsModelServing reports whether label names a stored model-serving
// profile (a profile with a hardware.gpu block). Unknown labels, the generic
// label and lookup errors are all treated as ordinary profiles.
func profileIsModelServing(ctx context.Context, db magazine.Database, label string) bool {
	if label == "" || label == genericProfile {
		return false
	}
	prf, found, err := db.ProfileGetByLabel(ctx, label)
	if err != nil || !found {
		return false
	}
	return prf.Hardware.IsModelServing()
}

// storedProfileOrGeneric maps a token profile onto a stored profile: unknown
// labels (e.g. an ephemeral agent profile such as oci/e2e/26) and deleted
// run-scoped profiles fall back to generic, and a bare tag (docker login may
// send only the tag portion) resolves to its label. This keeps HEAD/pull and
// push on the same profile instead of failing on a stale inventory entry.
func storedProfileOrGeneric(ctx context.Context, db magazine.Database, label string) string {
	if label == "" || label == genericProfile {
		return label
	}
	if _, found, err := db.ProfileGetByLabel(ctx, label); err == nil && found {
		return label
	}
	if prfs, err := db.ProfileGetAll(ctx); err == nil {
		for _, candidate := range prfs {
			if candidate.Tag == label {
				return candidate.Label
			}
		}
	}
	return genericProfile
}

// shouldApplySingleIndex reports whether a single-descriptor index may carry
// the profile's manipulations. Only model-serving profiles are restricted to
// model-centric pulls; ordinary profiles (glow and friends) apply on
// collections too.
func shouldApplySingleIndex(ctx context.Context, req types.Request, prf types.Profile) bool {
	if req.Profile == "" || req.Profile == genericProfile {
		return false
	}
	if prf.Hardware.IsModelServing() &&
		types.RequestedModelFromContext(ctx) == "" &&
		!isModelServingRepo(req.Repo) {
		return false
	}
	return true
}

// persistedIndexDigests remembers the manipulated-index digests this process has
// already stored in object storage, so a repeat profile-scoped pull skips the
// remote (~100-250ms against eu-west-1 S3) ManifestsPutObj on the GET path — the
// write-on-read that dominated the pull critical path. The index is
// content-addressed and immutable, so a cached digest can never serve wrong
// bytes; the worst case after a pod restart or on another replica is one
// redundant re-PUT. Cardinality is bounded by distinct manipulated indexes
// actually pulled (repo × tag × profile).
var persistedIndexDigests sync.Map // digest string -> struct{}

// handleIndex handles pull-side requests for OCI image indexes (formerly
// handleManifestList). It calls IndexGetByRequest which returns a
// fully-hydrated types.Index with the Manifests slice populated.
func (h Handlers) handleIndex(ctx context.Context, w http.ResponseWriter, req types.Request) (ok bool) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciIndex,
		"func":    "handleIndex",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
		"profile": req.Profile,
	})
	defer span.End()
	// Model-serving profiles apply only to model-centric pulls. Collection
	// pulls under such a profile must stay on the generic index; ordinary
	// profiles (e.g. glow) keep their non-model manipulation. profileForIndex
	// keeps the profile for a model-centric pull even though ManifestHandler
	// has already rewritten its repo to the runtime collection.
	req.Profile = profileForIndex(ctx, req, profileIsModelServing(ctx, h.mag.Database, req.Profile))
	ml, found, err := h.mag.IndexGetByRequest(ctx, req)
	if err != nil {
		l.Errorf("failed to fetch Index: %v", err)
		writeOCIError(w, fmt.Errorf("failed to fetch Index: %w", err))
		return true
	}
	if !found {
		return false
	}
	// Lazy manipulation: if the pulling profile has manipulations, apply them to
	// the Index's manifests on-demand (cached in the graph + MinIO for next pulls).
	if h.manipulateIndexInPlace(ctx, l, span, w, req, &ml) {
		return true
	}
	switch req.Method {
	case http.MethodGet:
		ml.WriteWWW(w)
	case http.MethodHead:
		ml.WriteWWWHeader(w)
	default:
		l.Errorf("unsupported method '%s' for index", strings.ToUpper(req.Method))
		writeOCIError(w, fmt.Errorf("unsupported method '%s' for index", strings.ToUpper(req.Method)))
	}
	return true
}

// manipulateIndexInPlace applies pull-time manipulations to the index's manifests
// (when the profile has any) and persists the manipulated Index in MinIO so a
// follow-up GET-by-digest finds it (the HEAD digest is otherwise transient).
// Returns true only if it wrote an OCI error (infeasible model quant → reject);
// other manipulation errors are non-fatal (serve the original).
func (h Handlers) manipulateIndexInPlace(ctx context.Context, l *logrus.Entry, span trace.Span, w http.ResponseWriter, req types.Request, ml *types.Index) bool {
	if len(ml.Manifests) == 0 || req.Profile == "" {
		return false
	}
	prf, prfFound, prfErr := h.mag.Database.ProfileGetByLabel(ctx, req.Profile)
	if prfErr != nil {
		utils.Warn(l, span, "failed to fetch profile '%s' for manipulation check: %v", req.Profile, prfErr)
		return false
	}
	if !prfFound {
		return false
	}
	manipulated, manipErr := h.mag.ApplyManipulationsToManifests(ctx, prf, ml.Manifests)
	if manipErr != nil {
		if isQuantInfeasible(manipErr) {
			writeOCIError(w, manipErr)
			return true
		}
		utils.Warn(l, span, "lazy manipulation failed for profile '%s' (serving original): %v", req.Profile, manipErr)
		return false
	}
	ml.Manifests = manipulated
	indexBytes := ml.ToOCIBytes()
	indexDigest := digest.FromBytes(indexBytes).String()
	// Write-on-read guard: only persist a given manipulated index once per
	// process. Skipping the re-PUT on every subsequent pull removes the S3 round
	// trip (~100-250ms) that dominated the manifest critical path. Record the
	// digest only AFTER a successful PUT, so we never skip storing an object that
	// isn't actually persisted yet.
	if _, stored := persistedIndexDigests.Load(indexDigest); stored {
		utils.Debug(l, span, "manipulated index %s already persisted, skipping MinIO PUT", utils.ShortenDigest(indexDigest))
		return false
	}
	if storeErr := h.mag.Minio.ManifestsPutObj(ctx, indexDigest, indexBytes); storeErr != nil {
		utils.Warn(l, span, "failed to persist manipulated index in MinIO: %v", storeErr)
	} else {
		persistedIndexDigests.Store(indexDigest, struct{}{})
		utils.Debug(l, span, "persisted manipulated index %s in MinIO (%d bytes)", utils.ShortenDigest(indexDigest), len(indexBytes))
	}
	return false
}

// handleIndexPut handles push-side requests for OCI image indexes (formerly
// handleOCIManifestListPut). It calls StoreIndexUpload which implements
// dual-storage semantics (origin=client-pushed, ordered REFERENCES edges,
// Slot HAS_PUSHED_INDEX attribution) per design D8.
func (h Handlers) handleIndexPut(ctx context.Context, w http.ResponseWriter, req types.Request, buf []byte) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciIndex,
		"func":    "handleIndexPut",
		"repo":    req.Repo,
	})
	defer span.End()
	ml, err := types.NewManifestListFromData(buf)
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "MANIFEST_INVALID", "failed to parse index: %v", err)
		return
	}
	utils.Debug(l, span, "Index has %d annotations", len(ml.Annotations))
	utils.Debug(l, span, "storing index with %d manifests", len(ml.Manifests))
	utils.Debug(l, span, "%s", utils.JsonPrettyPrint(buf))
	// Path Z task 8.1: StoreManifestListUpload was renamed to
	// StoreIndexUpload. The Path Z implementation adds dual-storage
	// semantics (origin=client-pushed, ordered REFERENCES edges,
	// Slot HAS_PUSHED_INDEX attribution) per design D8.
	if err = h.mag.StoreIndexUpload(ctx, req, &ml); err != nil {
		l.Errorf("failed to store index: %v", err)
		span.RecordError(err)
		writeOCIError(w, err)
		return
	}
	utils.Debug(l, span, "successfully stored index with id '%d'", ml.ID)
	utils.Debug(l, span, " successStoredManifestWDig:%s", ml.Digest)
	w.Header().Set("Docker-Content-Digest", ml.Digest)
	if ml.Subject != nil {
		w.Header().Set("OCI-Subject", string(ml.Subject.Digest))
	}
	w.WriteHeader(http.StatusCreated)
}
