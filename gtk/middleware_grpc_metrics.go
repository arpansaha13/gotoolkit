package gtk

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const grpcMetricsMeter = "github.com/arpansaha13/gotoolkit/gtk/grpc"

// GrpcMetricsInterceptor records unary RPC count, duration, and in-flight requests.
// Instruments bind to the global MeterProvider on the first request.
// Count and duration keep the request span only for server-error codes
// (Unknown, DeadlineExceeded, Internal, Unavailable, DataLoss) or
// duration >= 1s so Prometheus exemplars point at slow or failed traces.
//
// Count and duration are recorded in a defer, so they still fire if the handler
// panics. Place GrpcRecoveryInterceptor both before and after this interceptor:
//   - after: recovers handler/inner-interceptor panics, returns codes.Internal,
//     and lets this interceptor record status=Internal
//   - before: recovers a panic inside this interceptor itself (init or the
//     metrics defer)
//
// A single recovery only on one side either misses panic RPCs in metrics
// or lets a metrics panic escape.
func GrpcMetricsInterceptor() grpc.UnaryServerInterceptor {
	m := &grpcMetrics{}
	return m.unary
}

// GrpcStreamMetricsInterceptor is the streaming counterpart of
// GrpcMetricsInterceptor. Use the same recovery-before-and-after order.
func GrpcStreamMetricsInterceptor() grpc.StreamServerInterceptor {
	m := &grpcMetrics{}
	return m.stream
}

type grpcMetrics struct {
	once     sync.Once
	requests metric.Int64Counter
	duration metric.Float64Histogram
	inflight metric.Int64UpDownCounter
}

func (m *grpcMetrics) init() {
	meter := otel.Meter(grpcMetricsMeter)
	m.requests, _ = meter.Int64Counter(
		"grpc.server.request.count",
		metric.WithDescription("gRPC server requests"),
	)
	m.duration, _ = meter.Float64Histogram(
		"grpc.server.request.duration",
		metric.WithDescription("gRPC server request duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	m.inflight, _ = meter.Int64UpDownCounter(
		"grpc.server.active_requests",
		metric.WithDescription("In-flight gRPC server requests"),
	)
}

func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return st.Code()
}

func (m *grpcMetrics) record(ctx context.Context, method string, start time.Time, err error) {
	elapsed := time.Since(start)
	code := grpcCode(err)
	statusAttrs := metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("status", code.String()),
	)
	recordCtx := metricsExemplarContext(ctx, grpcAttachExemplar(code, elapsed))
	if m.requests != nil {
		m.requests.Add(recordCtx, 1, statusAttrs)
	}
	if m.duration != nil {
		m.duration.Record(recordCtx, elapsed.Seconds(), statusAttrs)
	}
}

func (m *grpcMetrics) unary(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	m.once.Do(m.init)
	if m.inflight != nil {
		m.inflight.Add(ctx, 1)
		defer m.inflight.Add(ctx, -1)
	}
	start := time.Now()
	defer func() { m.record(ctx, info.FullMethod, start, err) }()
	return handler(ctx, req)
}

func (m *grpcMetrics) stream(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	m.once.Do(m.init)
	ctx := ss.Context()
	if m.inflight != nil {
		m.inflight.Add(ctx, 1)
		defer m.inflight.Add(ctx, -1)
	}
	start := time.Now()
	defer func() { m.record(ctx, info.FullMethod, start, err) }()
	return handler(srv, ss)
}
