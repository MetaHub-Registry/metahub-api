package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"

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

const (
	pkgNameOCIBlob          = "handlers/oci_blob.go"
	Range                   = "Range"
	Location                = "Location"
	ContentLength           = "Content-Length"
	ChunkUploadErr          = "failed to create chunk upload"
	BlobUploadFetchErr      = "failed to fetch blob upload"
	failedToStoreBlobUpload = "failed to store blob upload"
	logSeverityKey          = "log.severity"
	dockerUploadUUIDKey     = "Docker-Upload-UUID"
	FailedAuthPushPrfFMT    = "failed to authorize push for profile '%s'"
)

func (h Handlers) BlobUploadPutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobUploadPutHandler",
		"method":  r.Method,
	})
	defer span.End()
	_ = l
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
		err := errors.Wrapf(e, FailedAuthPushPrfFMT, req.Profile)
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
	bu, found, err := h.mag.Database.BlobUploadGetOCIByUUID(ctx, req.UUID)
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
		logrus.Infof("completing upload UUID:%s with final chunk of size %d", req.UUID, len(buf))
		// We do not have a PATH but directly a PUT request with the data
		dig := digest.FromBytes(buf)
		bu.Digest = dig.String()
		/// Create New Chunk
		_, err := h.mag.Database.UploadChunkCreate(ctx, &bu, 0, int64(req.Length))
		if err != nil {
			msg := ChunkUploadErr
			err = errors.Wrap(err, msg)
			logrus.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		err = h.mag.StoreBlobUpload(ctx, &bu, buf)
		if err != nil {
			err = errors.Wrap(err, failedToStoreBlobUpload)
			logrus.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", failedToStoreBlobUpload, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		err = h.mag.Database.BlobUploadUpdateByUUID(ctx, &bu)
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
	// How many chunks do we have?
	chunks, err := h.mag.Database.UploadChunkGetByUUID(ctx, bu.ID)
	if err != nil {
		msg := BlobUploadFetchErr
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}

	err = h.mag.ComposeFromChunks(ctx, &bu, chunks)
	if err != nil {
		msg := "failed to compose blob from chunks"
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}

	// Finish the upload
	err = h.mag.Database.BlobUploadComplete(ctx, &bu)
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
		attribute.String(logSeverityKey, "debug"),
		attribute.String(Location, bu.Location),
		attribute.String(Range, fmt.Sprintf("%d-%d", bu.RangeStart, bu.RangeStop)),
		attribute.String(dockerUploadUUIDKey, bu.UUID.String()),
		attribute.String("Docker-Content-Digest", bu.Digest),
		attribute.String(ContentLength, fmt.Sprintf("%d", len(bu.Data))),
	))
	span.SetStatus(http.StatusAccepted, "Accepted")
	bu.WriteOCIPutResponse(w)
}

func (h Handlers) BlobUploadGetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobUploadGetHandler",
		"method":  r.Method,
	})
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("length").Int(req.Length),
		attribute.Key("uuid").String(req.UUID),
	)
	utils.Debug(l, span, "%-5s BlobUploadGetHandler(%s, %s) Len:%d", r.Method, req.Repo, req.UUID, req.Length)
	bu, found, err := h.mag.Database.BlobUploadGetByLocation(ctx, r.URL.Path)
	if err != nil || !found {
		if !found {
			err = utils.NewError(l, span, "BlobUpload with Location '%s' not found", r.URL.Path)
		}
		err = utils.ErrorHandler(l, span, err, BlobUploadFetchErr)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("BLOB_UNKNOWN", BlobUploadFetchErr, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	utils.Debug(l, span, "bu: %v", bu)
	chunks, err := h.mag.Database.UploadChunkGetByUUID(ctx, bu.ID)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "failed to fetch blob upload chunks")
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", "failed to fetch blob upload chunks", det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	}
	byteRange := "0-0"
	for idx, chunk := range chunks {
		end := chunk.StartByte + chunk.Bytes
		utils.Debug(l, span, " chunk#%d: %d-%d", idx, chunk.StartByte, end)
		byteRange = fmt.Sprintf("%d-%d", chunk.StartByte, int(end))
	}
	bu.WriteResponse(w, map[string]string{
		Location: bu.Location,
		Range:    byteRange,
	}, http.StatusNoContent)
}

// validateChunkSequence checks for duplicate chunks and validates range ordering.
// Returns expectedStart and stop=true if the caller should abort.
func (h Handlers) validateChunkSequence(ctx context.Context, w http.ResponseWriter, l interface{ Debugf(string, ...interface{}) }, span trace.Span, bu *types.OCIBlobUpload, req types.Request) (expectedStart int64, stop bool) {
	// Was the chunk already uploaded?
	chunks, err := h.mag.Database.UploadChunkGetByBU(ctx, *bu)
	if err != nil {
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", "failed to fetch blob upload chunks by digest", det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	}
	if len(chunks) > 0 {
		lastChunk := chunks[len(chunks)-1]
		if lastChunk.Digest == bu.Digest && lastChunk.StartByte == req.RangeStartByte {
			span.SetStatus(http.StatusRequestedRangeNotSatisfiable, "RangeNotSatisfiable")
			bu.WriteOCIPutResponse416(w)
		}
		return 0, true
	}
	// Fetch all chunks already uploaded to determine expected next start
	chunks, err = h.mag.Database.UploadChunkGetByUUID(ctx, bu.ID)
	if err != nil {
		msg := BlobUploadFetchErr
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return 0, true
	}
	switch {
	case len(chunks) == 0 && req.RangeStartByte != -1 && req.RangeStartByte != 0:
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", "Range should start at 0", det)
		helper.WriteOCIErrror(w, http.StatusRequestedRangeNotSatisfiable, e)
	case len(chunks) > 0:
		lastChunk := chunks[len(chunks)-1]
		expectedStart = lastChunk.StartByte + lastChunk.Bytes + 1
		if req.RangeStartByte != expectedStart {
			msg := fmt.Sprintf("Range should start at %d", expectedStart)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusRequestedRangeNotSatisfiable, e)
		}
	}
	return expectedStart, false
}

func (h Handlers) BlobUploadPatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobUploadPatchHandler",
		"method":  r.Method,
	})
	defer span.End()
	req := types.NewRequest(ctx, r)
	span.SetAttributes(
		attribute.Key("length").Int(req.Length),
		attribute.Key("Range").String(req.Range),
		attribute.Key("ContentType").String(req.ContentType),
		attribute.Key("uuid").String(req.UUID),
	)
	logrus.Infof("%-5s BlobUploadPatchHandler(%s, %s) Len:%d", r.Method, req.Repo, req.UUID, req.Length)
	_, e := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	if e != nil {
		err := errors.Wrapf(e, FailedAuthPushPrfFMT, req.Profile)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("DENIED", e.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}
	dumpedReq, _ := httputil.DumpRequest(r, false)
	utils.Debug(l, span, "Request Dump:\n%s", string(dumpedReq))
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatal("request", err)
	}
	dig := digest.FromBytes(buf)
	utils.Debug(l, span, "Digest from body: %s / len:%d (req:%d)", utils.ShortenDigest(dig.String()), len(buf), req.Length)
	if req.Length != len(buf) {
		utils.Warn(l, span, "Content-Length mismatch: Header:%d / Body:%d", req.Length, len(buf))
		req.Length = len(buf)
	}
	bu, found, err := h.mag.Database.BlobUploadGetOCIByUUID(ctx, req.UUID)
	if err != nil || !found {
		msg := BlobUploadFetchErr
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	utils.Debug(l, span, "req.Length:%d", req.Length)
	bu.RangeStop = bu.RangeStart + int64(req.Length) - 1
	bu.Digest = dig.String()
	// Validate chunk ordering and get expected start offset
	expectedStart, stop := h.validateChunkSequence(ctx, w, l, span, &bu, req)
	if stop {
		return
	}
	/// Create New Chunk
	chunk, err := h.mag.Database.UploadChunkCreate(ctx, &bu, req.RangeStartByte, req.RangeBytes)
	if err != nil {
		msg := ChunkUploadErr
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	_ = chunk
	err = h.mag.StoreBlobUpload(ctx, &bu, buf)
	if err != nil {
		err = errors.Wrap(err, failedToStoreBlobUpload)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", failedToStoreBlobUpload, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	err = h.mag.Database.BlobUploadUpdateByUUID(ctx, &bu)
	if err != nil {
		err = errors.Wrap(err, failedToStoreBlobUpload)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", failedToStoreBlobUpload, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	span.AddEvent("log", trace.WithAttributes(
		attribute.String(logSeverityKey, "debug"),
		attribute.String(Location, bu.Location),
		attribute.String(Range, fmt.Sprintf("%d-%d", req.RangeStartByte, req.RangeBytes+req.RangeStartByte-1)),
		attribute.String(dockerUploadUUIDKey, bu.UUID.String()),
		attribute.String(ContentLength, fmt.Sprintf("%d", len(bu.Data))),
	))
	span.SetStatus(http.StatusAccepted, "Accepted")
	cntLen := fmt.Sprintf("%d", len(bu.Data))
	utils.Debug(l, span, "RangeStart:%d / RangeStop:%d", bu.RangeStart, bu.RangeStop)
	var cntRange string
	if expectedStart > 0 {
		cntRange = fmt.Sprintf("0-%d", bu.RangeStop+expectedStart)
	} else {
		cntRange = fmt.Sprintf("%d-%d", bu.RangeStart, bu.RangeStop)
	}
	utils.Debug(l, span, "Responding with Location:%s / Content-Length:%s / Range:%s", bu.Location, cntLen, cntRange)
	bu.WriteResponse(w, map[string]string{
		Location:      bu.Location,
		ContentLength: cntLen,
		Range:         cntRange,
	}, http.StatusAccepted)
}

func (h Handlers) BlobUploadPostHandler(w http.ResponseWriter, r *http.Request) {
	// POST blob update can either only initiate an upload or include the data in one go
	// > if digest=ASDASD is included, the data is added
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
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "DENIED", FailedAuthPushPrfFMT, req.Profile)
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
		//Digest:        req.Digest,
		Repo: req.Repo,
	}
	err := h.mag.Database.BlobUploadCreateByOCI(ctx, &bu)
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to create blob upload")
		return
	}
	bu.Digest = req.Digest
	utils.Debug(l, span, "request.length:%d", req.Length)
	if req.Length > 0 {
		// Successful completion of the request MUST return a 201 Created and MUST include the following header:
		utils.Debug(l, span, "Read body")
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			err = errors.Wrap(err, "while io.ReadAll(Body)")
			l.Error(err)
			return
		}
		dig := digest.FromBytes(buf)
		bu.Digest = dig.String()
		utils.Debug(l, span, "Digest from body: %s", utils.ShortenDigest(dig.String()))
		/// Create New Chunk
		chunk, err := h.mag.Database.UploadChunkCreate(ctx, &bu, 0, int64(req.Length))
		if err != nil {
			msg := ChunkUploadErr
			err = errors.Wrap(err, msg)
			logrus.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}
		err = h.mag.StoreBlobUpload(ctx, &bu, buf)
		if err != nil {
			utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to store blob upload")
			return
		}
		err = h.mag.ComposeFromChunks(ctx, &bu, []types.BlobChunk{chunk})
		if err != nil {
			msg := "failed to compose blob from chunks"
			err = errors.Wrap(err, msg)
			logrus.Error(err)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
			return
		}

		// Finish the upload
		err = h.mag.Database.BlobUploadComplete(ctx, &bu)
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
			attribute.String(logSeverityKey, "debug"),
			attribute.String(Location, bu.Location),
			attribute.String(dockerUploadUUIDKey, bu.UUID.String()),
			attribute.String(ContentLength, fmt.Sprintf("%d", len(bu.Data))),
		))
		span.SetStatus(http.StatusCreated, "Created")
		bu.WriteOCICreateResponse(w)
	} else {
		// Registries that do not support single request monolithic uploads SHOULD return
		// a 202 Accepted status code and Location header and clients SHOULD proceed with a subsequent PUT request
		// TODO: Send presigned upload URL
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(logSeverityKey, "debug"),
			attribute.String(Location, bu.Location),
			attribute.String(dockerUploadUUIDKey, bu.UUID.String()),
			attribute.String(ContentLength, fmt.Sprintf("%d", len(bu.Data))),
		))
		span.SetStatus(http.StatusAccepted, "Accepted")
		bu.WriteOCIAcceptedResponse(w)
	}
}

// tryBlobGetRedirect attempts upstream or MinIO redirect for a GET blob request.
// Returns true if a redirect was written and the caller should return.
func (h Handlers) tryBlobGetRedirect(ctx context.Context, w http.ResponseWriter, r *http.Request, l *logrus.Entry, span trace.Span, req types.Request, up upstream.UpstreamTriple) bool {
	if lyrUp, found, e := h.mag.Database.UpstreamGetByLayerDigest(ctx, req.Digest); e != nil {
		utils.ErrorHandler(l, span, e, "failed to fetch layer by digest")
		return true
	} else if found && h.mag.AllowsUnauthorizedPull(lyrUp) {
		utils.Debug(l, span, "upstream redirect found: %s (reqDig: %s / upstreamDig: %s)", lyrUp.GetLabel(), utils.ShortenDigest(req.Digest), utils.ShortenDigest(lyrUp.Digest.String()))
		redUrl := lyrUp.GetBlobUrl(req.Digest)
		utils.Debug(l, span, "redirecting to upstream: %s", redUrl)
		w.Header().Set("Docker-Content-Digest", req.Digest)
		w.Header().Set(Location, redUrl)
		w.Header().Del("Authorization")
		http.Redirect(w, r, redUrl, http.StatusTemporaryRedirect)
		return true
	} else if found {
		utils.Warn(l, span, "upstream does not allow unauthorized pull: %s", lyrUp.GetLabel())
	} else {
		utils.Debug(l, span, "upstream redirect not found: %s (digest: %s)", up.GetLabel(), utils.ShortenDigest(req.Digest))
	}
	if h.mag.Minio.GetBlobBucketConfig().Redirect {
		utils.Info(l, span, "redirecting to MinIO/S3 presigned URL")
		psUrl, err := h.mag.LayerGetPresignedUrlByHTTPRequest(r)
		if err != nil {
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("INTERNAL_ERROR", "failed to fetch layer presigned url", det)
			helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		} else {
			span.SetStatus(http.StatusTemporaryRedirect, "temporary redirect blob to MinIO/S3 presigned URL")
			http.Redirect(w, r, psUrl, http.StatusTemporaryRedirect)
			return true
		}
	}
	return false
}

func (h Handlers) BlobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIBlob,
		"func":    "BlobHandler",
		"method":  r.Method,
	})
	defer span.End()
	req := types.NewRequest(ctx, r)
	msg := fmt.Sprintf("%-5s BlobHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	utils.Debug(l, span, "%s", msg)
	// Check for blob in Graph
	up := upstream.UpstreamTriple{
		Reg:    req.Host,
		Repo:   req.Repo,
		Tag:    req.Tag,
		Digest: digest.Digest(req.Digest),
	}

	blob, found, err := h.mag.BlobGetByHTTPRequest(ctx, r)
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
	if !found {
		utils.Debug(l, span, "blob for w/ digest '%s' not found", utils.ShortenDigest(req.Digest))
		// Not found
		msg = "could not find blob"
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("NAME_UNKNOWN", msg, det)
		span.SetStatus(http.StatusNotFound, e.Message)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	utils.Debug(l, span, "blob for w/ digest '%s' found", utils.ShortenDigest(req.Digest))
	// If redirect is enabled, we need to fetch the PreSignedURL
	switch r.Method {
	case "HEAD":
		utils.Debug(l, span, "HEAD request for blob")
		blob.WriteWWWHeader(w)
		return
	case "GET":
		utils.Debug(l, span, "GET request for blob")
		if redirected := h.tryBlobGetRedirect(ctx, w, r, l, span, req, up); redirected {
			return
		}
	}
	layer, found, err := h.mag.LayerGetByHTTPRequest(r)
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
		span.SetStatus(codes.Ok, "found blob")
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
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "DENIED", "failed to authorize profile '%s'", req.Profile)
		return
	}
	found, err := h.mag.BlobDelete(ctx, req)
	if err != nil {
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "INTERNAL_ERROR", "failed to delete blob")
		return
	}
	if !found {
		e := fmt.Errorf(types.FailBlobNotFound)
		utils.ErrorHandlerWithWWW(w, l, span, req.Repo, "push", e, "MANIFEST_UNKNOWN", "")
		return
	}
	span.SetStatus(http.StatusAccepted, "")
	w.WriteHeader(http.StatusAccepted)
}
