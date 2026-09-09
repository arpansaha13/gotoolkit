package gtk

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey is an unexported type to avoid key collisions.
type contextKey string

const loggerContextKey contextKey = "logger"

// NewZapLogger creates a zap.Logger with JSON output to stdout.
func NewZapLogger(level zapcore.Level) (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "function",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	return zap.New(
		stdoutCore,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	), nil
}

// LoggerFromContext returns the *zap.Logger stored in the context.
// If no logger is found, it returns zap.L().
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*zap.Logger); ok {
		return logger
	}
	return zap.L()
}

// LoggerWithContext stores a *zap.Logger in ctx for downstream retrieval via LoggerFromContext.
func LoggerWithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// LoggerWithFields adds fields to the *zap.Logger stored in ctx and returns a new context.
func LoggerWithFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := LoggerFromContext(ctx)
	newLogger := logger.WithOptions(zap.Fields(fields...))
	return LoggerWithContext(ctx, newLogger)
}
