package handlers

import (
	"io"
	"net/http"

	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
	mlibUtils "github.com/metahub-registry/metahub-core/pkg/utils"
)

type blobContentResponse struct {
	Content string `json:"content"`
}

const (
	pkgNameManifestBlob  = "handlers/manifest_blob.go"
	modelConfigMediaType = "application/vnd.qnib.model.config.v1+json"
)

// ManifestBlobContentGet handles GET /api/v1/manifest/blob-content?repo=&ref=
// It resolves repo:ref → OCI manifest → config layer blob and streams the blob
// content as plain text. The endpoint uses API JWT auth (not OCI Bearer) so the
// browser UI can display config content without going through the OCI auth flow.
func (h Handlers) ManifestBlobContentGet(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := mlibUtils.InitLogging(r.Context(), tracer, map[string]any{
		"package": pkgNameManifestBlob,
		"func":    "ManifestBlobContentGet",
	})
	defer span.End()

	repo := r.URL.Query().Get("repo")
	ref := r.URL.Query().Get("ref")
	if repo == "" || ref == "" {
		_ = h.writeJSON(w, http.StatusBadRequest, envelope{"error": "repo and ref are required"})
		return
	}

	req := mlibTypes.Request{Repo: repo, Tag: ref}
	m, found, err := h.mag.ManifestGetByRequest(ctx, req)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "ManifestGetByRequest failed")
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	if !found {
		_ = h.writeJSON(w, http.StatusNotFound, envelope{"error": "manifest not found"})
		return
	}

	// Prefer the model-config media type; fall back to the first layer.
	layerDigest := pickBlobLayerDigest(m.Layers)
	if layerDigest == "" {
		_ = h.writeJSON(w, http.StatusNotFound, envelope{"error": "no layers in manifest"})
		return
	}

	rc, err := h.mag.Minio.BlobGetReader(ctx, layerDigest)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "BlobGetReader failed")
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "ReadAll blob failed")
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	_ = h.writeJSON(w, http.StatusOK, blobContentResponse{Content: string(b)})
}

// pickBlobLayerDigest returns the digest of the layer whose content to serve:
// the model-config layer when present, else the first layer, else "" when the
// manifest has no layers.
func pickBlobLayerDigest(layers []mlibTypes.Layer) string {
	for _, lyr := range layers {
		if lyr.MediaType == modelConfigMediaType {
			return lyr.Digest
		}
	}
	if len(layers) > 0 {
		return layers[0].Digest
	}
	return ""
}
