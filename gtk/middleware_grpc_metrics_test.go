package gtk

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGrpcMetricsInterceptorPassesThrough(t *testing.T) {
	want := status.Error(codes.NotFound, "missing")
	info := &grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Get"}
	interceptor := GrpcMetricsInterceptor()
	_, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return "ok", want
	})
	if err != want {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestGrpcMetricsInterceptorRecoversViaInnerHandler(t *testing.T) {
	// Inner recovery converts panic to Internal; metrics must still return that error.
	info := &grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Panic"}
	chain := GrpcMetricsInterceptor()
	inner := GrpcRecoveryInterceptor()
	_, err := chain(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return inner(ctx, req, info, func(context.Context, any) (any, error) {
			panic("boom")
		})
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
}
