package handlers

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-api/pkg/types"
)

func (h Handlers) CatalogGetHandler(w http.ResponseWriter, r *http.Request) {
	req := types.NewRequest(r)
	msg := fmt.Sprintf("%-5s CatalogGetHandler(%s)", r.Method, req.Repo)
	logrus.Info(msg)

	res, err := h.mag.Collections_GetCatalog(req)
	if err != nil {
		msg := fmt.Sprintf("failed to fetch catalog: %v", err)
		logrus.Error(msg)
		msg = "failed to fetch tags"
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, msg)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	res.WriteWWWHeader(w)
	res.WriteWWWBody(w)
}
