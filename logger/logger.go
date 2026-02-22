package logger

import (
	"context"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// contextKey is an unexported type to avoid key collisions.
type contextKey string

const loggerContextKey contextKey = "logger"

// FromContext returns the *zap.Logger stored in the context with automatic OTel span correlation.
// The uptrace/otelzap package automatically attaches span context when logging.
// If no logger is found, it returns zap.L() with span correlation.
func FromContext(ctx context.Context) *otelzap.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*otelzap.Logger); ok {
		return logger
	}
	return otelzap.L()
}

// WithContext stores a *zap.Logger in ctx for downstream retrieval via Ctx.
func WithContext(ctx context.Context, l *otelzap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// WithFields adds fields to the *zap.Logger stored in ctx.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := FromContext(ctx)
	newLogger := logger.WithOptions(zap.Fields(fields...))
	return WithContext(ctx, newLogger)
}
