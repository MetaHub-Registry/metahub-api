package handlers

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (h Handlers) TagGetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "TagGetHandler")
	defer span.End()
	req := types.NewRequest(ctx, r)
	msg := fmt.Sprintf("%-5s TagGetHandler(%s)", r.Method, req.Repo)
	logrus.Info(msg)
	res, err := h.mag.TagsGetList(ctx, req)
	if err != nil {
		msg := fmt.Sprintf("failed to fetch tags: %v", err)
		logrus.Error(msg)
		msg = "failed to fetch tags"
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	res.WriteWWW(w)
}
