package logger

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that logs with high observability.
// It reads trace_id and span_id from the active OTel span in the request context,
// and captures caller_ip, method name, latency, and gRPC status on completion.
//
// Note: This interceptor must be chained AFTER any OTel gRPC instrumentation (e.g., otelgrpc)
// so that span context is already present in ctx.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		sc := trace.SpanFromContext(ctx).SpanContext()

		callerIP := ""
		if peerInfo, ok := peer.FromContext(ctx); ok {
			callerIP = peerInfo.Addr.String()
		}

		logger := zap.L().With(
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
			zap.String("method", info.FullMethod),
			zap.String("caller_ip", callerIP),
		)

		ctx = WithContext(ctx, logger)

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

		logger.Info("grpc call completed", logFields...)

		return resp, err
	}
}
