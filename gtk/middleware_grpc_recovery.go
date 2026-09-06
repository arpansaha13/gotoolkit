package gtk

import (
	"context"

	"github.com/arpansaha13/gotoolkit/internal/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GrpcRecoveryInterceptor recovers from panics in gRPC handlers
func GrpcRecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				lgr := logger.LoggerFromContext(ctx)
				lgr.Error("panic recovered", zap.Any("panic_value", r), zap.String("method", info.FullMethod))
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}
