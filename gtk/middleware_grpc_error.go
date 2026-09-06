package gtk

import (
	"context"
	"fmt"

	"github.com/arpansaha13/gotoolkit/internal/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GrpcErrorInterceptor is a middleware that catches errors and translates domain errors to gRPC status codes
func GrpcErrorInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)

		if err != nil {
			lgr := logger.LoggerFromContext(ctx)
			lgr.Error("grpc error", zap.String("method", info.FullMethod), zap.Error(err))
			return nil, errorToGRPCError(err)
		}

		return resp, nil
	}
}

// errorToGRPCError translates domain errors to gRPC status codes
func errorToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// Check error types and map to appropriate gRPC codes
	if IsValidation(err) {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	if IsConflict(err) {
		return status.Error(codes.AlreadyExists, err.Error())
	}

	if IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}

	if IsUnauthorized(err) {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	if IsForbidden(err) {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	if IsServiceUnavailable(err) {
		return status.Error(codes.Unavailable, err.Error())
	}

	// Default to internal error
	return status.Error(codes.Internal, fmt.Sprintf("internal server error: %v", err))
}
