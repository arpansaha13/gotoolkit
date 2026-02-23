package logger

import (
	"net/http"

	"google.golang.org/grpc"

	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
)

// HttpMiddleware is an HTTP middleware that injects a logger into the request context.
func HttpMiddleware(next http.Handler) http.Handler {
	return internalLogger.HttpMiddleware(next)
}


// GrpcInterceptor returns a gRPC unary server interceptor that logs with high observability.
func GrpcInterceptor() grpc.UnaryServerInterceptor {
	return internalLogger.GrpcInterceptor()
}
