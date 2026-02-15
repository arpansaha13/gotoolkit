package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that logs with high observability.
// It captures trace_id, span_id, caller_ip, method name, and latency.
// It also logs error details and status codes on error.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		// Get or generate trace_id from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		traceID := ""
		if ok {
			values := md.Get("trace-id")
			if len(values) > 0 {
				traceID = values[0]
			}
		}
		if traceID == "" {
			traceID = GenerateHexString()
		}

		// Generate span_id
		spanID := GenerateHexString()

		// Extract caller_ip from peer information
		callerIP := ""
		if peerInfo, ok := peer.FromContext(ctx); ok {
			callerIP = peerInfo.Addr.String()
		}

		// Create a logger with trace_id, span_id, and method
		base := zap.L()
		logger := base.With(
			zap.String("trace_id", traceID),
			zap.String("span_id", spanID),
			zap.String("method", info.FullMethod),
			zap.String("caller_ip", callerIP),
		)

		// Inject logger into context
		ctx = WithContext(ctx, logger)

		// Call the handler
		resp, err := handler(ctx, req)

		// Calculate latency
		latencyMs := float64(time.Since(start).Milliseconds())

		// Extract status code and text from error
		statusCode := 0
		statusText := "OK"
		errorDetails := ""

		if err != nil {
			st, _ := status.FromError(err)
			statusCode = int(st.Code())
			statusText = st.Code().String()
			errorDetails = err.Error()
		}

		// Log completion
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
