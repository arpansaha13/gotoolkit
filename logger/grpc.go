package logger

import (
	"context"
	"time"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that logs with high observability.
// The logger (wrapped by uptrace otelzap) automatically captures active OTel span context
// when logging. It also captures caller_ip, method name, latency, and gRPC status on completion.
//
// Note: This interceptor must be chained AFTER any OTel gRPC instrumentation (e.g., otelgrpc)
// so that span context is already present in ctx.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		callerIP := ""
		if peerInfo, ok := peer.FromContext(ctx); ok {
			callerIP = peerInfo.Addr.String()
		}

		reqLogger := otelzap.L().WithOptions(
			zap.Fields(
				zap.String("method", info.FullMethod),
				zap.String("caller_ip", callerIP),
			),
		)
		ctx = WithContext(ctx, reqLogger)

		resp, err := handler(ctx, req)

		latencyMs := float64(time.Since(start).Milliseconds())

		statusCode := 0
		statusText := "OK"
		errorDetails := ""

		if err != nil {
			st, _ := status.FromError(err)
			statusCode = int(st.Code())
			statusText = st.Code().String()
			errorDetails = err.Error()
		}

		logFields := []zap.Field{
			zap.Int("status_code", statusCode),
			zap.String("status_text", statusText),
			zap.Float64("latency_ms", latencyMs),
		}

		if errorDetails != "" {
			logFields = append(logFields, zap.String("error_details", errorDetails))
		}

		FromContext(ctx).Info("grpc call completed", logFields...)

		return resp, err
	}
}
