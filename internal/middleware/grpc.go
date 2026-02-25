package middleware

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/arpansaha13/gotoolkit/internal/domain"
	"github.com/arpansaha13/gotoolkit/logger"
)

// GrpcRecoveryInterceptor recovers from panics in gRPC handlers
func GrpcRecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				lgr := logger.FromContext(ctx).Ctx(ctx)
				lgr.Error("panic recovered", zap.Any("panic_value", r), zap.String("method", info.FullMethod))
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// GrpcErrorInterceptor is a middleware that catches errors and translates domain errors to gRPC status codes
func GrpcErrorInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)

		if err != nil {
			lgr := logger.FromContext(ctx).Ctx(ctx)
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
	if domain.IsValidation(err) {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	if domain.IsConflict(err) {
		return status.Error(codes.AlreadyExists, err.Error())
	}

	if domain.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}

	if domain.IsUnauthorized(err) {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	if domain.IsForbidden(err) {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	// Default to internal error
	return status.Error(codes.Internal, fmt.Sprintf("internal server error: %v", err))
}
