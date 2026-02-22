package logger

import (
	"net/http"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Middleware is an HTTP middleware that injects a logger into the request context.
// It reads trace_id and span_id from the active OTel span in the request context,
// and also captures caller_ip and content_length.
//
// Note: This middleware must run AFTER any OTel HTTP instrumentation (e.g., otelhttp)
// so that span context is already present. It must run BEFORE Auth middleware so that
// Auth can add user_id to the logger context after validation.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sc := trace.SpanFromContext(r.Context()).SpanContext()

		logger := zap.L().With(
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
			zap.String("caller_ip", r.RemoteAddr),
			zap.Int64("content_length", r.ContentLength),
		)

		logger.Info("incoming request",
			zap.String("method", r.Method),
			zap.String("path", r.RequestURI),
		)

		next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), logger)))
	})
}
