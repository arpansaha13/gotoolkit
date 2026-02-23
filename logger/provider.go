package logger

import (
	"github.com/arpansaha13/gotoolkit/logger/exporter"
	"github.com/arpansaha13/gotoolkit/logger/processor"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewKafkaLoggerProvider creates an OTel LoggerProvider with a BatchProcessor backed
// by KafkaExporter. The caller must call loggerProvider.Shutdown(ctx) on graceful shutdown.
// Note: the provided kafka.Writer is owned by the exporter after this call — do NOT
// close it separately, as Shutdown will close it.
func NewKafkaLoggerProvider(writer *kafka.Writer, res *resource.Resource) (*log.LoggerProvider, error) {
	exporter := exporter.NewKafkaExporter(writer)
	traceSpanProcessor := &processor.TraceSpanAttrProcessor{}
	batchProcessor := log.NewBatchProcessor(exporter)
	provider := log.NewLoggerProvider(
		log.WithProcessor(traceSpanProcessor), // add IDs to attributes
		log.WithProcessor(batchProcessor),     // then batch/export
		log.WithResource(res),
	)
	return provider, nil
}
