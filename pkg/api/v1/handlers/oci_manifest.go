package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (h Handlers) ManifestHeadHandler(w http.ResponseWriter, r *http.Request) {
	// Check whether it is a manifest or manifest list
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ManifestHeadHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
	)

	m, found, err := h.mag.Manifest_GetByHTTPRequest(ctx, r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		logrus.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	if found {
		m.WriteWWWHeader(w)
		return
	}
	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(ctx, r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		logrus.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if found {
		logrus.Warnf("ManifestList found for %s/%s: \n%v", req.Repo, req.Tag, ml)
		ml.WriteWWWHeader(w)
		return
	}
	imgName := fmt.Sprintf("%s/%s", req.Repo, req.Tag)
	msg := fmt.Sprintf("failed to fetch ManifestList for collection '%s'", imgName)
	logrus.Error(msg)
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
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ManifestGetHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
	)
	m, found, err := h.mag.Manifest_GetByHTTPRequest(ctx, r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch Manifest: %v", err)
		logrus.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	if found {
		m.WriteWWW(w)
		return
	}
	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(ctx, r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		logrus.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if found {
		ml.WriteWWW(w)
		return
	} else {
		msg := fmt.Sprintf("no manifest '%s:%s' found for profile '%s'", req.Repo, req.Tag, req.Profile)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	}
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
