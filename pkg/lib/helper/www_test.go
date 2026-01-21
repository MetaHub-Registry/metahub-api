package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
)

func TestWriteReponseWithoutContent(t *testing.T) {
	w := httptest.NewRecorder()
	WriteReponseWithoutContent(w, http.StatusNoContent, map[string]string{"X-Test-Header": "test"})
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "", w.Body.String())
	assert.Equal(t, "test", w.Header().Get("X-Test-Header"))
}
