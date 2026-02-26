package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
)

// InitLogger creates a zap.Logger with JSON output to stdout.
func InitLogger(level zapcore.Level) (*zap.Logger, error) {
	return internalLogger.InitLogger(level)
}
