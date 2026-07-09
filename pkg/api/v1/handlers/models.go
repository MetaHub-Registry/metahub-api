package handlers

import (
	"context"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
	mlibUtils "github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/trace"
)

const pkgNameModels = "handlers/models.go"

// initModelLog starts a trace span + logger for a models.go handler. Extracted so
// the handlers below don't each repeat the InitLogging attribute block.
func initModelLog(r *http.Request, fn string) (*logrus.Entry, context.Context, trace.Span) {
	return mlibUtils.InitLogging(r.Context(), tracer, map[string]any{
		"package": pkgNameModels,
		"func":    fn,
	})
}

// loadModelDetail resolves the {ref} path param to a model card, writing the
// appropriate JSON error and returning ok=false when the caller should stop:
// 400 when ref is missing, 500 when the backend fails, 404 when no such
// `model/<name>` exists. Shared by ModelsGetOne and ModelsGetCompositions, which
// differ only in which slice of the resolved card they emit.
func (h Handlers) loadModelDetail(w http.ResponseWriter, r *http.Request, ctx context.Context, l *logrus.Entry, span trace.Span) (mlibTypes.ModelDetail, bool) {
	ref := chi.URLParam(r, "ref")
	// Org-qualified names route via the {org}/{name} pair (the %2F redirect
	// target); reassemble them into the `org/name` ref the backend expects.
	if ref == "" {
		if org := chi.URLParam(r, "org"); org != "" {
			ref = org + "/" + chi.URLParam(r, "name")
		}
	}
	if ref == "" {
		_ = h.writeJSON(w, http.StatusBadRequest, envelope{"error": "model ref required"})
		return mlibTypes.ModelDetail{}, false
	}
	// Org-qualified model names carry a `/` (models/<org>/<name>), which the UI
	// sends as a single %2F-encoded path segment; chi hands us the still-escaped
	// value, so decode it (`qnib%2Fminicpm5` → `qnib/minicpm5`) before lookup —
	// otherwise the literal `%` fails the neo safe-name guard and 404s.
	if dec, derr := url.PathUnescape(ref); derr == nil {
		ref = dec
	}
	detail, found, err := h.mag.Database.ModelDetailGet(ctx, ref)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "ModelDetailGet failed")
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return mlibTypes.ModelDetail{}, false
	}
	if !found {
		_ = h.writeJSON(w, http.StatusNotFound, envelope{"error": "model not found"})
		return mlibTypes.ModelDetail{}, false
	}
	return detail, true
}

// ModelsGetAll handles GET /api/v1/models — the model catalog. It returns one
// entry per `model/<name>` manipulation (produced by `metahub-admin repackage`):
// the model name, the VRAM profiles that inject it, and its repackaged
// model-layer image variant(s). These images are excluded from the collections
// listing, so the Models tab is where they surface.
func (h Handlers) ModelsGetAll(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := initModelLog(r, "ModelsGetAll")
	defer span.End()

	models, err := h.mag.Database.ModelListingGetAll(ctx)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "ModelListingGetAll failed")
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}

	_ = h.writeJSON(w, http.StatusOK, envelope{"models": models})
}

// ModelsGetOne handles GET /api/v1/models/{ref} — the card payload for one model
// (`model/<name>`): every quantization grouped under it (layer image + size),
// the per-profile runtime configs (`config/<name>`), the profiles it is served
// under, and the source ORAS artifact. Answers 404 when no such model exists.
func (h Handlers) ModelsGetOne(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := initModelLog(r, "ModelsGetOne")
	defer span.End()

	detail, ok := h.loadModelDetail(w, r, ctx, l, span)
	if !ok {
		return
	}
	_ = h.writeJSON(w, http.StatusOK, envelope{"model": detail})
}

// ModelsGetCompositions handles GET /api/v1/models/{ref}/compositions — the
// resolved compositions for a model (runtime + quant + config), derived from the
// model's referrers. 404 when no such model exists.
func (h Handlers) ModelsGetCompositions(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := initModelLog(r, "ModelsGetCompositions")
	defer span.End()

	detail, ok := h.loadModelDetail(w, r, ctx, l, span)
	if !ok {
		return
	}
	_ = h.writeJSON(w, http.StatusOK, envelope{"compositions": detail.Compositions})
}

// RuntimesGetAll handles GET /api/v1/runtimes — the runtime registry (application
// containers + their model-format / config-contract compatibility).
func (h Handlers) RuntimesGetAll(w http.ResponseWriter, r *http.Request) {
	l, ctx, span := initModelLog(r, "RuntimesGetAll")
	defer span.End()

	runtimes, err := h.mag.Database.RuntimesGetAll(ctx)
	if err != nil {
		mlibUtils.ErrorHandler(l, span, err, "RuntimesGetAll failed")
		_ = h.writeJSON(w, http.StatusInternalServerError, envelope{"error": err.Error()})
		return
	}
	_ = h.writeJSON(w, http.StatusOK, envelope{"runtimes": runtimes})
}
