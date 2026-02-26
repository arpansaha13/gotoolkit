package logger

import (
	"net/http"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
)

// HttpMiddleware returns an HTTP middleware factory that injects a logger into the request context.
// It extracts trace_id and span_id from the OTel span context and adds them as fields.
func HttpMiddleware(l *zap.Logger) func(http.Handler) http.Handler {
	return internalLogger.HttpMiddleware(l)
}

// GrpcInterceptor returns a gRPC unary server interceptor that logs with high observability.
// It extracts trace_id and span_id from the OTel span context and adds them as fields.
func GrpcInterceptor(l *zap.Logger) grpc.UnaryServerInterceptor {
	return internalLogger.GrpcInterceptor(l)
}
