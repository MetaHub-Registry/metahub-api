package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/metahub-registry/metahub-api/pkg/lib/helper"
	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
)

// isQuantInfeasible reports whether err (or a wrapped cause) is the model
// resolver's infeasible-quantization error — the pull was for a quant that does
// not fit the aggregated profile's VRAM capacity.
func isQuantInfeasible(err error) bool {
	var e *mlibTypes.ErrQuantInfeasible
	return errors.As(err, &e)
}

// writeOCIError maps a metalib error to the appropriate OCI distribution-spec
// error envelope and writes it to the response. This centralises the error
// mapping that was previously scattered across individual push/pull handlers.
//
// Because metalib does not yet export sentinel errors (planned for Phase C3),
// we use substring matching against the error message. The canonical markers
// are embedded by metalib's StoreIndexUpload, IndexGetByRequest, etc.
//
// Design reference: D9 in the refactor-handlers-slot-centred-model design.md.
func writeOCIError(w http.ResponseWriter, err error) {
	msg := err.Error()
	var code string
	var status int

	// Model resolution: an infeasible quantization (does not fit the aggregated
	// profile's VRAM capacity) maps to OCI UNSUPPORTED / 400 rather than serve an
	// unrunnable image.
	if isQuantInfeasible(err) {
		det := helper.Detail{}
		e := helper.NewErrPayload("UNSUPPORTED", msg, det)
		helper.WriteOCIErrror(w, http.StatusBadRequest, e)
		return
	}

	switch {
	case strings.Contains(msg, "MANIFEST_BLOB_UNKNOWN"):
		code = "MANIFEST_BLOB_UNKNOWN"
		status = http.StatusBadRequest
	case strings.Contains(msg, "MANIFEST_INVALID"):
		code = "MANIFEST_INVALID"
		status = http.StatusBadRequest
	case strings.Contains(msg, "MANIFEST_UNKNOWN") || strings.Contains(msg, "manifest not found"):
		code = "MANIFEST_UNKNOWN"
		status = http.StatusNotFound
	case strings.Contains(msg, "DENIED") || strings.Contains(msg, "not authorized"):
		code = "DENIED"
		status = http.StatusForbidden
	default:
		code = "INTERNAL_ERROR"
		status = http.StatusInternalServerError
	}

	det := helper.Detail{}
	e := helper.NewErrPayload(code, msg, det)
	helper.WriteOCIErrror(w, status, e)
}
