package grpcx

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
)

// metricsExemplarSlow is the duration at or above which a successful request
// still gets a trace exemplar. Faster non-error requests omit the span
// context so OpenTelemetry's TraceBasedFilter does not attach an exemplar.
const metricsExemplarSlow = time.Second

// metricsExemplarContext selects the context passed to metric recording calls.
// This relies on OpenTelemetry's default TraceBasedFilter:
//   - attach=true: passes ctx with the active span so TraceBasedFilter accepts
//     the measurement and records trace_id and span_id as an exemplar.
//   - attach=false: passes context.Background(), causing TraceBasedFilter to
//     drop the exemplar because no sampled span is present.
//
// Note: If the MeterProvider uses AlwaysOnFilter, withholding ctx will not
// suppress exemplars.
func metricsExemplarContext(ctx context.Context, attach bool) context.Context {
	if attach {
		return ctx
	}
	return context.Background()
}

// attachGRPCExemplar is true for server-error codes or duration at/above the slow threshold.
func attachGRPCExemplar(code codes.Code, d time.Duration) bool {
	switch code {
	case codes.Unknown, codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss:
		return true
	}
	return d >= metricsExemplarSlow
}
