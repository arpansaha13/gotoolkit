package logger

import (
	"context"

	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// FromContext returns the *otelzap.Logger stored in the context with automatic OTel span correlation.
func FromContext(ctx context.Context) *otelzap.Logger {
	return internalLogger.FromContext(ctx)
}

// WithContext stores a *otelzap.Logger in ctx for downstream retrieval.
func WithContext(ctx context.Context, l *otelzap.Logger) context.Context {
	return internalLogger.WithContext(ctx, l)
}

// WithFields adds fields to the *otelzap.Logger stored in ctx.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return internalLogger.WithFields(ctx, fields...)
}
