package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/arpansaha13/gotoolkit/internal/domain"
)

func TestGrpcRecoveryInterceptor_WithPanic(t *testing.T) {
	interceptor := GrpcRecoveryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		panic("test panic")
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestGrpcRecoveryInterceptor_NoError(t *testing.T) {
	interceptor := GrpcRecoveryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	resp, err := interceptor(ctx, nil, info, handler)

	assert.Nil(t, err)
	assert.Equal(t, "response", resp)
}

func TestGrpcErrorInterceptor_ValidationError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, &domain.ValidationError{Message: "invalid input", Field: "email"}
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid input")
}

func TestGrpcErrorInterceptor_ConflictError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, &domain.ConflictError{Message: "resource already exists"}
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

func TestGrpcErrorInterceptor_NotFoundError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, &domain.NotFoundError{Message: "resource not found"}
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGrpcErrorInterceptor_UnauthorizedError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, &domain.UnauthorizedError{Message: "invalid token"}
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGrpcErrorInterceptor_ForbiddenError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, &domain.ForbiddenError{Message: "insufficient permissions"}
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "insufficient permissions")
}

func TestGrpcErrorInterceptor_InternalError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, &domain.InternalError{Message: "database error"}
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestGrpcErrorInterceptor_GenericError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, assert.AnError // Custom error not matching any domain type
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, handler)

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestGrpcErrorInterceptor_NoError(t *testing.T) {
	interceptor := GrpcErrorInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	resp, err := interceptor(ctx, nil, info, handler)

	assert.Nil(t, err)
	assert.Equal(t, "response", resp)
}
