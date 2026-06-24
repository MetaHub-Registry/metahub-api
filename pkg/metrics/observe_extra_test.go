package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// statusWriter is a ResponseWriter that exposes Status(), mimicking the chi
// WrapResponseWriter the metrics Middleware installs.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s statusWriter) Status() int { return s.status }

func TestStatusOfWithStatusWriter(t *testing.T) {
	// A writer reporting a concrete status returns it verbatim.
	sw := statusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusNotFound}
	if got := StatusOf(sw); got != http.StatusNotFound {
		t.Fatalf("StatusOf(404 writer) = %d, want 404", got)
	}
	// A writer reporting 0 (nothing written yet) falls back to 200.
	zero := statusWriter{ResponseWriter: httptest.NewRecorder(), status: 0}
	if got := StatusOf(zero); got != http.StatusOK {
		t.Fatalf("StatusOf(0 writer) = %d, want 200", got)
	}
}

func TestStatusLabel(t *testing.T) {
	if got := statusLabel(0); got != "200" {
		t.Fatalf("statusLabel(0) = %q, want \"200\"", got)
	}
	if got := statusLabel(503); got != "503" {
		t.Fatalf("statusLabel(503) = %q, want \"503\"", got)
	}
}

func TestObserveTagsAndCatalogList(t *testing.T) {
	tagsBefore := testutil.ToFloat64(tagsListTotal.WithLabelValues("200"))
	ObserveTagsList(http.StatusOK)
	if got := testutil.ToFloat64(tagsListTotal.WithLabelValues("200")) - tagsBefore; got != 1 {
		t.Fatalf("tags_list_total delta = %v, want 1", got)
	}

	catBefore := testutil.ToFloat64(catalogListTotal.WithLabelValues("500"))
	ObserveCatalogList(http.StatusInternalServerError)
	if got := testutil.ToFloat64(catalogListTotal.WithLabelValues("500")) - catBefore; got != 1 {
		t.Fatalf("catalog_list_total delta = %v, want 1", got)
	}
}

func TestEnabledToggle(t *testing.T) {
	t.Cleanup(func() { SetEnabled(true) })
	SetEnabled(true)
	if !Enabled() {
		t.Fatal("Enabled() = false after SetEnabled(true)")
	}
	SetEnabled(false)
	if Enabled() {
		t.Fatal("Enabled() = true after SetEnabled(false)")
	}
}
