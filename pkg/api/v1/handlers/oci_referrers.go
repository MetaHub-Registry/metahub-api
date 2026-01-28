package handlers

import (
	"fmt"
	"net/http"

	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameOCIReferrers = "handlers/oci_referrers.go"
)

func (h Handlers) ReferrersGetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := types.NewRequest(ctx, r)
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameOCIReferrers,
		"func":    "ReferrersGetHandler",
		"method":  r.Method,
		"repo":    req.Repo,
		"tag":     req.Tag,
	})
	defer span.End()
	msg := fmt.Sprintf("%-5s ReferrersGetHandler(%s, ", r.Method, req.Repo)
	if req.Tag == "" {
		msg += fmt.Sprintf("%s)", req.Digest)
	} else {
		msg += fmt.Sprintf("%s)", req.Tag)
	}
	utils.Debug(l, span, "%s", msg)
	refs, err := h.mag.Database.ReferrersGetBySubjectDigest(ctx, req.Digest)
	if err != nil {
		msg = fmt.Sprintf("failed to fetch referrers: %v", err)
		utils.ErrorHandler(l, span, err, "failed to fetch referrers")
		det := helper.NewDetailFromReq(req)
		e := helper.NewErrPayload("INTERNAL_ERROR", msg, det)
		helper.WriteOCIErrror(w, http.StatusInternalServerError, e)
		return
	}
	refs.WriteWWW(w)
	w.WriteHeader(http.StatusOK)
	return
}
