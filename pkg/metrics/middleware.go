package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// ChiMiddleware returns a chi-compatible middleware that records
// request count, duration, and response size per (method, route, status).
func ChiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		route := routePattern(r)
		method := r.Method
		status := strconv.Itoa(ww.Status())

		HTTPRequestsTotal.WithLabelValues(method, route, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
		HTTPResponseSize.WithLabelValues(method, route).Observe(float64(ww.BytesWritten()))
	})
}

func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	return fmt.Sprintf("%s (unmatched)", r.URL.Path)
}
