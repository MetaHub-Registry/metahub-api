package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_Heartbeat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.HeartbeatHandler(w, req)
	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}
