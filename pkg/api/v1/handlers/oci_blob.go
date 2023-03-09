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
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (h Handlers) BlobUploadPutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "GetConfig")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("uuid").String(req.UUID),
	)
	// Complete the upload specified by uuid, optionally appending the body as the final chunk.
	logrus.Infof("%-5s BlobUploadPutHandler(%s, %s)", r.Method, req.Repo, req.UUID)
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
	// Hanlder to upload a blob to the object store
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "BlobUploadPostHandler")
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("length").Int(req.Length),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("uuid").String(req.UUID),
	)
	defer span.End()
	mt := "NONE"
	msg := fmt.Sprintf("%-5s BlobUploadPostHandler(%s, Type:%s, Len:%d", r.Method, mt, req.Repo, req.Length)
	if req.Tag != "" {
		msg = fmt.Sprintf("%s, Tag:%s)", msg, req.Tag)
	} else {
		msg = fmt.Sprintf("%s, Digest:%s)", msg, utils.ShortenDigest(req.Digest))
	}
	if req.UUID != "" {
		msg = fmt.Sprintf("%s, UUID:%s)", msg, req.UUID)
	}
	logrus.Warnf(msg)
	bu := types.OCIBlobUpload{
		UploadUser:    req.User,
		UploadProfile: req.Profile,
		Digest:        req.Digest,
		Repo:          req.Repo,
	}
	err := h.mag.Database.BlobUpload_Create_ByOCI(ctx, &bu)
	if err != nil {
		logrus.Error(err)
		span.RecordError(err)
		msg := "failed to create blob upload"
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	tracing.SpanDebug(span, "request.length:%d", req.Length)
	if req.Length > 0 {
		// Successful completion of the request MUST return a 201 Created and MUST include the following header:
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			err = errors.Wrap(err, "while io.ReadAll(Body)")
			logrus.Error(err)
			return
		}
		dig := digest.FromBytes(buf)
		bu.Digest = dig.String()
		err = h.mag.StoreBlobUpload(ctx, &bu, buf)
		if err != nil {
			logrus.Error(err)
			span.RecordError(err)
			msg := "failed to store blob upload"
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
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
	ctx, span := tracer.Start(ctx, "BlobHandler")
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("tag").String(req.Tag),
	)
	defer span.End()
	msg := fmt.Sprintf("%-5s BlobHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	logrus.Info(msg)
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
	// OK, let's check if it is a layer
	// If redirect is enabled, we need to fetch the PreSignedURL
	if r.Method == "GET" && h.mag.Minio.GetLayerBucketConfig().Redirect {
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
	// Not found
	msg = "could not find blob"
	det := helper.NewDetailFromReq(req)
	e := helper.NewErrPayload("NAME_UNKNOWN", msg, det)
	span.SetStatus(http.StatusNotFound, e.Message)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
}

func (h Handlers) BlobDelHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "BlobDelHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("method").String(r.Method),
		attribute.Key("repo").String(req.Repo),
		attribute.Key("tag").String(req.Tag),
	)

	err := h.mag.BlobDelete(ctx, req)
	if err != nil {
		msg := fmt.Sprintf("failed to delete blob: %v", err)
		logrus.Error(msg)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
		span.SetStatus(http.StatusNotFound, e.Message)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	span.SetStatus(http.StatusAccepted, "")
	w.WriteHeader(http.StatusAccepted)
}
