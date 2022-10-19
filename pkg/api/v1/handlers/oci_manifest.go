package handlers

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (h Handlers) ManifestHeadHandler(w http.ResponseWriter, r *http.Request) {
	// Check whether it is a manifest or manifest list
	req := types.NewRequest(r)
	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		logrus.Error(msg)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, msg)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if found {
		ml.WriteWWWHeader(w)
		return
	}
	imgName := fmt.Sprintf("%s/%s", req.Repo, req.Tag)
	msg := fmt.Sprintf("failed to fetch ManifestList for collection '%s'", imgName)
	logrus.Error(msg)
	e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, msg)
	helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
}

func (h Handlers) ManifestGetHandler(w http.ResponseWriter, r *http.Request) {
	ml, found, err := h.mag.ManifestList_GetByHTTPRequest(r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		logrus.Error(msg)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, msg)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	if found {
		ml.WriteWWWHeader(w)
		ml.WriteWWWBody(w)
		return
	}
	m, found, err := h.mag.Manifest_GetByHTTPRequest(r)
	if err != nil {
		// 1. FALSE
		msg := fmt.Sprintf("failed to fetch ManifestList: %v", err)
		logrus.Error(msg)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, msg)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
		return
	}
	if found {
		m.WriteWWWHeader(w)
		m.WriteWWWBody(w)
		return
	} else {
		msg := "no such manifest"
		e := helper.NewErrPayload("MANIFEST_UNKNOWN", msg, msg)
		helper.WriteOCIErrror(w, http.StatusNotFound, e)
	}
}
