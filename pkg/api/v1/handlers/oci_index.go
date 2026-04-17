package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameOciIndex = "api/handler/oci_index.go"
)

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
	ml, found, err := h.mag.IndexGetByRequest(ctx, req)
	if err != nil {
		l.Errorf("failed to fetch Index: %v", err)
		writeOCIError(w, fmt.Errorf("failed to fetch Index: %w", err))
		ok = true
		return
	}
	if found {
		// Lazy manipulation: if the pulling profile has manipulations,
		// apply them to the Index's manifests on-demand. Manipulated
		// copies are cached in the graph + MinIO for subsequent pulls.
		if len(ml.Manifests) > 0 && req.Profile != "" {
			prf, prfFound, prfErr := h.mag.Database.ProfileGetByLabel(ctx, req.Profile)
			if prfErr != nil {
				utils.Warn(l, span, "failed to fetch profile '%s' for manipulation check: %v", req.Profile, prfErr)
			} else if prfFound {
				manipulated, manipErr := h.mag.ApplyManipulationsToManifests(ctx, prf, ml.Manifests)
				if manipErr != nil {
					utils.Warn(l, span, "lazy manipulation failed for profile '%s' (serving original): %v", req.Profile, manipErr)
				} else {
					ml.Manifests = manipulated
					// Persist the manipulated Index in MinIO so that
					// Docker's follow-up GET-by-digest can find it.
					// Without this, the on-the-fly Index digest returned
					// in HEAD is transient and the GET returns 404.
					indexBytes := ml.ToOCIBytes()
					indexDigest := digest.FromBytes(indexBytes).String()
					if storeErr := h.mag.Minio.ManifestsPutObj(ctx, indexDigest, indexBytes); storeErr != nil {
						utils.Warn(l, span, "failed to persist manipulated index in MinIO: %v", storeErr)
					} else {
						utils.Debug(l, span, "persisted manipulated index %s in MinIO (%d bytes)", utils.ShortenDigest(indexDigest), len(indexBytes))
					}
				}
			}
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
		ok = true
		return
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
