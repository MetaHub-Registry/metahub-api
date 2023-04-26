package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (h Handlers) ManifestHeadHandler(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": "api/handler/oci_manifest.go",
		"func":    "ManifestHeadHandler",
	})
	// Check whether it is a manifest or manifest list
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ManifestHeadHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
	)
	up := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  digest.Digest(req.Digest),
		Profile: req.Profile,
	}
	if req.Digest != "" {
		span.SetAttributes(attribute.Key("digest").String(req.Digest))
		l.Infof("%-5s ManifestHeadHandler(%s@sha256:%s) Profile:%s", r.Method, req.Repo, utils.ShortenDigest(req.Digest), req.Profile)
	} else {
		l.Infof("%-5s ManifestHeadHandler(%s:%s) Profile:%s", r.Method, req.Repo, req.Tag, req.Profile)
	}

	colFound, err := h.mag.Database.CollectionExists(ctx, up)
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
		l.Infof("Collection NOT found for %s/%s", req.Repo, req.Tag)
		colFound, e := h.mag.HeadManifestNotFound(ctx, req)
		if e != nil {
			err := errors.Wrap(e, "failed to HeadManifestNotFound")
			span.RecordError(err)
			l.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		if !colFound {
			imgName := fmt.Sprintf("%s/%s", req.Repo, req.Tag)
			msg := fmt.Sprintf("failed to fetch collection upstream '%s'", imgName)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
		}
	}
	l.Infof("Collection found for %s/%s", req.Repo, req.Tag)
	mediaType, maniFound, err := h.mag.Database.ManifestExists(ctx, up)
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
	if !maniFound {
		upstreamFound, e := h.mag.HeadManifestNotFound(ctx, req)
		if e != nil {
			err := errors.Wrap(e, "failed to HeadManifestNotFound")
			span.RecordError(err)
			l.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		if !upstreamFound {
			imgName := fmt.Sprintf("%s/%s", req.Repo, req.Tag)
			msg := fmt.Sprintf("failed to fetch manifest upstream '%s'", imgName)
			span.AddEvent("log", trace.WithAttributes(
				attribute.String("log.severity", "debug"),
				attribute.String("log.message", msg),
			))
			l.Debug(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
			helper.WriteOCIErrror(w, http.StatusNotFound, e)
		}

	}
	msg := fmt.Sprintf("Manifest found with type '%s'", mediaType)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.message", msg),
	))
	l.Debug(msg)
	switch mediaType {
	case "application/vnd.oci.image.index.v1+json":
		ml, found, err := h.mag.ManifestList_GetByHTTPRequest(ctx, r)
		if err != nil {
			// 1. FALSE
			msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		if found {
			ml.WriteWWW(w)
			return
		}
	case "application/vnd.oci.image.manifest.v1+json":
		m, found, err := h.mag.Manifest_GetByHTTPRequest(ctx, r)
		if err != nil {
			// 1. FALSE
			msg := fmt.Sprintf("failed to fetch Manifest: %v", err)
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusNotFound, e)
			return
		}
		if found {
			m.WriteWWW(w)
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
	imgName := fmt.Sprintf("%s/%s", req.Repo, req.Tag)
	msg = fmt.Sprintf("failed to fetch ManifestList for collection '%s'", imgName)
	l.Error(msg)
	det := helper.NewDetailFromReq(req)
	e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
}

func (h Handlers) ManifestDelHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ManifestDelHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	err := h.mag.Manifest_DeleteByHTTPRequest(ctx, r)
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

func (h Handlers) ManifestGetHandler(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": "api/handler/oci_manifest.go",
		"func":    "ManifestGetHandler",
	})
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ManifestGetHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("tag").String(req.Tag),
		attribute.Key("digest").String(req.Digest),
	)
	l = l.WithFields(logrus.Fields{
		"repo":   req.Repo,
		"tag":    req.Tag,
		"digest": utils.ShortenDigest(req.Digest),
	})
	up := upstream.UpstreamTriple{
		Reg:     req.Host,
		Repo:    req.Repo,
		Tag:     req.Tag,
		Digest:  digest.Digest(req.Digest),
		Profile: req.Profile,
	}
	if req.Digest != "" {
		span.SetAttributes(attribute.Key("digest").String(req.Digest))
		l.Infof("%-5s ManifestGetHandler(%s@sha256:%s) Profile:%s", r.Method, req.Repo, utils.ShortenDigest(req.Digest), req.Profile)
	} else {
		l.Infof("%-5s ManifestGetHandler(%s:%s) Profile:%s", r.Method, req.Repo, req.Tag, req.Profile)
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
		msg := fmt.Sprintf("collection for manifest(list) '%s@%s' not found", req.Repo, utils.ShortenDigest(req.Digest))
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
	case "application/vnd.oci.image.index.v1+json":
		ml, found, err := h.mag.ManifestList_GetByHTTPRequest(ctx, r)
		if err != nil {
			// 1. FALSE
			msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		if found {
			ml.WriteWWW(w)
			return
		}
	case "application/vnd.oci.image.manifest.v1+json":
		m, found, err := h.mag.Manifest_GetByHTTPRequest(ctx, r)
		if err != nil {
			// 1. FALSE
			msg := fmt.Sprintf("failed to fetch Manifest: %v", err)
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusNotFound, e)
			return
		}
		if found {
			m.WriteWWW(w)
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

func (h Handlers) ManifestPutHandler(w http.ResponseWriter, r *http.Request) {
	l := logrus.WithFields(logrus.Fields{
		"package": "oci_manifest.go",
		"func":    "ManifestPutHandler",
	})
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ManifestPutHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	l.Infof("%-5s ManifestPutHandler(%s, %s) Profile:%s", r.Method, req.Repo, req.Tag, req.Profile)
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Fatal("request", err)
	}
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("body-length").Int(len(buf)),
		attribute.Key("profile").String(req.Profile),
	)
	_, e := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		err = errors.Wrapf(e, "failed to authorize push for profile '%s'", req.Profile)
		span.RecordError(err)
		l.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("DENIED", e.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}
	prf, found, err := h.mag.Database.Profile_Get_ByLabel(ctx, req.Profile)
	if err != nil {
		err := errors.Wrapf(err, "failed to fetch profile '%s'", req.Profile)
		l.Error(err)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}
	if !found {
		err := errors.Errorf("profile '%s' not found", req.Profile)
		l.Error(err)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}

	switch req.ContentType {
	case "application/vnd.docker.distribution.manifest.v2+json":
		// Put manifest in object store
		m, err := h.mag.StoreManifestUpload(ctx, req, buf)
		if err != nil {
			err := errors.Wrapf(err, "failed to store manifest")
			l.Error(err)
			span.RecordError(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		msg := fmt.Sprintf("successfully stored maniefst with digest '%s'", m.Digest)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		// TODO: 2023-04-14 Unwind profile for uploaded manifest
		err = h.mag.Database.Profile_UnwindRules(ctx, prf.ID, m.ID)
		if err != nil {
			err := errors.Wrapf(err, "failed to unwind profile '%s' for manifest '%s'", req.Profile, m.Label)
			l.Error(err)
			span.RecordError(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		// Compute manifest
		mani, err := h.mag.Database.Manifest_GetByLabel(ctx, m.Label)
		if err != nil {
			err = errors.Wrap(err, "while executing Manifest_GetByLabel")
			span.RecordError(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("DENIED", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		err = h.mag.ComputeManifest(ctx, mani)
		if err != nil {
			err = errors.Wrap(err, "while executing ComputeManifest")
			span.RecordError(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		err = h.mag.ComputeManifestlists(ctx)
		if err != nil {
			err = errors.Wrap(err, "while executing ComputeManifestlists")
			span.RecordError(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", err.Error(), det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		span.SetStatus(codes.Ok, "manifest successfully uploaded")
		m.WriteWWWHeader(w)
		w.Header().Set("Location", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", req.Protocol, req.Host, req.Repo, req.Tag))
		w.WriteHeader(http.StatusCreated)
	case "application/vnd.docker.distribution.manifest.v1+prettyjws":
		m, err := h.mag.StoreManifestUpload(ctx, req, buf)
		if err != nil {
			msg := fmt.Sprintf("failed to store manifest: %v", err)
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("MANIFEST_INVALID", msg, det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		m.WriteWWWHeader(w)
		w.Header().Set("Location", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", req.Protocol, req.Host, req.Repo, req.Tag))
		w.WriteHeader(http.StatusCreated)
	case "application/vnd.oci.image.manifest.v1+json":
		// Put manifest in object store
		m, err := h.mag.StoreManifestUpload(ctx, req, buf)
		if err != nil {
			msg := fmt.Sprintf("failed to store manifest: %v", err)
			l.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("MANIFEST_INVALID", msg, det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		// Put manifest in database
		m.WriteWWWHeader(w)
		w.Header().Set("Location", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", req.Protocol, req.Host, req.Repo, req.Tag))
		w.WriteHeader(http.StatusCreated)
	default:
		msg := fmt.Sprintf("Unsupported ContentType: %s", req.ContentType)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	}
}
