package handlers

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (h Handlers) BlobGetHandler(w http.ResponseWriter, r *http.Request) {
	req := types.NewRequest(r)
	msg := fmt.Sprintf("%-5s BlobGetHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	logrus.Info(msg)
	cfg, found, err := h.mag.ConfigBlob_GetByHTTPRequest(r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch config: %v", err)
		logrus.Error(msg)
		msg = "failed to fetch config blob"
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, msg)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if found {
		cfg.WriteWWWHeader(w)
		cfg.WriteWWWBody(w)
		return
	}
	// OK, let's check if it is a layer
	layer, found, err := h.mag.Layer_GetByHTTPRequest(r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch layer: %v", err)
		logrus.Error(msg)
		msg = "failed to fetch layer blob"
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, msg)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if found {
		/*
			// Get Presigned URL
			if svcConfig.Storage.RedirectBlob {
				psUrl, err := storage.GetPresignedUrl(digest.String())
				if err == nil {
					l.Debugf("307 temporariy redirect blob from '%s' to MinIO/S3 presigend URL", repository)
					http.Redirect(w, r, psUrl, http.StatusTemporaryRedirect)
					MetaHubBlobRedirectBytes.With(prometheus.Labels{"repo": repository, "digest": utils.ShortenDigest(digest.String())}).Add(float64(blob.Size))
					return
				}
			}
		*/
		layer.WriteWWWHeader(w)
		layer.WriteWWWBody(w)
		return
	}
	// Not found
	msg = "could not find blob"
	e := helper.NewErrPayload("NAME_UNKNOWN", msg, msg)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
}

/*
func (db NeoMinDatabase) GetPresignedUrl(digest string) (urlStr string, err error) {
	l := log.WithFields(log.Fields{
		"package": "neomin",
		"db":      "minio",
		"func":    "GetPresignedUrl",
		"digest":  utils.ShortenDigest(digest),
	})
	mediaType, err := db.GetMediaTypeByDigest(digest)
	if err != nil {
		return
	}
	switch mediaType {
	case "image.rootfs.diff.tar.gzip":
	default:
		err = fmt.Errorf("mediaType '%s' not supposed to be stored in the blobs bucket. Only 'image.rootfs.diff.tar.gzip'", mediaType)
		return
	}
	// Set request parameters for content-disposition.
	reqParams := make(url.Values)

	// Generates a presigned url which expires in a day.
	psUrl, err := db.layerClient.PresignedGetObject(
		context.Background(),
		db.MinioConfig.GetBucketName("layers"),
		digest,
		time.Second*30*60,
		reqParams)
	if err != nil {
		fmt.Println(err)
		return
	}
	l.Debug("Successfully generated presigned URL")
	urlStr = psUrl.String()
	return
}
*/
