package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// KafkaExporter implements sdklog.Exporter.
// It serializes OTel log records to the zap JSON schema and writes them to Kafka.
type KafkaExporter struct {
	writer  *kafka.Writer
	mu      sync.Mutex
	stopped bool
}

func newKafkaExporter(writer *kafka.Writer) *KafkaExporter {
	return &KafkaExporter{writer: writer}
}

// Export serializes each record to JSON and bulk-writes to Kafka.
// Individual serialization errors are skipped to match the old drop-on-full behavior.
func (e *KafkaExporter) Export(ctx context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	msgs := make([]kafka.Message, 0, len(records))
	for _, r := range records {
		b, err := serializeRecord(r)
		if err != nil {
			continue
		}
		msgs = append(msgs, kafka.Message{Value: b})
	}
	if len(msgs) == 0 {
		return nil
	}
	return e.writer.WriteMessages(ctx, msgs...)
}

// Shutdown marks the exporter stopped and closes the Kafka writer.
// The kafka.Writer lifecycle ends here; callers must not close the writer separately.
func (e *KafkaExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return nil
	}
	e.stopped = true
	return e.writer.Close()
}

// ForceFlush is a no-op; kafka-go manages its own internal batching and flush.
func (e *KafkaExporter) ForceFlush(_ context.Context) error {
	return nil
}

// NewKafkaLoggerProvider creates an OTel LoggerProvider with a BatchProcessor backed
// by KafkaExporter. The caller must call loggerProvider.Shutdown(ctx) on graceful shutdown.
// Note: the provided kafka.Writer is owned by the exporter after this call — do NOT
// close it separately, as Shutdown will close it.
func NewKafkaLoggerProvider(writer *kafka.Writer, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exporter := newKafkaExporter(writer)
	processor := sdklog.NewBatchProcessor(exporter)
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(processor),
		sdklog.WithResource(res),
	)
	return provider, nil
}

// serializeRecord converts an sdklog.Record to a flat JSON []byte using the same
// field names as the zap JSON encoder config: timestamp, level, message, plus all
// attributes as top-level keys.
func serializeRecord(r sdklog.Record) ([]byte, error) {
	ts := r.Timestamp()
	if ts.IsZero() {
		ts = r.ObservedTimestamp()
	}
	if ts.IsZero() {
		ts = time.Now()
	}

	m := make(map[string]any, r.AttributesLen()+3)
	m["timestamp"] = ts.UTC().Format(time.RFC3339Nano)
	m["level"] = otelSeverityToZapLevel(r.Severity())
	m["message"] = r.Body().AsString()

	r.WalkAttributes(func(kv otellog.KeyValue) bool {
		m[string(kv.Key)] = otelValueToAny(kv.Value)
		return true
	})

	return json.Marshal(m)
}

// otelSeverityToZapLevel maps OTel severity to the lowercase level string used by
// zapcore.LowercaseLevelEncoder, preserving all seven zap level names.
func otelSeverityToZapLevel(s otellog.Severity) string {
	switch {
	case s >= otellog.SeverityFatal3:
		return "fatal"
	case s >= otellog.SeverityFatal2:
		return "panic"
	case s >= otellog.SeverityFatal:
		return "dpanic"
	case s >= otellog.SeverityError:
		return "error"
	case s >= otellog.SeverityWarn:
		return "warn"
	case s >= otellog.SeverityInfo:
		return "info"
	default:
		return "debug"
	}
}

// otelValueToAny converts an otellog.Value to a native Go type suitable for json.Marshal.
func otelValueToAny(v otellog.Value) any {
	switch v.Kind() {
	case otellog.KindBool:
		return v.AsBool()
	case otellog.KindInt64:
		return v.AsInt64()
	case otellog.KindFloat64:
		return v.AsFloat64()
	case otellog.KindString:
		return v.AsString()
	case otellog.KindBytes:
		return v.AsBytes()
	case otellog.KindSlice:
		items := v.AsSlice()
		out := make([]any, len(items))
		for i, sv := range items {
			out[i] = otelValueToAny(sv)
		}
		return out
	case otellog.KindMap:
		kvs := v.AsMap()
		out := make(map[string]any, len(kvs))
		for _, kv := range kvs {
			out[string(kv.Key)] = otelValueToAny(kv.Value)
		}
		return out
	default:
		return fmt.Sprintf("%v", v)
	}
}
