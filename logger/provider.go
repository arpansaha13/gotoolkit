package logger

import (
	internalLogger "github.com/arpansaha13/gotoolkit/internal/logger"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewKafkaLoggerProvider creates an OTel LoggerProvider with a BatchProcessor backed by KafkaExporter.
func NewKafkaLoggerProvider(writer *kafka.Writer, res *resource.Resource) (*log.LoggerProvider, error) {
	return internalLogger.NewKafkaLoggerProvider(writer, res)
}
