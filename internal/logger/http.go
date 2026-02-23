package logger

import (
	"net/http"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// HttpMiddleware is an HTTP middleware that injects a logger into the request context.
// The logger (wrapped by uptrace otelzap) automatically captures active OTel span context
// when logging. It also captures caller_ip and content_length.
//
// Note: This middleware must run AFTER any OTel HTTP instrumentation (e.g., otelhttp)
// so that span context is already present. It must run BEFORE Auth middleware so that
// Auth can add user_id to the logger context after validation.
func HttpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Per-request logger: global fields (service_name etc.) + HTTP fields
		reqLogger := otelzap.L().WithOptions(
			zap.Fields(
				zap.String("caller_ip", r.RemoteAddr),
				zap.Int64("content_length", r.ContentLength),
			),
		)

		// Log with span correlation (uptrace otelzap automatically attaches span)
		reqLogger.Ctx(ctx).Info("incoming request",
			zap.String("method", r.Method),
			zap.String("path", r.RequestURI),
		)

		// Store logger in context for downstream handlers
		ctx = WithContext(ctx, reqLogger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
