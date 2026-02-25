package gotoolkit

import (
	"context"
	"net/http"

	"google.golang.org/grpc"

	internalmw "github.com/arpansaha13/gotoolkit/internal/middleware"
)

// ErrorResponse is the standard error response format
type ErrorResponse = internalmw.ErrorResponse

// HTTP Middleware Functions

// HttpRecoveryMiddleware is a minimal recovery middleware that catches any
// uncaught panics and returns a generic 500 error response.
// It should be the outermost middleware in the chain.
func HttpRecoveryMiddleware(next http.Handler) http.Handler {
	return internalmw.HttpRecoveryMiddleware(next)
}

// HttpErrorMiddleware recovers from panics thrown by handlers and converts
// domain errors to HTTP responses. It should be placed after HttpRecoveryMiddleware
// and LoggingMiddleware in the middleware chain.
func HttpErrorMiddleware(next http.Handler) http.Handler {
	return internalmw.HttpErrorMiddleware(next)
}

// HttpWriteErrorWithContext writes an error response with logging to the client
func HttpWriteErrorWithContext(w http.ResponseWriter, ctx context.Context, err error) {
	internalmw.HttpWriteErrorWithContext(w, ctx, err)
}

// gRPC Interceptor Functions

// GrpcRecoveryInterceptor recovers from panics in gRPC handlers
func GrpcRecoveryInterceptor() grpc.UnaryServerInterceptor {
	return internalmw.GrpcRecoveryInterceptor()
}

// GrpcErrorInterceptor is a middleware that catches errors and translates domain errors to gRPC status codes
func GrpcErrorInterceptor() grpc.UnaryServerInterceptor {
	return internalmw.GrpcErrorInterceptor()
}
