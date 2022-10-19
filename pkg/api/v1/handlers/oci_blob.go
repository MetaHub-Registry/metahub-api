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
		layer.WriteWWWHeader(w)
		layer.WriteWWWBody(w)
		return
	}
	// Not found
	msg = "could not find blob"
	e := helper.NewErrPayload("NAME_UNKNOWN", msg, msg)
	helper.WriteOCIErrror(w, http.StatusNotFound, e)
}
