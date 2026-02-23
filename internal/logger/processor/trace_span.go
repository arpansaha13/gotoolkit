package processor

import (
	"context"

	logapi "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type TraceSpanAttrProcessor struct{}

func (p *TraceSpanAttrProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool {
	return true
}

func (p *TraceSpanAttrProcessor) OnEmit(_ context.Context, record *sdklog.Record) error {
	tid := record.TraceID()
	if tid.IsValid() {
		record.AddAttributes(logapi.String("trace_id", tid.String()))
	}
	sid := record.SpanID()
	if sid.IsValid() {
		record.AddAttributes(logapi.String("span_id", sid.String()))
	}
	return nil
}

func (p *TraceSpanAttrProcessor) Shutdown(context.Context) error { return nil }
func (p *TraceSpanAttrProcessor) ForceFlush(context.Context) error { return nil }
