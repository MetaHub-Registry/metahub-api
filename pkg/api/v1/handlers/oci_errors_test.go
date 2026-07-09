package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mlibTypes "github.com/metahub-registry/metahub-core/pkg/types"
)

// ociErrorEnvelope mirrors the OCI error response shape for test assertions.
type ociErrorEnvelope struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func TestWriteOCIError_SentinelMappings(t *testing.T) {
	tests := []struct {
		name       string
		errMsg     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "MANIFEST_BLOB_UNKNOWN marker",
			errMsg:     "Manifest not found by digest: sha256:abc123 (MANIFEST_BLOB_UNKNOWN)",
			wantStatus: http.StatusBadRequest,
			wantCode:   "MANIFEST_BLOB_UNKNOWN",
		},
		{
			name:       "MANIFEST_INVALID marker",
			errMsg:     "failed to parse index: MANIFEST_INVALID",
			wantStatus: http.StatusBadRequest,
			wantCode:   "MANIFEST_INVALID",
		},
		{
			name:       "MANIFEST_UNKNOWN marker",
			errMsg:     "no manifest found: MANIFEST_UNKNOWN",
			wantStatus: http.StatusNotFound,
			wantCode:   "MANIFEST_UNKNOWN",
		},
		{
			name:       "manifest not found (metalib constant)",
			errMsg:     "manifest not found",
			wantStatus: http.StatusNotFound,
			wantCode:   "MANIFEST_UNKNOWN",
		},
		{
			name:       "DENIED marker",
			errMsg:     "DENIED: push access denied",
			wantStatus: http.StatusForbidden,
			wantCode:   "DENIED",
		},
		{
			name:       "not authorized substring",
			errMsg:     "user is not authorized to push",
			wantStatus: http.StatusForbidden,
			wantCode:   "DENIED",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeOCIError(w, errors.New(tc.errMsg))

			assert.Equal(t, tc.wantStatus, w.Code, "unexpected HTTP status")

			var env ociErrorEnvelope
			err := json.Unmarshal(w.Body.Bytes(), &env)
			require.NoError(t, err, "response body must be valid JSON")
			require.Len(t, env.Errors, 1, "expected exactly one error in envelope")
			assert.Equal(t, tc.wantCode, env.Errors[0].Code, "unexpected OCI error code")
			assert.Contains(t, env.Errors[0].Message, tc.errMsg, "error message should be preserved")
		})
	}
}

// TestWriteOCIError_QuantInfeasible verifies the model resolver's typed
// *ErrQuantInfeasible maps to OCI UNSUPPORTED / 400, both directly and when
// wrapped, with the quant + profile preserved in the message.
func TestWriteOCIError_QuantInfeasible(t *testing.T) {
	base := &mlibTypes.ErrQuantInfeasible{
		Quant: "q8", Profile: "12gb#strixhalo", Required: 34359738368, Available: 12884901888,
	}
	for _, err := range []error{error(base), fmt.Errorf("resolve failed: %w", base)} {
		w := httptest.NewRecorder()
		writeOCIError(w, err)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var env ociErrorEnvelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		require.Len(t, env.Errors, 1)
		assert.Equal(t, "UNSUPPORTED", env.Errors[0].Code)
		assert.Contains(t, env.Errors[0].Message, "q8")
		assert.Contains(t, env.Errors[0].Message, "12gb#strixhalo")
	}
}

func TestWriteOCIError_UnknownErrorMapsToInternal(t *testing.T) {
	w := httptest.NewRecorder()
	writeOCIError(w, errors.New("some unexpected database timeout"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var env ociErrorEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &env)
	require.NoError(t, err)
	require.Len(t, env.Errors, 1)
	assert.Equal(t, "INTERNAL_ERROR", env.Errors[0].Code)
}
