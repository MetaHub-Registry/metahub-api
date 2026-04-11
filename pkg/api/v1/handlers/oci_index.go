package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
