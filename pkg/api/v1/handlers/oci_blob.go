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
	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-api/pkg/pullthrough"
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
	dockerContentDigestKey  = "Docker-Content-Digest"
	FailedAuthPushPrfFMT    = "failed to authorize push for profile '%s'"
)

// legacyPushAuthz applies the pre-namespace push rule, and reports nothing when
// the namespace-authz matrix is authoritative.
//
// ProfileAuthorizedPushHandler applies a much cruder rule than canPush(): a
// non-admin may only push to a repo whose path starts with their own USERNAME.
// oci_manifest.go already skipped it under enforce_write_ownership (93878c2),
// but every blob handler here still called it unconditionally -- and
// POST /blobs/uploads/ is the first call of any push carrying new content. A
// project admin whom OCIWriteAuthMiddleware had just authorised via
// mh-<org>-<project>-admin was therefore refused immediately after, so the
// namespace matrix could never actually grant a non-admin a push.
//
// It went unnoticed because re-pushing an image whose blobs already exist skips
// the uploads entirely and only PUTs the manifest -- the one path that was
// gated. The per-user usage e2e (alice pushes, then bob pushes THE SAME image)
// passed for exactly that reason.
//
// Returning an error rather than writing the response keeps each call site's
// existing single-branch handling, which differs between them (some write an
// OCI payload directly, others go through ErrorHandlerWithWWW).
func (h Handlers) legacyPushAuthz(ctx context.Context, req types.Request) error {
	if h.registry.EnforceWriteOwnership {
		return nil
	}
	_, err := h.mag.Database.ProfileAuthorizedPushHandler(ctx, req)
	return err
}

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
	if e := h.legacyPushAuthz(ctx, req); e != nil {
		err := errors.Wrapf(e, FailedAuthPushPrfFMT, req.Profile)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		payload := helper.NewErrPayload("DENIED", e.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, payload)
		return
	}
	bu, found, err := h.mag.Database.BlobUploadGetOCIByUUID(ctx, req.UUID)
	if err != nil {
		msg := BlobUploadFetchErr
		err = errors.Wrap(err, msg)
		logrus.Error(err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
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
	if h.finalizeUploadBody(ctx, w, r, req, &bu) {
		return
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

	// The finalize PUT carries the whole-blob digest as ?digest=; use it as the
	// composed blob's identity (bu.Digest otherwise holds the last chunk's digest,
	// which is wrong for a multi-chunk upload).
	if req.Digest != "" {
		bu.Digest = req.Digest
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
		attribute.String(dockerContentDigestKey, bu.Digest),
		attribute.String(ContentLength, fmt.Sprintf("%d", len(bu.Data))),
	))
	span.SetStatus(http.StatusAccepted, "Accepted")
	bu.WriteOCIPutResponse(w)
}

// writeBlobInternalError logs err and writes a 500 OCI INTERNAL_ERROR response.
func (h Handlers) writeBlobInternalError(w http.ResponseWriter, req types.Request, err error, msg string) {
	logrus.Error(err)
	det := helper.NewDetailFromReq(req)
	helper.WriteOCIErrror(w, http.StatusInternalServerError, helper.NewErrPayload("INTERNAL_ERROR", msg, det))
}

// finalizeUploadBody records + stores the final chunk of a PUT that carries a
// body (req.Length>0). A monolithic PUT that carries the whole-blob digest in
// ?digest= and has no prior PATCH chunks STREAMs straight to MinIO at constant
// memory (never io.ReadAll, which OOMs on 10-100 GB model layers); chunked /
// no-digest uploads take the buffered path. A req.Length of 0 is a no-op.
// Returns true when it has already written an OCI error response (caller returns).
func (h Handlers) finalizeUploadBody(ctx context.Context, w http.ResponseWriter, r *http.Request, req types.Request, bu *types.OCIBlobUpload) bool {
	if req.Length == 0 {
		return false
	}
	prior, e := h.mag.Database.UploadChunkGetByUUID(ctx, bu.ID)
	if e != nil {
		h.writeBlobInternalError(w, req, errors.Wrap(e, BlobUploadFetchErr), BlobUploadFetchErr)
		return true
	}
	var buf []byte
	stream := req.Digest != "" && len(prior) == 0
	if stream {
		bu.Digest = req.Digest
	} else {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			h.writeBlobInternalError(w, req, errors.Wrap(err, "while reading request body"), "failed to read request body")
			return true
		}
		buf = b
		bu.Digest = digest.FromBytes(buf).String()
	}
	logrus.Infof("completing upload UUID:%s (%d bytes, streaming=%t)", req.UUID, req.Length, stream)
	if _, e := h.mag.Database.UploadChunkCreate(ctx, bu, 0, int64(req.Length)); e != nil {
		h.writeBlobInternalError(w, req, errors.Wrap(e, ChunkUploadErr), ChunkUploadErr)
		return true
	}
	var err error
	if stream {
		err = h.mag.StoreBlobUploadReader(ctx, bu, r.Body, int64(req.Length))
	} else {
		err = h.mag.StoreBlobUpload(ctx, bu, buf)
	}
	if err != nil {
		h.writeBlobInternalError(w, req, errors.Wrap(err, failedToStoreBlobUpload), failedToStoreBlobUpload)
		return true
	}
	if err := h.mag.Database.BlobUploadUpdateByUUID(ctx, bu); err != nil {
		h.writeBlobInternalError(w, req, errors.Wrap(err, "failed to update blob upload"), "failed to update blob upload")
		return true
	}
	return false
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
		// lastChunk.Bytes stores the inclusive END offset (Content-Range
		// "start-end" → RangeBytes = end, see request.ParseRange), so the next
		// chunk must start at END+1. Adding StartByte here double-counted it and
		// rejected the 3rd+ chunk of any >2-chunk upload with 416 — the 2-chunk
		// tests never exercised a last chunk whose StartByte != 0.
		expectedStart = lastChunk.Bytes + 1
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
	if e := h.legacyPushAuthz(ctx, req); e != nil {
		err := errors.Wrapf(e, FailedAuthPushPrfFMT, req.Profile)
		span.RecordError(err)
		det := helper.NewDetailFromReq(req)
		payload := helper.NewErrPayload("DENIED", e.Error(), det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, payload)
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
	// `e` stays at function scope because later branches still reference it.
	e := h.legacyPushAuthz(ctx, req)
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
		//
		// CAUTION — do NOT naively "return a presigned S3 PUT URL here" as a push
		// offload (mirror of the pull-side presigned GET redirect). Push is not a
		// single request: it is POST -> optional PATCH(Content-Range) -> PUT(?digest=),
		// and it is content-addressed. An S3 presigned PUT is a single fixed-signature
		// object write that cannot carry the ?digest= finalize query (breaks the
		// signature), cannot accept PATCH chunk semantics, and — critically — bypasses
		// the pod so the server can neither verify the digest nor create the :Blob
		// graph node from the bytes. This is why Docker Distribution / Harbor / Zot
		// redirect pulls but stream pushes through the registry. It is also why the
		// in-cluster seed/repackage path (regclient -> api, both pod-side) would NOT
		// benefit — its slow leg is pod->S3 egress either way; the dedup skip in
		// metalib StoreBlobUpload{,Reader}/ComposeFromChunks already removed that
		// path's real waste. The only viable variant (307 the *monolithic* PUT to a
		// presigned S3 PUT + optimistically create the :Blob node from the declared
		// ?digest= + lazily verify via HEAD) trusts client-supplied digests until
		// verified and only helps *remote* large-blob pushers — treat it as a
		// scoped, flag-gated experiment, not a drop-in. See PERF-FINDINGS.md.
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

// tryPullThroughBlobMiss routes a blob GET whose bytes are absent locally to the
// configured upstream, and enqueues an async cache job. Returns true when it has
// written a response (client redirect or proxied bytes); false lets the caller
// follow the existing not-found path (non-alias, non-pull-through, or upstream
// fetch failure).
func (h Handlers) tryPullThroughBlobMiss(ctx context.Context, w http.ResponseWriter, r *http.Request, l *logrus.Entry, span trace.Span, req types.Request) bool {
	if h.pullThrough == nil || r.Method != http.MethodGet {
		return false
	}
	// Resolve the upstream registry from the request repo prefix (D6: no
	// Blob.upstream_registries; the alias carries the registry).
	regURL, strippedRepo, ok := h.mag.ResolveUpstreamRegistry(req.Repo)
	if !ok {
		return false
	}
	pol, ok := h.pullThrough.Policy(regURL)
	if !ok || !pol.Enabled {
		return false
	}
	lyrUp, found, err := h.mag.Database.UpstreamGetByLayerDigest(ctx, req.Digest)
	if err != nil || !found {
		return false
	}
	// Best-effort, non-blocking cache enqueue for the requested digest (D1).
	h.pullThrough.Enqueue(pullthrough.Job{
		Registry:  regURL,
		Repo:      strippedRepo,
		Tag:       lyrUp.Tag,
		Digest:    req.Digest,
		MediaType: lyrUp.MediaType,
	})
	if pol.ClientRedirect {
		redURL := h.mag.GetBlobRedirectURL(lyrUp, req.Digest)
		utils.Info(l, span, "pull-through: 307 %s to upstream %s", utils.ShortenDigest(req.Digest), redURL)
		w.Header().Set(dockerContentDigestKey, req.Digest)
		w.Header().Set(Location, redURL)
		w.Header().Del("Authorization")
		http.Redirect(w, r, redURL, http.StatusTemporaryRedirect)
		return true
	}
	// Proxy the upstream bytes to the client (no tee; the worker caches the
	// requested digest independently at constant memory).
	up := upstream.UpstreamTriple{
		Reg:       lyrUp.Reg,
		Repo:      lyrUp.Repo,
		Tag:       lyrUp.Tag,
		Digest:    digest.Digest(req.Digest),
		MediaType: lyrUp.MediaType,
	}
	rc, _, err := h.mag.GetBlobReaderByUpstreamTripleByDigest(ctx, up, req.Digest)
	if err != nil {
		utils.Warn(l, span, "pull-through proxy: upstream fetch failed for %s: %v", utils.ShortenDigest(req.Digest), err)
		return false
	}
	defer rc.Close()
	utils.Info(l, span, "pull-through: proxying %s from upstream", utils.ShortenDigest(req.Digest))
	w.Header().Set(dockerContentDigestKey, req.Digest)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
	if _, cerr := io.Copy(w, rc); cerr != nil {
		utils.Warn(l, span, "pull-through proxy: stream failed for %s: %v", utils.ShortenDigest(req.Digest), cerr)
	}
	return true
}

// tryBlobGetRedirect attempts upstream or MinIO redirect for a GET blob request.
// Returns true if a redirect was written and the caller should return.
func (h Handlers) tryBlobGetRedirect(ctx context.Context, w http.ResponseWriter, r *http.Request, l *logrus.Entry, span trace.Span, req types.Request, up upstream.UpstreamTriple) bool {
	// Redirect decision is the server-side hot spot on a model pull when S3/MinIO
	// redirect is enabled (dev): the bytes never traverse the API — only the
	// upstream-layer lookup + presigned-URL mint do. Span it so the critical path
	// distinguishes "presign + 307" from the streamed-copy path below.
	ctx, redirSpan := tracer.Start(ctx, "BlobHandler.tryBlobGetRedirect")
	defer redirSpan.End()
	redirSpan.SetAttributes(attribute.Key("blob.digest").String(req.Digest))
	if lyrUp, found, e := h.mag.Database.UpstreamGetByLayerDigest(ctx, req.Digest); e != nil {
		utils.ErrorHandler(l, span, e, "failed to fetch layer by digest")
		return true
	} else if found && h.mag.AllowsUnauthorizedPull(lyrUp) {
		utils.Debug(l, span, "upstream redirect found: %s (reqDig: %s / upstreamDig: %s)", lyrUp.GetLabel(), utils.ShortenDigest(req.Digest), utils.ShortenDigest(lyrUp.Digest.String()))
		redUrl := h.mag.GetBlobRedirectURL(lyrUp, req.Digest)
		utils.Debug(l, span, "redirecting to upstream: %s", redUrl)
		w.Header().Set(dockerContentDigestKey, req.Digest)
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
		redirSpan.SetAttributes(attribute.Key("blob.redirect").String("minio_presign"))
		psCtx, psSpan := tracer.Start(ctx, "BlobHandler.presign")
		psUrl, err := h.mag.LayerGetPresignedUrlByHTTPRequest(psCtx, r)
		psSpan.End()
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
	// Usage: bounded blob-pull metric + detailed VictoriaLogs event.
	defer recordPull(w, r, req, metrics.KindBlob)
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

	// Metadata-only lookup — never buffer the blob here. Model layers can be
	// 10s–100s of GB; fetching bytes eagerly (BlobGetByHTTPRequest buffers on
	// GET) OOMs the pod. The bytes are streamed below at constant memory.
	blob, found, err := h.mag.Database.BlobGetByDigest(ctx, req.Digest)
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
		// Content-addressed fallback: blob metadata lives in the graph, but the
		// authoritative store is the object bucket (blobs are keyed by digest). The
		// e2e `parse rules --empty-graph=…` wipe DETACH DELETEs :Blob nodes while
		// leaving the S3 objects intact, so a graph-miss does NOT mean the blob is
		// gone. Stat the object; when present, synthesise the descriptor and serve
		// it (HEAD 200 / GET stream) so clients don't re-upload/re-send an existing
		// (potentially ~GB) layer. HEAD and GET stay consistent — a client that
		// HEADs 200 must be able to GET the bytes.
		obj, statErr := h.mag.Minio.BlobStatObj(ctx, req.Digest)
		if statErr != nil {
			// Pull-through delivery: a base layer that was never cached still
			// has an upstream source. Route the client (redirect or proxy) and
			// enqueue an async cache job before falling through to the 404.
			if h.tryPullThroughBlobMiss(ctx, w, r, l, span, req) {
				return
			}
			utils.Info(l, span, "blob graph-miss AND object-absent — 404 method=%s repo=%s digest=%s stat_err=%v",
				r.Method, req.Repo, req.Digest, statErr)
			// Not found
			msg = "could not find blob"
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("NAME_UNKNOWN", msg, det)
			span.SetStatus(http.StatusNotFound, e.Message)
			helper.WriteOCIErrror(w, http.StatusNotFound, e)
			return
		}
		utils.Info(l, span, "blob graph-miss but object present — content-addressed serve method=%s repo=%s digest=%s size=%d",
			r.Method, req.Repo, req.Digest, obj.Size)
		blob = &types.Blob{Digest: req.Digest, MediaType: "application/octet-stream", Size: obj.Size}
	} else {
		utils.Debug(l, span, "blob for w/ digest '%s' found in graph", utils.ShortenDigest(req.Digest))
	}
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
	// Anything other than GET reaching here (HEAD returned above) needs only
	// headers, not a body.
	if r.Method != "GET" {
		blob.WriteWWWHeader(w)
		return
	}
	// Stream the blob straight from object storage at constant memory. Never
	// buffer the whole layer (BlobGetBytes) — model layers can be 10s–100s of GB
	// and buffering OOMs the pod. cf. minio.BlobGetReader.
	// Span the object-store open separately so time-to-first-byte (stat + open)
	// is distinguishable from the body transfer below on the pull critical path.
	openCtx, openSpan := tracer.Start(ctx, "BlobHandler.openBlobReader")
	openSpan.SetAttributes(attribute.String("digest", req.Digest))
	rc, err := h.mag.Minio.BlobGetReader(openCtx, req.Digest)
	openSpan.End()
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			msg = "could not find blob"
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("NAME_UNKNOWN", msg, det)
			span.SetStatus(http.StatusNotFound, e.Message)
			helper.WriteOCIErrror(w, http.StatusNotFound, e)
			return
		}
		logrus.Errorf("failed to open blob stream: %v", err)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", "failed to fetch layer blob", det)
		span.SetStatus(http.StatusInternalServerError, e.Message)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	defer rc.Close()
	span.SetStatus(codes.Ok, "streaming blob")
	w.Header().Set(dockerContentDigestKey, blob.Digest)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set(ContentLength, fmt.Sprintf("%d", blob.Size))
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
	// The raw byte transfer — dominant server-side cost when redirect is OFF
	// (the API streams the layer from object storage at constant memory).
	// bytes_streamed lets the trace attribute wall-time to actual throughput.
	_, streamSpan := tracer.Start(ctx, "BlobHandler.streamBlob")
	streamSpan.SetAttributes(
		attribute.Key("blob.digest").String(req.Digest),
		attribute.Key("blob.size").Int64(int64(blob.Size)),
	)
	n, cerr := io.Copy(w, rc)
	streamSpan.SetAttributes(attribute.Key("blob.bytes_streamed").Int64(n))
	if cerr != nil {
		// Headers already sent — can only log a truncated transfer.
		streamSpan.RecordError(cerr)
		utils.Warn(l, span, "failed to stream blob %s: %v", utils.ShortenDigest(req.Digest), cerr)
	}
	streamSpan.End()
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
	// `e` stays at function scope because the delete-failure branch below
	// still references it.
	e := h.legacyPushAuthz(ctx, req)
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
