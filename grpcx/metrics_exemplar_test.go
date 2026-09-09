package grpcx

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
)

func TestGrpcAttachExemplar(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		d    time.Duration
		want bool
	}{
		{"fast OK", codes.OK, 50 * time.Millisecond, false},
		{"slow OK", codes.OK, time.Second, true},
		{"fast NotFound", codes.NotFound, 10 * time.Millisecond, false},
		{"Internal", codes.Internal, 10 * time.Millisecond, true},
		{"Unavailable", codes.Unavailable, 10 * time.Millisecond, true},
		{"Unknown", codes.Unknown, 10 * time.Millisecond, true},
		{"DeadlineExceeded", codes.DeadlineExceeded, 10 * time.Millisecond, true},
		{"DataLoss", codes.DataLoss, 10 * time.Millisecond, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attachGRPCExemplar(tt.code, tt.d); got != tt.want {
				t.Fatalf("attachGRPCExemplar(%s, %s) = %v, want %v", tt.code, tt.d, got, tt.want)
			}
		})
	}
}

func TestMetricsExemplarContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "keep")
	if metricsExemplarContext(ctx, true) != ctx {
		t.Fatal("attach=true must keep the request context")
	}
	got := metricsExemplarContext(ctx, false)
	if got == ctx {
		t.Fatal("attach=false must drop the request context")
	}
	if got.Value(key{}) != nil {
		t.Fatal("attach=false context must not carry request values")
	}
}
