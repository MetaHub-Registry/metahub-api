package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
