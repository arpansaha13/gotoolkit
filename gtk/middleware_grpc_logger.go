package gtk

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// GrpcLoggerInterceptor returns a gRPC unary server interceptor that logs with high observability.
// It extracts trace_id and span_id from the OTel span context and adds them as fields.
// It also captures caller_ip, method name, latency, and gRPC status on completion.
//
// Note: This interceptor must be chained AFTER any OTel gRPC instrumentation (e.g., otelgrpc)
// so that span context is already present in ctx.
func GrpcLoggerInterceptor(l *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		callerIP := ""
		if peerInfo, ok := peer.FromContext(ctx); ok {
			callerIP = peerInfo.Addr.String()
		}

		// Extract span context and add trace_id and span_id
		span := trace.SpanFromContext(ctx)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("caller_ip", callerIP),
		}

		if span.SpanContext().IsValid() {
			fields = append(fields,
				zap.String("trace_id", span.SpanContext().TraceID().String()),
				zap.String("span_id", span.SpanContext().SpanID().String()),
			)
		}

		reqLogger := l.WithOptions(zap.Fields(fields...))
		ctx = LoggerWithContext(ctx, reqLogger)

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

		LoggerFromContext(ctx).Info("grpc call completed", logFields...)

		return resp, err
	}
}
