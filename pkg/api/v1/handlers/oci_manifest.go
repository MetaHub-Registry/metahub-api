package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (h Handlers) ManifestHeadHandler(w http.ResponseWriter, r *http.Request) {
	// Check whether it is a manifest or manifest list
	req := types.NewRequest(r)
	m, found, err := h.mag.Manifest_GetByHTTPRequest(r)
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
	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(r)
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
	req := types.NewRequest(r)
	err := h.mag.Manifest_DeleteByHTTPRequest(r)
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
	req := types.NewRequest(r)
	m, found, err := h.mag.Manifest_GetByHTTPRequest(r)
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
		m.WriteWWWHeader(w)
		m.WriteWWWBody(w)
		return
	}
	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(r)
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
		ml.WriteWWWBody(w)
		ml.WriteWWWHeader(w)

		return
	} else {
		msg := fmt.Sprintf("no manifest '%s:%s' found for profile '%s'", req.Repo, req.Tag, req.Profile)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	}
	/*m, found, err = h.mag.Manifest_GetByHTTPRequest(r)
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
		m.WriteWWWBody(w)
		m.WriteWWWHeader(w)
		return
	} else {
		msg := fmt.Sprintf("no manifest '%s:%s' found for profile '%s'", req.Repo, req.Tag, req.Profile)
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, det)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	} */
}

func (h Handlers) ManifestPutHandler(w http.ResponseWriter, r *http.Request) {
	req := types.NewRequest(r)
	logrus.Infof("%-5s ManifestPutHandler(%s, %s)", r.Method, req.Repo, req.Tag)
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Fatal("request", err)
	}
	switch req.ContentType {
	case "application/vnd.docker.distribution.manifest.v2+json":
		// Put manifest in object store
		m, err := h.mag.StoreManifestUpload(req, buf)
		if err != nil {
			msg := fmt.Sprintf("failed to store manifest: %v", err)
			logrus.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("MANIFEST_INVALID", msg, det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		// Put maniest in database
		m.WriteWWWHeader(w)
		w.Header().Set("Location", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", req.Protocol, req.Host, req.Repo, req.Tag))
		w.WriteHeader(http.StatusCreated)
	case "application/vnd.oci.image.manifest.v1+json":
		// Put manifest in object store
		m, err := h.mag.StoreManifestUpload(req, buf)
		if err != nil {
			msg := fmt.Sprintf("failed to store manifest: %v", err)
			logrus.Error(msg)
			det := helper.NewDetailFromReq(req)
			e := helper.NewErrPayload("MANIFEST_INVALID", msg, det)
			helper.WriteOCIErrror(w, http.StatusBadRequest, e)
			return
		}
		// Put maniest in database
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
