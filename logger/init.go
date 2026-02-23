package logger

import (
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap/zapcore"
	
	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
)

// InitLogger creates a zap.Logger wrapped with uptrace otelzap for automatic OTel span correlation.
func InitLogger(loggerProvider *sdklog.LoggerProvider, level zapcore.Level) (*otelzap.Logger, error) {
	return internalLogger.InitLogger(loggerProvider, level)
}
