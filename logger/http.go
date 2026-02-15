package logger

import (
	"net/http"

	"go.uber.org/zap"
)

// Middleware is an HTTP middleware that injects a logger into the request context.
// It captures trace_id, span_id, caller_ip, and content_length.
// Note: This middleware should run BEFORE Auth middleware. The auth middleware
// will add user_id to the logger context after validation.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get or generate trace_id from X-Trace-ID header
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = GenerateHexString()
		}

		// Generate span_id
		spanID := GenerateHexString()

		// Extract caller_ip from RemoteAddr
		callerIP := r.RemoteAddr

		// Get content_length
		contentLength := r.ContentLength

		// Create a logger with trace_id, span_id, caller_ip, and content_length
		// user_id will be added by the Auth Middleware
		base := zap.L()
		logger := base.With(
			zap.String("trace_id", traceID),
			zap.String("span_id", spanID),
			zap.String("caller_ip", callerIP),
			zap.Int64("content_length", contentLength),
		)

		// Log incoming request
		logger.Info("incoming request",
			zap.String("method", r.Method),
			zap.String("path", r.RequestURI),
		)

		// Inject logger into context
		ctx := WithContext(r.Context(), logger)

		// Call the next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
