package gtk

import (
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// HttpTraceMiddleware extracts the OpenTelemetry trace ID from the request context
// and sets it in the response headers as X-Trace-ID.
// It should be registered early in the middleware chain (after otelhttp).
func HttpTraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if span != nil && span.SpanContext().IsValid() {
			w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
		}
		next.ServeHTTP(w, r)
	})
}
