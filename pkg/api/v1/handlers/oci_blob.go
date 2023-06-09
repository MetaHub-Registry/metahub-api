package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameOCIBlob = "handlers/oci_blob.go"
)

func (h Handlers) BlobUploadPutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "BlobUploadPutHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("uuid").String(req.UUID),
		attribute.Key("content-len").Int(req.Length),
	)
	// Complete the upload specified by uuid, optionally appending the body as the final chunk.
	logrus.Infof("%-5s BlobUploadPutHandler(%s, %s)", r.Method, req.Repo, req.UUID)
	_, e := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		err := errors.Wrapf(e, "failed to authorize push for profile '%s'", req.Profile)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("DENIED", e.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal("request", err)
	}
	bu, found, err := h.mag.Database.BlobUpload_GetOCI_ByUUID(ctx, req.UUID)
	if err != nil {
		log.Fatal("request", err)
	}
	if !found {
		msg := fmt.Sprintf("BlobUpload with UUID %s not found", req.UUID)
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("BLOB_UPLOAD_UNKNOWN", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	if req.Length > 0 {
		// We do not have a PATH but directly a PUT request with the data
		dig := digest.FromBytes(buf)
		bu.Digest = dig.String()
		err = h.mag.StoreBlobUpload(ctx, &bu, buf)
		if err != nil {
			msg := "failed to store blob upload"
			err = errors.Wrap(err, msg)
			logrus.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		err = h.mag.Database.BlobUpload_Update_ByUUID(ctx, &bu)
		if err != nil {
			msg := "failed to update blob upload"
			err = errors.Wrap(err, msg)
			logrus.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}

	}
	// Finish the upload
	err = h.mag.Database.BlobUpload_Complete(ctx, &bu)
	if err != nil {
		msg := "failed to update blob in DB"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("Location", bu.Location),
		attribute.String("Range", fmt.Sprintf("%d-%d", bu.RangeStart, bu.RangeStop)),
		attribute.String("Docker-Upload-UUID", bu.UUID.String()),
		attribute.String("Docker-Content-Digest", bu.Digest),
		attribute.String("Content-Length", fmt.Sprintf("%d", len(bu.Data))),
	))
	span.SetStatus(http.StatusAccepted, "Accepted")
	bu.WriteOCIPutResponse(w)
}

func (h Handlers) BlobUploadPatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "BlobUploadPatchHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("length").Int(req.Length),
		attribute.Key("uuid").String(req.UUID),
	)
	logrus.Infof("%-5s BlobUploadPatchHandler(%s, %s) Len:%d", r.Method, req.Repo, req.UUID, req.Length)
	_, e := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		err := errors.Wrapf(e, "failed to authorize push for profile '%s'", req.Profile)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("DENIED", e.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}
	bu, found, err := h.mag.Database.BlobUpload_GetOCI_ByUUID(ctx, req.UUID)
	if err != nil || !found {
		msg := "failed to fetch blob upload"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	// Fetch all chunks already uploaded
	chunks, err := h.mag.Database.UploadChunk_Get_ByUUID(ctx, bu.ID)
	if err != nil {
		msg := "failed to fetch blob upload"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if len(chunks) == 0 && req.RangeStartByte != -1 {
		// We expect the first chunk to start at offset 0
		if req.RangeStartByte != 0 {
			msg := "Range should start at 0"
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusRequestedRangeNotSatisfiable, e)
		}
	}
	/// Create New Chunk
	chunk, err := h.mag.Database.UploadChunk_Create(ctx, bu.ID, req.RangeStartByte, req.RangeBytes)
	if err != nil {
		msg := "failed to create chunk upload"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	_ = chunk
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal("request", err)
	}
	dig := digest.FromBytes(buf)
	bu.Digest = dig.String()
	logrus.Warnf("  >> Before StoreBlobUpload(): bu.ID:%d, UUID:%s", bu.ID, bu.UUID)
	err = h.mag.StoreBlobUpload(ctx, &bu, buf)
	if err != nil {
		msg := "failed to store blob upload"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	err = h.mag.Database.BlobUpload_Update_ByUUID(ctx, &bu)
	if err != nil {
		msg := "failed to update blob upload"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("Location", bu.Location),
		attribute.String("Range", fmt.Sprintf("%d-%d", bu.RangeStart, bu.RangeStop)),
		attribute.String("Docker-Upload-UUID", bu.UUID.String()),
		attribute.String("Content-Length", fmt.Sprintf("%d", len(bu.Data))),
	))
	span.SetStatus(http.StatusAccepted, "Accepted")
	bu.WriteOCIPatchResponse(w)
}

func (h Handlers) BlobUploadPostHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobUploadPostHandler",
		"method":  r.Method,
		"repo":    req.Repo,
		"uuid":    req.UUID,
		"user":    req.User,
		"role":    req.UserRole,
	})
	defer span.End()
	mt := "NONE"
	msg := fmt.Sprintf("%-5s BlobUploadPostHandler(%s, Type:%s, Len:%d", r.Method, mt, req.Repo, req.Length)
	_, e := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		utils.ErrorHandler_WithWWW(w, l, span, req.Repo, "push", e, "DENIED", "failed to authorize push for profile '%s'", req.Profile)
		return
	}
	if req.Tag != "" {
		msg = fmt.Sprintf("%s, Tag:%s)", msg, req.Tag)
	} else {
		msg = fmt.Sprintf("%s, Digest:%s)", msg, utils.ShortenDigest(req.Digest))
	}
	if req.UUID != "" {
		msg = fmt.Sprintf("%s, UUID:%s)", msg, req.UUID)
	}
	l.Info(msg)
	bu := types.OCIBlobUpload{
		UploadUser:    req.User,
		UploadProfile: req.Profile,
		Digest:        req.Digest,
		Repo:          req.Repo,
	}
	err := h.mag.Database.BlobUpload_Create_ByOCI(ctx, &bu)
	if err != nil {
		utils.ErrorHandler_WithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to create blob upload")
		return
	}
	tracing.SpanDebug(span, "request.length:%d", req.Length)
	if req.Length > 0 {
		// Successful completion of the request MUST return a 201 Created and MUST include the following header:
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			err = errors.Wrap(err, "while io.ReadAll(Body)")
			l.Error(err)
			return
		}
		dig := digest.FromBytes(buf)
		bu.Digest = dig.String()
		err = h.mag.StoreBlobUpload(ctx, &bu, buf)
		if err != nil {
			utils.ErrorHandler_WithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to store blob upload")
			return
		}
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("Location", bu.Location),
			attribute.String("Docker-Upload-UUID", bu.UUID.String()),
			attribute.String("Content-Length", fmt.Sprintf("%d", len(bu.Data))),
		))
		span.SetStatus(http.StatusCreated, "Created")
		bu.WriteOCICreateResponse(w)
	} else {
		// Registries that do not support single request monolithic uploads SHOULD return
		// a 202 Accepted status code and Location header and clients SHOULD proceed with a subsequent PUT request
		// TODO: Send presigned upload URL
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("Location", bu.Location),
			attribute.String("Docker-Upload-UUID", bu.UUID.String()),
			attribute.String("Content-Length", fmt.Sprintf("%d", len(bu.Data))),
		))
		span.SetStatus(http.StatusAccepted, "Accepted")
		bu.WriteOCIAcceptedResponse(w)
	}
}

func (h Handlers) BlobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobHandler",
		"method":  r.Method,
		"repo":    req.Repo,
		"tag":     req.Tag,
	})
	defer span.End()
	msg := fmt.Sprintf("%-5s BlobHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	utils.Debug(l, span, msg)
	// Check for blob in Graph
	up := upstream.UpstreamTriple{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: digest.Digest(req.Digest),
	}
	mediaType, blobFound, err := h.mag.BlobExists(ctx, up)
	if err != nil {
		msg := fmt.Sprintf("failed to check if blob exists: %v", err)
		l.Error(msg)
		err := errors.New(msg)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if !blobFound {
		utils.Debug(l, span, "blob for w/ digest '%s' not found", utils.ShortenDigest(req.Digest))
		// Not found
		msg = "could not find blob"
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("NAME_UNKNOWN", msg, det)
		span.SetStatus(http.StatusNotFound, e.Message)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	utils.Debug(l, span, "blob for w/ digest '%s' found w/ mediaType '%s'", utils.ShortenDigest(req.Digest), mediaType)
	switch mediaType {
	case
		"application/vnd.docker.image.rootfs.diff.tar.gzip",
		"application/vnd.oci.image.layer.v1.tar+gzip",
		"unknownLayer":
		// If redirect is enabled, we need to fetch the PreSignedURL
		if r.Method == "GET" {
			utils.Debug(l, span, "GET request for blob")
			// 307 Temporary Redirect
			// Location: <blob location>
			// Docker-Content-Digest: <digest>
			if lyrUp, found, e := h.mag.Database.UpstreamGetByLayerDigest(ctx, req.Digest); e != nil {
				utils.ErrorHandler(l, span, e, "failed to fetch layer by digest")
				return
			} else if found {
				// Redirct allows BLOB fetch (registry.gitlab.com does not)
				if h.mag.AllowsUnauthorizedPull(lyrUp) {
					utils.Debug(l, span, "upstream redirect found: %s (reqDig: %s / upstreamDig: %s)", lyrUp.GetLabel(), utils.ShortenDigest(req.Digest), utils.ShortenDigest(lyrUp.Digest.String()))
					w.Header().Set("Docker-Content-Digest", req.Digest)
					redUrl := lyrUp.GetBlobUrl(req.Digest)
					utils.Debug(l, span, "redirecting to upstream: %s", redUrl)
					w.Header().Set("Location", redUrl)
					w.Header().Del("Authorization")
					http.Redirect(w, r, redUrl, http.StatusTemporaryRedirect)
					return
				} else {
					utils.Warn(l, span, "upstream does not allow unauthorized pull: %s", lyrUp.GetLabel())
				}
			} else {
				utils.Debug(l, span, "upstream redirect not found: %s (digest: %s)", up.GetLabel(), utils.ShortenDigest(req.Digest))
			}
			if h.mag.Minio.GetLayerBucketConfig().Redirect {
				utils.Info(l, span, "redirecting to MinIO/S3 presigned URL")
				psUrl, err := h.mag.Layer_GetPresignedUrlByHTTPRequest(r)
				if err != nil {
					det := helper.NewDetailFromReq(req)
					e := helper.NewErrPayload("INTERNAL_ERROR", "failed to fetch layer presigned url", det)
					helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
				} else {
					tracing.SpanLog(span, "debug", "307 temporariy redirect blob from '%s' to MinIO/S3 presigend URL: %s", req.Repo, psUrl)
					span.SetStatus(http.StatusTemporaryRedirect, "temporariy redirect blob to MinIO/S3 presigend URL")
					http.Redirect(w, r, psUrl, http.StatusTemporaryRedirect)
					return
				}
			}
		}
		layer, found, err := h.mag.Layer_GetByHTTPRequest(r)
		if err != nil {
			// 1. FALSE
			msg := fmt.Sprintf("failed to fetch layer: %v", err)
			logrus.Error(msg)
			msg = "failed to fetch layer blob"
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			span.SetStatus(http.StatusInternalServerError, e.Message)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		if found {
			if r.Method == "GET" {
				layer.WriteWWW(w)
			}
			layer.WriteWWWHeader(w)
			return
		}
	case
		"application/vnd.oci.image.config.v1+json",
		"application/vnd.docker.container.image.v1+json",
		"application/vnd.unknownConfig",
		"unknownConfig":
		cfg, found, err := h.mag.ConfigBlob_GetByHTTPRequest(ctx, r)
		if err != nil {
			// 1. FALSE
			msg := fmt.Sprintf("failed to fetch config: %v", err)
			logrus.Error(msg)
			msg = "failed to fetch config blob"
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		if found {
			if r.Method == "GET" {
				cfg.WriteWWW(w)
			} else {
				cfg.WriteWWWHeader(w)
			}
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
	// Not found
	msg = "could not find blob"
	det := helper.NewDetailFromReq(req)
	e := helper.NewErrPayload("NAME_UNKNOWN", msg, det)
	span.SetStatus(http.StatusNotFound, e.Message)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
}

func (h Handlers) BlobDelHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobDelHandler",
		"method":  r.Method,
		"repo":    req.Repo,
		"uuid":    req.UUID,
		"user":    req.User,
		"role":    req.UserRole,
	})
	defer span.End()
	_, e := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		utils.ErrorHandler_WithWWW(w, l, span, req.Repo, "push", e, "DENIED", "failed to authorize profile '%s'", req.Profile)
		return
	}
	found, err := h.mag.BlobDelete(ctx, req)
	if err != nil {
		utils.ErrorHandler_WithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to delete blob")
		return
	}
	if !found {
		e := fmt.Errorf(types.FailBlobNotFound)
		utils.ErrorHandler_WithWWW(w, l, span, req.Repo, "push", e, "MANIFEST_UNKNOWN", "")
		return
	}
	span.SetStatus(http.StatusAccepted, "")
	w.WriteHeader(http.StatusAccepted)
}
