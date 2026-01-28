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
)

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
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to check if manifest exists")
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
	// 1. Check if a manifest with repo:tag exists
	up.Profile = defaultProfile
	utils.Debug(l, span, "Check if manifest exists for profile '%s'", up.Profile)
	_, maniFound, e := h.mag.Database.ManifestExists(ctx, up)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to check if manifest exists")
		return
	}
	var upstreamFound bool
	if !maniFound {
		var md string
		var e error
		md, upstreamFound, e = h.mag.HeadManifestNotFound(ctx, req)
		if e != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to HeadManifestNotFound")
			return
		}
		if !upstreamFound {
			switch md {
			case dockerManifestListMediaType:
				err := fmt.Errorf("image indexes are not supported (yet): %s", md)
				utils.ErrorHandlerWithWWW(w, l, span, req.Repo,
					"head", err, "MANIFEST_UNKNOWN", "")
				// We are returning as we did not store an upstream manifest and it won't be found in the graph
				return
			}
			e = errors.New("manifest not found")
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo,
				"head", e, "MANIFEST_UNKNOWN", "failed to fetch manifest upstream '%s/%s'", req.Repo, req.Tag)
			// We do not stop here, as the manifest will now be in the graph and subsequent queries will find it
		}
	}
	/// 2. Check if a manifest with repo:tag exists for profile
	up.Profile = req.Profile
	utils.Debug(l, span, "Check if manifest exists for profile '%s'", up.Profile)
	tpMediaType, tpManiFound, e := h.mag.Database.ManifestExistsForTagProfile(ctx, up)
	if e != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to check if manifest exists")
		return
	}
	utils.Debug(l, span, "Manifest found for profile '%s':  %t - MediaType '%s'", up.Profile, tpManiFound, tpMediaType)
	var tpUpstreamFound bool

	if !tpManiFound {
		// TODO: ok, we found a manifest for generic, not we need to specialise it for the profile
		tpMediaType, tpUpstreamFound, e = h.mag.HeadManistNotFoundForProfile(ctx, req)
		if e != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to HeadManifestNotFound")
			return
		}
		//err := fmt.Errorf("failed to fetch ManifestList for collection: %s/%s", req.Repo, req.Tag)
		//utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", err, "MANIFEST_UNKNOWN", "")
		//return
	}
	if tpUpstreamFound {
		tpMediaType = ociManifestListMediaType
		utils.Debug(l, span, "Manifest was fetched upstream, set mediaType to '%s'", tpMediaType)
	} else {
		utils.Debug(l, span, "Manifest found with type '%s'", tpMediaType)
	}
	switch tpMediaType {
	case ociManifestListMediaType:
		ml, found, err := h.mag.ManifestListGetByHTTPRequest(ctx, r)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to fetch ManifestList")
			return
		}
		if found {
			ml.WriteWWW(w)
			return
		}
	case ociManifestMediaType:
		m, found, e := h.mag.ManifestGetByHTTPRequest(ctx, r)
		if e != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", e, "INTERNAL_ERROR", "failed to fetch Manifest")
			return
		}
		if found {
			m.WriteWWW(w)
			return
		}
	default:
		//application/vnd.oci.image.manifest.v1+json
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "head", fmt.Errorf("unkown mediaType for manifest: '%s'", tpMediaType), "INTERNAL_ERROR", "")
		return
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
	h.ManifestHandler(ctx, w, req)
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

func (h Handlers) handleManifestList(ctx context.Context, w http.ResponseWriter, req types.Request) (ok bool) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "handleManifestList",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	ml, found, err := h.mag.ManifestListGetByRequest(ctx, req)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		l.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
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
			msg := fmt.Sprintf("unsupported method '%s' for manifest list", strings.ToUpper(req.Method))
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		}
		ok = true
		return
	}
	return false
}

func (h Handlers) ManifestHandler(ctx context.Context, w http.ResponseWriter, req types.Request) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOciMani,
		"func":    "ManifestHandler",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
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
		msg := fmt.Sprintf("failed to check if collection exists: %v", err)
		l.Error(msg)
		err := errors.New(msg)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if !colFound {
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
		return
	}
	msg := fmt.Sprintf("Manifest found with type '%s'", mediaType)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	switch mediaType {
	case ociManifestListMediaType:
		if ok := h.handleManifestList(ctx, w, req); ok {
			return
		}
	case
		ociManifestMediaType,
		dockerManifestMediaType:
		if ok := h.handleManifest(ctx, w, req); ok {
			return
		}
	default:
		msg := fmt.Sprintf("unkown mediaType '%s'", mediaType)
		l.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	msg = fmt.Sprintf("no manifest '%s:%s' found for profile '%s'", req.Repo, req.Tag, req.Profile)
	det := helper.NewDetailFromReq(req)
	e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
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
	h.ManifestHandler(ctx, w, req)
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
	span.SetAttributes(
		attribute.Key("body-length").Int(len(buf)),
	)
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
	if !found {
		e = fmt.Errorf("profile '%s' not found", req.Profile)
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "")
		return
	}
	switch req.ContentType {
	case dockerManifestMediaType:
		// Put manifest in object store
		m, err := h.mag.StoreManifestUpload(ctx, req, buf)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "failed to store manifest")
			return
		}
		utils.Debug(l, span, successStoredManifestWDig, m.Digest)
		// TODO: 2023-04-14 Unwind profile for uploaded manifest
		err = h.mag.Database.ProfileUnwindRules(ctx, prf.ID, m.ID)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "failed to unwind profile '%s' for manifest '%s'", req.Profile, m.Label)
			return
		}
		// Compute manifest
		mani, found, err := h.mag.Database.ManifestGetByLabel(ctx, m.Label)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "DENIED", "while executing ManifestGetByLabel")
			return
		}
		if !found {
			err = utils.NewError(l, span, "manifest not found")
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "")
			return
		}
		err = h.mag.ComputeManifest(ctx, mani)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "while executing ComputeManifest")
			return
		}
		err = h.mag.ComputeManifestlists(ctx)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "INTERNAL_ERROR", "while executing ComputeManifestlists")
			return
		}
		h.ManifestPutHandlerWrapper(ctx, w, req, m)
	case ociManifestMediaType:
		// Put manifest in object store
		m, err := h.mag.StoreManifestUpload(ctx, req, buf)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "MANIFEST_INVALID", "failed to store manifest: %v", err)
			return
		}
		h.ManifestPutHandlerWrapper(ctx, w, req, m)
	case ociManifestListMediaType:
		// Put manifest list in object store
		ml, err := types.NewManifestListFromData(buf)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "MANIFEST_INVALID", "failed to parse manifest list: %v", err)
			return
		}
		utils.Debug(l, span, "ManifestList has %d annotations", len(ml.Annotations))
		utils.Debug(l, span, "storing manifest list with %d manifests", len(ml.Manifests))
		utils.Debug(l, span, "%s", utils.JsonPrettyPrint(buf))
		err = h.mag.StoreManifestListUpload(ctx, req, &ml)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", err, "MANIFEST_INVALID", "failed to store manifest list: %v", err)
			return
		} else {
			utils.Debug(l, span, "successfully stored manifest list with id '%d'", ml.ID)
		}
		utils.Debug(l, span, " successStoredManifestWDig:%s", ml.Digest)
		w.Header().Set("Docker-Content-Digest", ml.Digest)
		if ml.Subject != nil {
			w.Header().Set("OCI-Subject", string(ml.Subject.Digest))
		}
		w.WriteHeader(http.StatusCreated)
		return
	default:
		e = fmt.Errorf("unsupported content type: %s (supported: %s)", req.ContentType, strings.Join([]string{dockerManifestMediaType, ociManifestMediaType}, ","))
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "MANIFEST_INVALID", "")
		return
	}
}

func (h Handlers) ManifestPutHandlerWrapper(ctx context.Context, w http.ResponseWriter, req types.Request, m types.ManifestData) {
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
	w.Header().Set("Location", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", req.Protocol, req.Host, req.Repo, req.Tag))
	w.WriteHeader(http.StatusCreated)
}
