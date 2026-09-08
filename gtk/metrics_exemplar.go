package gtk

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
)

// metricsExemplarSlow is the duration at or above which a successful request
// still gets a trace exemplar. Faster non-error requests omit the span
// context so TraceBasedFilter does not attach an exemplar.
const metricsExemplarSlow = time.Second

func metricsExemplarContext(ctx context.Context, attach bool) context.Context {
	if attach {
		return ctx
	}
	return context.Background()
}

func httpAttachExemplar(status int, d time.Duration) bool {
	return status >= 500 || d >= metricsExemplarSlow
}

func grpcAttachExemplar(code codes.Code, d time.Duration) bool {
	switch code {
	case codes.Unknown, codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss:
		return true
	}
	return d >= metricsExemplarSlow
}
