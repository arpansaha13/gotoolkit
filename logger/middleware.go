package logger

import (
	"net/http"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"google.golang.org/grpc"

	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
)

// HttpMiddleware returns an HTTP middleware factory that injects a logger into the request context.
func HttpMiddleware(l *otelzap.Logger) func(http.Handler) http.Handler {
	return internalLogger.HttpMiddleware(l)
}

// GrpcInterceptor returns a gRPC unary server interceptor that logs with high observability.
func GrpcInterceptor(l *otelzap.Logger) grpc.UnaryServerInterceptor {
	return internalLogger.GrpcInterceptor(l)
}
