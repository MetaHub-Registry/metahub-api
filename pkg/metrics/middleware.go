package metrics

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Middleware records operational HTTP metrics (in-flight gauge, request
// counter, duration histogram) for every request. It labels by method, the
// matched chi route pattern, and the response status code. The concrete
// request path is deliberately NOT used as a label, to keep cardinality
// bounded. The route pattern is read after the inner handler runs, by which
// point chi has populated the route context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled.Load() {
			next.ServeHTTP(w, r)
			return
		}
		httpInFlight.Inc()
		defer httpInFlight.Dec()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		code := statusLabel(ww.Status())
		httpRequestsTotal.WithLabelValues(r.Method, route, code).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route, code).Observe(time.Since(start).Seconds())
	})
}
