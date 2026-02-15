package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"go.uber.org/zap"
)

// contextKey is an unexported type to avoid key collisions.
type contextKey string

const loggerContextKey contextKey = "logger"

// FromContext returns the logger stored in the context.
// If no logger is found, it returns the default zap.L().
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*zap.Logger); ok {
		return logger
	}
	return zap.L()
}

// WithContext injects a logger into the context.
func WithContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// WithFields adds additional fields to the logger in the context,
// creating a new logger and injecting it back.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := FromContext(ctx)
	newLogger := logger.With(fields...)
	return WithContext(ctx, newLogger)
}

// GenerateHexString generates a random 16-character hex string.
func GenerateHexString() string {
	b := make([]byte, 8) // 8 bytes = 16 hex characters
	if _, err := rand.Read(b); err != nil {
		// Fallback if random generation fails
		return fmt.Sprintf("%016x", 0)
	}
	return hex.EncodeToString(b)
}
