package gtk

import (
	"net/http"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// HttpLoggerMiddleware returns an HTTP middleware that injects a logger into the request context.
// It extracts trace_id and span_id from the OTel span context and adds them as fields.
//
// Note: This middleware must run AFTER any OTel HTTP instrumentation (e.g., otelhttp)
// so that span context is already present. It must run BEFORE Auth middleware so that
// Auth can add user_id to the logger context after validation.
func HttpLoggerMiddleware(l *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract span context and add trace_id and span_id as fields
			span := trace.SpanFromContext(ctx)
			fields := []zap.Field{
				zap.String("caller_ip", r.RemoteAddr),
				zap.Int64("content_length", r.ContentLength),
			}

			if span.SpanContext().IsValid() {
				fields = append(fields,
					zap.String("trace_id", span.SpanContext().TraceID().String()),
					zap.String("span_id", span.SpanContext().SpanID().String()),
				)
			}

			// Per-request logger: global fields (service_name etc.) + HTTP fields
			reqLogger := l.WithOptions(zap.Fields(fields...))

			userAgent := r.Header.Get("User-Agent")

			// Log incoming request
			reqLogger.Info("incoming request",
				zap.String("method", r.Method),
				zap.String("path", r.RequestURI),
				zap.String("user_agent", userAgent),
			)

			// Store logger in context for downstream handlers
			ctx = LoggerWithContext(ctx, reqLogger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
