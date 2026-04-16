package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameOciMani              = "api/handler/oci_manifest.go"
	ociManifestMediaType        = "application/vnd.oci.image.manifest.v1+json"
	ociManifestListMediaType    = "application/vnd.oci.image.index.v1+json"
	dockerManifestMediaType     = "application/vnd.docker.distribution.manifest.v2+json"
	dockerManifestListMediaType = "application/vnd.docker.distribution.manifest.list.v2+json"
	successStoredManifestWDig   = "successfully stored manifest with digest '%s'"
	failedAuthPushPrfFMT        = "failed to authorize push for profile '%s'"
	failedCheckManifestExists   = "failed to check if manifest exists"
)

// clientAcceptsIndex returns true when the request's Accept header includes an
// image-index (or Docker manifest-list) media type. When false, the client has
// explicitly asked for a single manifest and the response path should attempt
// content-negotiation by unwrapping a single-descriptor index.
func clientAcceptsIndex(r *http.Request) bool {
	for _, a := range r.Header.Values("Accept") {
		for _, mt := range strings.Split(a, ",") {
			mt = strings.TrimSpace(mt)
			// Strip optional quality parameter (e.g. ";q=0.9")
			if idx := strings.Index(mt, ";"); idx != -1 {
				mt = strings.TrimSpace(mt[:idx])
			}
			switch mt {
			case ociManifestListMediaType, dockerManifestListMediaType, "*/*":
				return true
			}
		}
	}
	return false
}

func (h Handlers) ManifestHeadHandlerDigest(ctx context.Context, w http.ResponseWriter, r *http.Request, up upstream.UpstreamTriple) {
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "api.ManifestHeadHandlerDigest",
		"method":  r.Method,
		"repo":    req.Repo,
		"digest":  req.Digest,
		"profile": req.Profile,
		"host":    req.Host,
	})
	defer span.End()
	l.Infof("%-5s ManifestHeadHandlerDigest(%s@sha256:%s) Profile:%s", r.Method, req.Repo, utils.ShortenDigest(req.Digest), req.Profile)
	mediaType, maniFound, e := h.mag.Database.ManifestExists(ctx, up)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", failedCheckManifestExists)
		return
	}
	if !maniFound {
		err := fmt.Errorf("failed to fetch ManifestList for collection: %s@%s", req.Repo, req.Digest)
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", err, "MANIFEST_UNKNOWN", "")
		return
	}
	utils.Debug(l, span, "Manifest found: %t - MediaType '%s'", maniFound, mediaType)
	switch mediaType {
	case ociManifestListMediaType, dockerManifestListMediaType:
		ml, found, e := h.mag.ManifestListGetByHTTPRequest(ctx, r)
		if e != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to fetch ManifestList")
			return
		}
		if found {
			ml.WriteWWW(w)
			return
		}
	case ociManifestMediaType, dockerManifestMediaType:
		m, found, err := h.mag.ManifestGetByHTTPRequest(ctx, r)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to fetch Manifest")
			return
		}
		if found {
			m.WriteWWW(w)
			return
		}
	default:
		e := fmt.Errorf("unkown mediaType for manifest: '%s'", mediaType)
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "unkown mediaType '%s'", mediaType)
		return
	}
	err := fmt.Errorf("failed to fetch ManifestList for collection: %s@%s", req.Repo, req.Digest)
	utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", err, "MANIFEST_UNKNOWN", "")
}

func (h Handlers) ManifestHeadHandlerTag(ctx context.Context, w http.ResponseWriter, r *http.Request, up upstream.UpstreamTriple) {
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "api.ManifestHeadHandlerTag",
		"method":  r.Method,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"profile": req.Profile,
		"host":    req.Host,
	})
	defer span.End()
	utils.Info(l, span, "%-5s ManifestHeadHandlerTag(%s:%s) Profile:%s", r.Method, req.Repo, req.Tag, req.Profile)
	_, _, e := h.mag.Database.CollectionGetCreateByRepoTag(ctx, req.Repo, req.Tag)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to CollectionGetCreateByRepoTag")
		return
	}
	// 1. Check if a manifest with repo:tag exists (generic profile)
	up.Profile = defaultProfile
	if stop := h.ensureManifestForGenericProfile(ctx, w, req, up); stop {
		return
	}
	// 2. Check if a manifest with repo:tag exists for requested profile
	up.Profile = req.Profile
	tpMediaType, err := h.resolveManifestMediaTypeForProfile(ctx, w, req, up)
	if err != nil {
		return
	}
	h.writeManifestHeadResponse(ctx, w, r, req, tpMediaType)
}

// ensureManifestForGenericProfile checks whether the manifest exists for the default
// profile and fetches it from upstream if not. Returns true if the caller should stop.
func (h Handlers) ensureManifestForGenericProfile(ctx context.Context, w http.ResponseWriter, req types.Request, up upstream.UpstreamTriple) bool {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{"package": pkgNameOciMani, "func": "ensureManifestForGenericProfile"})
	defer span.End()
	_, maniFound, e := h.mag.Database.ManifestExists(ctx, up)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", failedCheckManifestExists)
		return true
	}
	if maniFound {
		return false
	}
	md, upstreamFound, e := h.mag.HeadManifestNotFound(ctx, req)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to HeadManifestNotFound")
		return true
	}
	if !upstreamFound {
		if md == dockerManifestListMediaType {
			err := fmt.Errorf("image indexes are not supported (yet): %s", md)
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", err, "MANIFEST_UNKNOWN", "")
			return true
		}
		e = errors.New("manifest not found")
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "MANIFEST_UNKNOWN", "failed to fetch manifest upstream '%s/%s'", req.Repo, req.Tag)
		// Do not stop here — manifest is now in the graph and subsequent queries will find it
	}
	return false
}

// resolveManifestMediaTypeForProfile returns the media type for the requested profile,
// fetching from upstream if needed. Returns error if the caller should stop.
func (h Handlers) resolveManifestMediaTypeForProfile(ctx context.Context, w http.ResponseWriter, req types.Request, up upstream.UpstreamTriple) (string, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{"package": pkgNameOciMani, "func": "resolveManifestMediaTypeForProfile"})
	defer span.End()
	tpMediaType, tpManiFound, e := h.mag.Database.ManifestExistsForTagProfile(ctx, up)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", failedCheckManifestExists)
		return "", e
	}
	if !tpManiFound {
		// TODO: ok, we found a manifest for generic, now we need to specialise it for the profile
		var tpUpstreamFound bool
		tpMediaType, tpUpstreamFound, e = h.mag.HeadManistNotFoundForProfile(ctx, req)
		if e != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to HeadManifestNotFound")
			return "", e
		}
		if tpUpstreamFound {
			tpMediaType = ociManifestListMediaType
		}
	}
	return tpMediaType, nil
}

// writeManifestHeadResponse writes the appropriate HEAD response based on media type.
func (h Handlers) writeManifestHeadResponse(ctx context.Context, w http.ResponseWriter, r *http.Request, req types.Request, tpMediaType string) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{"package": pkgNameOciMani, "func": "writeManifestHeadResponse"})
	defer span.End()
	switch tpMediaType {
	case ociManifestListMediaType:
		ml, found, err := h.mag.ManifestListGetByHTTPRequest(ctx, r)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", err, "INTERNAL_ERROR", "failed to fetch ManifestList")
			return
		}
		if found {
			ml.WriteWWW(w)
		}
	case ociManifestMediaType:
		m, found, e := h.mag.ManifestGetByHTTPRequest(ctx, r)
		if e != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to fetch Manifest")
			return
		}
		if found {
			m.WriteWWW(w)
		}
	default:
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", fmt.Errorf("unknown mediaType for manifest: '%s'", tpMediaType), "INTERNAL_ERROR", "")
	}
}

func (h Handlers) ManifestHeadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "ManifestHeadHandler",
		"method":  r.Method,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
		"host":    req.Host,
	})
	_ = l
	defer span.End()
	h.ManifestHandler(ctx, w, r, req)
}

func (h Handlers) ManifestDelHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "api.ManifestDelHandler",
		"method":  r.Method,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
		"host":    req.Host,
	})
	_ = l
	defer span.End()
	err := h.mag.ManifestDeleteByHTTPRequest(ctx, r)
	if err != nil {
		msg := fmt.Sprintf("failed to delete manifest: %v", err)
		logrus.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h Handlers) handleManifest(ctx context.Context, w http.ResponseWriter, req types.Request) (ok bool) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "handleManifest",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	m, found, err := h.mag.ManifestGetByRequest(ctx, req)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch Manifest: %v", err)
		l.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return true
	}
	if found {
		switch req.Method {
		case http.MethodGet:
			m.WriteWWW(w)
		case http.MethodHead:
			m.WriteWWWHead(w)
		default:
			msg := fmt.Sprintf("unsupported method '%s' for manifest", strings.ToUpper(req.Method))
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		}
		return true
	}
	return false
}

// dispatchByMediaType routes a manifest GET/HEAD to the appropriate handler
// based on the stored media type and the client's Accept header. Returns true
// if the response was written.
func (h Handlers) dispatchByMediaType(ctx context.Context, w http.ResponseWriter, r *http.Request, req types.Request, mediaType string) bool {
	switch mediaType {
	case ociManifestListMediaType:
		// Content negotiation: when the client requests only single-manifest
		// types (i.e. does NOT accept index/manifest-list), try to unwrap
		// a single-descriptor index into the underlying manifest.
		if !clientAcceptsIndex(r) {
			if ok := h.handleIndexAsManifest(ctx, w, r, req); ok {
				return true
			}
		}
		return h.handleIndex(ctx, w, req)
	case ociManifestMediaType, dockerManifestMediaType:
		return h.handleManifest(ctx, w, req)
	default:
		msg := fmt.Sprintf("unkown mediaType '%s'", mediaType)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return true
	}
}

// handleIndexAsManifest implements Accept-header content negotiation for tags
// that resolve to an index. When the index contains exactly one descriptor and
// the client did not ask for an index media type, this helper serves the
// underlying single manifest (with its own digest and media type) so that OCI
// conformance tests and simple clients get the manifest they pushed rather
// than the server-side aggregated index.
//
// Returns true if the response was written (caller should stop), false if the
// caller should fall through to the normal index path.
func (h Handlers) handleIndexAsManifest(ctx context.Context, w http.ResponseWriter, r *http.Request, req types.Request) bool {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "handleIndexAsManifest",
		"repo":    req.Repo,
		"tag":     req.Tag,
	})
	defer span.End()
	ml, found, err := h.mag.IndexGetByRequest(ctx, req)
	if err != nil || !found {
		return false // let normal path handle errors / not-found
	}
	if len(ml.Manifests) != 1 {
		utils.Debug(l, span, "index has %d descriptors, cannot unwrap to single manifest", len(ml.Manifests))
		return false // multi-descriptor → serve index as-is
	}
	// Lazy manipulation for single-descriptor index unwrap
	if req.Profile != "" {
		prf, prfFound, prfErr := h.mag.Database.ProfileGetByLabel(ctx, req.Profile)
		if prfErr == nil && prfFound {
			manipulated, manipErr := h.mag.ApplyManipulationsToManifests(ctx, prf, ml.Manifests)
			if manipErr == nil {
				ml.Manifests = manipulated
			}
		}
	}
	// Single descriptor: serve the underlying manifest
	singleDigest := ml.Manifests[0].Digest
	utils.Debug(l, span, "content-negotiation: unwrapping single-descriptor index to manifest %s", utils.ShortenDigest(singleDigest))
	maniReq := req
	maniReq.Digest = singleDigest
	m, mFound, mErr := h.mag.ManifestGetByRequest(ctx, maniReq)
	if mErr != nil || !mFound {
		return false // fall through to normal index path
	}
	switch req.Method {
	case http.MethodGet:
		m.WriteWWW(w)
	case http.MethodHead:
		m.WriteWWWHead(w)
	default:
		return false
	}
	return true
}

// resolveUpstreamTarget detects an upstream registry prefix on req.Repo
// (e.g. "zot.example.com/library/alpine" → target="zot.example.com"). Only matches
// against configured registries — unknown prefixes return "" and are treated
// as local repo names.
//
// D5/D6 (prefix-preserving pullthrough): the registry prefix is intentionally
// NOT stripped from req.Repo. Preserving the prefix means pullthrough-populated
// repos live in a namespace region (<upstream-host>/<rest>) that cannot collide
// with client-push repos or admin-parsed aliases. Only the target registry is
// threaded into pullthrough discovery; the local alias stays as the full URL path.
func (h Handlers) resolveUpstreamTarget(l *logrus.Entry, span trace.Span, repo string) string {
	if regHost, _, found := h.mag.ResolveUpstreamRegistry(repo); found {
		utils.Info(l, span, "upstream prefix detected: registry=%s (local alias preserved: %s)", regHost, repo)
		return regHost
	}
	return ""
}

// tryPullthroughOnMiss attempts pullthrough when a tag-based manifest lookup
// failed locally. Returns (mediaType, found) after retrying ManifestExists.
// When found is false, the caller SHOULD return a 404 to the client.
func (h Handlers) tryPullthroughOnMiss(ctx context.Context, l *logrus.Entry, span trace.Span, req types.Request, up upstream.UpstreamTriple, targetReg string) (string, bool) {
	utils.Info(l, span, "manifest not found locally, attempting pullthrough for %s:%s (target=%s)", req.Repo, req.Tag, targetReg)
	if _, _, e := h.mag.Database.CollectionGetCreateByRepoTag(ctx, req.Repo, req.Tag); e != nil {
		utils.ErrorHandler(l, span, e, "failed to CollectionGetCreateByRepoTag")
	}
	_, upFound, e := h.mag.HeadManifestNotFound(ctx, req, targetReg)
	if e != nil || !upFound {
		return "", false
	}
	mediaType, colFound, err := h.mag.Database.ManifestExists(ctx, up)
	if err != nil || !colFound {
		return "", false
	}
	return mediaType, true
}

// resolveMissingManifest attempts recovery paths when ManifestExists returned
// found=false: direct digest lookup (Path Z), then tag-based pullthrough. It
// writes the response and returns stop=true in any case where the caller must
// not proceed to dispatch. When stop=false it returns the resolved mediaType
// (pullthrough succeeded).
func (h Handlers) resolveMissingManifest(ctx context.Context, w http.ResponseWriter, l *logrus.Entry, span trace.Span, req types.Request, up upstream.UpstreamTriple, targetReg string) (string, bool) {
	// Path Z: direct digest lookup when ManifestExists's wrapper query missed.
	if req.Digest != "" && h.handleManifest(ctx, w, req) {
		return "", true
	}
	// Pullthrough for tag-based requests only. Digest-based requests cannot
	// be discovered upstream (no tag to resolve against).
	if req.Tag != "" {
		if mt, ok := h.tryPullthroughOnMiss(ctx, l, span, req, up, targetReg); ok {
			return mt, false
		}
	}
	writeManifestNotFound(w, l, span, req)
	return "", true
}

// writeManifestNotFound writes the 404 MANIFEST_UNKNOWN response.
func writeManifestNotFound(w http.ResponseWriter, l *logrus.Entry, span trace.Span, req types.Request) {
	msg := fmt.Sprintf("Manifest for manifest(list) '%s@%s' not found", req.Repo, utils.ShortenDigest(req.Digest))
	l.Error(msg)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	msg = fmt.Sprintf("no manifest '%s:%s' found for profile '%s'", req.Repo, req.Tag, req.Profile)
	det := helper.NewDetailFromReq(req)
	e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
}

// writeInternalError writes a 500 INTERNAL_ERROR response and records on span.
func writeInternalError(w http.ResponseWriter, l *logrus.Entry, span trace.Span, req types.Request, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.Error(msg)
	span.RecordError(errors.New(msg))
	det := helper.NewDetailFromReq(req)
	e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
	helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
}

func (h Handlers) ManifestHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, req types.Request) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "ManifestHandler",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()

	targetReg := h.resolveUpstreamTarget(l, span, req.Repo)
	up := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  digest.Digest(req.Digest),
		Profile: req.Profile,
	}
	if req.Digest != "" {
		span.SetAttributes(attribute.Key("digest").String(req.Digest))
		l.Infof("%-5s ManifestGetHandler(%s@sha256:%s) Profile:%s", req.Method, req.Repo, utils.ShortenDigest(req.Digest), req.Profile)
	} else {
		l.Infof("%-5s ManifestGetHandler(%s:%s) Profile:%s", req.Method, req.Repo, req.Tag, req.Profile)
	}

	mediaType, colFound, err := h.mag.Database.ManifestExists(ctx, up)
	if err != nil {
		writeInternalError(w, l, span, req, "failed to check if collection exists: %v", err)
		return
	}
	if !colFound {
		mt, stop := h.resolveMissingManifest(ctx, w, l, span, req, up, targetReg)
		if stop {
			return
		}
		mediaType = mt
	}
	msg := fmt.Sprintf("Manifest found with type '%s'", mediaType)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	if ok := h.dispatchByMediaType(ctx, w, r, req, mediaType); ok {
		return
	}
	writeManifestNotFound(w, l, span, req)
}

func (h Handlers) ManifestGetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "api.ManifestGetHandler",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	_ = l
	h.ManifestHandler(ctx, w, r, req)
}

func (h Handlers) ManifestPutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "oci_manifest.go",
		"func":    fmt.Sprintf("ManifestPutHandler %s", r.URL.Path),
		"method":  r.Method,
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  req.Digest,
		"profile": req.Profile,
		"host":    req.Host,
	})
	defer span.End()
	reference := chi.URLParam(r, "reference")
	// TODO: if reference is set, we will link the uploaded manifest to the reference (tag)?
	//.      Will we? `reference` also seems to be used for digest/tags? does this make sense?
	span.SetAttributes(attribute.Key("reference").String(reference))
	l.Infof("%-5s ManifestPutHandler(%s, %s) Profile:%s", r.Method, req.Repo, req.Tag, req.Profile)
	buf, e := io.ReadAll(r.Body)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to read body")
		return
	}
	l = l.WithFields(logrus.Fields{
		"body-length": len(buf),
	})
	span.SetAttributes(attribute.Key("body-length").Int(len(buf)))
	_, e = h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "DENIED", failedAuthPushPrfFMT, req.Profile)
		return
	}
	prf, found, err := h.mag.Database.ProfileGetByLabel(ctx, req.Profile)
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "failed to fetch profile '%s'", req.Profile)
		return
	}
	// If label lookup failed, try matching by tag (e.g. "zen2" → "cpu#zen2").
	// The profile field from docker login may contain only the tag portion.
	if !found {
		prfs, prfErr := h.mag.Database.ProfileGetAll(ctx)
		if prfErr == nil {
			for _, candidate := range prfs {
				if candidate.Tag == req.Profile {
					prf = candidate
					found = true
					l.Infof("resolved profile tag '%s' to label '%s'", req.Profile, prf.Label)
					break
				}
			}
		}
	}
	if !found {
		e = fmt.Errorf("profile '%s' not found", req.Profile)
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "")
		return
	}
	switch req.ContentType {
	case dockerManifestMediaType:
		h.handleDockerManifestPut(ctx, w, req, buf, prf)
	case ociManifestMediaType:
		m, err := h.mag.StoreManifestUpload(ctx, req, buf)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "MANIFEST_INVALID", "failed to store manifest: %v", err)
			return
		}
		// Skip compute step when it cannot meaningfully run:
		// - ORAS artifacts (non-empty ArtifactType) do not participate in
		//   multi-arch Index aggregation or profile-based filtering.
		// - Push-by-digest (empty tag) has no Slot triple to compute against.
		if m.ArtifactType != "" {
			utils.Info(l, span, "ORAS artifact detected (type=%s), skipping computeAfterManifestPush", m.ArtifactType)
		} else if req.Tag == "" {
			utils.Info(l, span, "Push-by-digest (no tag), skipping computeAfterManifestPush")
		} else {
			if err = h.computeAfterManifestPush(ctx, l, span, req, prf); err != nil {
				utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "while executing computeAfterManifestPush")
				return
			}
		}
		h.ManifestPutHandlerWrapper(ctx, w, req, m)
	case ociManifestListMediaType:
		h.handleIndexPut(ctx, w, req, buf)
	default:
		e = fmt.Errorf("unsupported content type: %s (supported: %s)", req.ContentType, strings.Join([]string{dockerManifestMediaType, ociManifestMediaType}, ","))
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "MANIFEST_INVALID", "")
	}
}

func (h Handlers) handleDockerManifestPut(ctx context.Context, w http.ResponseWriter, req types.Request, buf []byte, pusherPrf types.Profile) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "handleDockerManifestPut",
		"repo":    req.Repo,
	})
	defer span.End()
	m, err := h.mag.StoreManifestUpload(ctx, req, buf)
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "failed to store manifest")
		return
	}
	utils.Debug(l, span, successStoredManifestWDig, m.Digest)
	// TODO: 2023-04-14 Unwind profile for uploaded manifest
	if err = h.mag.Database.ProfileUnwindRules(ctx, pusherPrf.ID, m.ID); err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "failed to unwind profile '%s' for manifest '%s'", req.Profile, m.Label)
		return
	}
	// Phase C3 compatibility: the wrapper :Manifest node is still created
	// as a shim (metalib storeManifestUploadOCIManifest) until cypher
	// queries are fully rewritten to Slot-based walks. ComputeManifest
	// runs the wrapper-era Docker v2→OCI v1 reshaping.
	mani, found, err := h.mag.Database.ManifestWrapperGetByLabel(ctx, m.Label)
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "DENIED", "while executing ManifestWrapperGetByLabel")
		return
	}
	if found {
		if err = h.mag.ComputeManifest(ctx, mani); err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "while executing ComputeManifest")
			return
		}
	}
	// Push-time scoped index rebuild via the new
	// ComputeManifestlistsForRepoTag helper. This replaces the previous
	// full-system ComputeManifestlists sweep, which was both:
	//
	//   (a) O(all profiles × all collections × all tags) on every push,
	//       wasteful even when correct
	//   (b) silently broken for freshly-pushed content because the rule
	//       getters it relied on walked an Upstream→Manifest edge that
	//       does not exist in the actual graph (Upstream is linked from
	//       ManifestData via [:ORIGINAL])
	//
	// Compose-level validation in the add-multiarch-tag-aggregation-e2e
	// change confirmed that:
	//   - The old sweep produced ZERO ManifestList rows for amd64 then
	//     arm64 alpine pushes (CTPP had empty manifestIDs).
	//   - The new helper produces a 1-descriptor index after the amd64
	//     push and a 2-descriptor index after the arm64 push, with the
	//     historical 1-descriptor index preserved as audit history.
	//
	// Removing the old call is therefore strictly a no-op for the happy
	// path and a perf win for every push.
	if err = h.computeAfterManifestPush(ctx, l, span, req, pusherPrf); err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "while executing computeAfterManifestPush")
		return
	}
	h.ManifestPutHandlerWrapper(ctx, w, req, m)
}

// computeAfterManifestPush runs the scoped push-time index rebuild for the
// just-pushed (req.Repo, req.Tag). It calls
// metalib's ComputeManifestlistsForRepoTag, which:
//
//  1. Walks via the Upstream node's (repo, tag) compound key to find every
//     ManifestData ever pushed to this (repo, tag) — multi-arch friendly.
//  2. Injects the pusher profile's deterministic (anno eq) annotations onto
//     each manifestdata; non-deterministic rules are silently skipped.
//  3. Re-evaluates all rules so the new annotations propagate into
//     Rule-[:TRUE]->Annotation edges.
//  4. For every Profile in the system, rebuilds the per-(repo, tag, profile)
//     ManifestList index from the matching manifestdata. Per-profile compute
//     failures are logged inside the inner helper as warnings and do not
//     fail the push.
//
// Hard errors (e.g. the raw fetcher itself fails, or the rule eval fails)
// are returned and treated by the caller as a push failure, matching the
// existing ComputeManifestlists error handling.
//
// pusherPrf is the Profile resolved from req.Profile in ManifestPutHandler;
// passing it lets ComputeManifestlistsForRepoTag attribute injected
// annotations to the right profile. The function intentionally takes the
// full Profile (not just the ID) so the inner helper can log a meaningful
// label.
func (h Handlers) computeAfterManifestPush(ctx context.Context, l *logrus.Entry, span trace.Span, req types.Request, pusherPrf types.Profile) error {
	utils.Debug(l, span, "computeAfterManifestPush(%s:%s, profile=%s) starting scoped index rebuild", req.Repo, req.Tag, pusherPrf.Label)
	if err := h.mag.ComputeManifestlistsForRepoTag(ctx, req.Repo, req.Tag, &pusherPrf); err != nil {
		return utils.ErrorHandler(l, span, err, "computeAfterManifestPush(%s:%s, profile=%s) failed", req.Repo, req.Tag, pusherPrf.Label)
	}
	return nil
}

func (h Handlers) ManifestPutHandlerWrapper(ctx context.Context, w http.ResponseWriter, req types.Request, m types.Manifest) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "api.ManifestPutHandlerWrapper",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	utils.Debug(l, span, successStoredManifestWDig, m.Digest)
	err := m.WriteWWWHeader(w)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to write WWW header")
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	// Use tag if present, otherwise fall back to digest for the Location header.
	// When a manifest is pushed by digest reference (e.g., PUT /v2/<name>/manifests/sha256:...),
	// req.Tag is empty and we must use the digest to build the correct Location.
	ref := req.Tag
	if ref == "" {
		ref = req.Digest
	}
	w.Header().Set("Location", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", req.Protocol, req.Host, req.Repo, ref))
	w.WriteHeader(http.StatusCreated)
}
