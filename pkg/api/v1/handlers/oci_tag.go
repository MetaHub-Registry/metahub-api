package handlers

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-api/pkg/metrics"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgFileOciTagGo = "handlers/oci_tag.go"
)

func (h Handlers) TagGetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer func() { metrics.ObserveTagsList(metrics.StatusOf(w)) }()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgFileOciTagGo,
		"func":    "TagGetHandler",
		"repo":    req.Repo,
	})
	defer span.End()
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
	utils.Debug(l, span, "Found %d tags", len(res.Tags))
	b, err := res.ToBytes()
	if err != nil {
		msg := fmt.Sprintf("failed to marshal tags: %v", err)
		logrus.Error(msg)
		msg = "failed to marshal tags"
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	utils.Debug(l, span, "%s", string(b))
	res.WriteWWW(w)
}
