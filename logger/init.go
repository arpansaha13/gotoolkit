package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLoggerWithChannel creates a zap logger that writes logs to the provided channel.
// This logger is suitable for production environments where logs need to be streamed.
//
// Parameters:
//   - logChan: Buffered channel that will receive log entries
//   - level: Minimum log level to emit (e.g. zapcore.DebugLevel, zapcore.InfoLevel)
//
// The channel should have sufficient buffer to avoid blocking the application.
// Recommended buffer size: 1000-5000 depending on log volume.
func InitLoggerWithChannel(logChan chan<- []byte, level zapcore.Level) (*zap.Logger, error) {
	// Create channel writer
	channelWriter := NewChannelWriter(logChan)

	// Create encoder config for structured logging
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

	// Create encoder (JSON for easy parsing)
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create core with custom writer
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(channelWriter),
		level,
	)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}
