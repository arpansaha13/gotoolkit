package logger

import (
	"os"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger creates a zap.Logger that fans out to two cores via zapcore.NewTee:
//  1. A stdout JSON core — same schema and encoder config as before.
//  2. An OTel bridge core — routes records through loggerProvider to the Kafka exporter.
//
// The loggerProvider must be created with NewKafkaLoggerProvider and its Shutdown
// must be deferred by the caller for proper flush on graceful shutdown.
func InitLogger(loggerProvider *sdklog.LoggerProvider, level zapcore.Level) (*zap.Logger, error) {
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

	otelCore := otelzap.NewCore(
		"gotoolkit/logger",
		otelzap.WithLoggerProvider(loggerProvider),
	)

	logger := zap.New(
		zapcore.NewTee(stdoutCore, otelCore),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger, nil
}
