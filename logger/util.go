package logger

import (
	"context"

	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
	"go.uber.org/zap"
)

// FromContext returns the *zap.Logger stored in the context.
// If no logger is found, it returns zap.L().
func FromContext(ctx context.Context) *zap.Logger {
	return internalLogger.FromContext(ctx)
}

// WithContext stores a *zap.Logger in ctx for downstream retrieval.
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return internalLogger.WithContext(ctx, l)
}

// WithFields adds fields to the *zap.Logger stored in ctx.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return internalLogger.WithFields(ctx, fields...)
}

// WithSpanContext extracts trace_id and span_id from the context's span and adds them as fields.
func WithSpanContext(ctx context.Context) context.Context {
	return internalLogger.WithSpanContext(ctx)
}
