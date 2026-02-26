package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// contextKey is an unexported type to avoid key collisions.
type contextKey string

const loggerContextKey contextKey = "logger"

// FromContext returns the *zap.Logger stored in the context.
// If no logger is found, it returns zap.L().
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*zap.Logger); ok {
		return logger
	}
	return zap.L()
}

// WithContext stores a *zap.Logger in ctx for downstream retrieval via FromContext.
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// WithFields adds fields to the *zap.Logger stored in ctx and returns a new context.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := FromContext(ctx)
	newLogger := logger.WithOptions(zap.Fields(fields...))
	return WithContext(ctx, newLogger)
}

// WithSpanContext extracts trace_id and span_id from the context's span and adds them as fields.
func WithSpanContext(ctx context.Context) context.Context {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ctx
	}

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	return WithFields(ctx, zap.String("trace_id", traceID), zap.String("span_id", spanID))
}
