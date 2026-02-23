package logger

import (
	"os"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger creates a zap.Logger wrapped with uptrace otelzap for automatic OTel span correlation.
// The uptrace otelzap wrapper automatically attaches OTel span context to log records at emit time.
// The loggerProvider must be created with NewKafkaLoggerProvider and its Shutdown
// must be deferred by the caller for proper flush on graceful shutdown.
func InitLogger(loggerProvider *sdklog.LoggerProvider, level zapcore.Level) (*otelzap.Logger, error) {
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

	base := zap.New(
		stdoutCore,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// Wrap with uptrace otelzap for automatic span correlation
	return otelzap.New(
		base,
		otelzap.WithLoggerProvider(loggerProvider),
		otelzap.WithMinLevel(level),
		otelzap.WithCallerDepth(1),
	), nil
}
