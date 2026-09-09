package nats

import (
	"context"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const natsTracerName = "github.com/arpansaha13/gotoolkit/nats"

type spanKey struct{}

type spanTracer interface {
	StartPublish(ctx context.Context, subject string) context.Context
	StartConsume(ctx context.Context, subject string) context.Context
	End(ctx context.Context, err error)
	Inject(ctx context.Context, header nats.Header)
	Extract(ctx context.Context, header nats.Header) context.Context
}

type noopTracer struct{}

func (noopTracer) StartPublish(ctx context.Context, _ string) context.Context { return ctx }
func (noopTracer) StartConsume(ctx context.Context, _ string) context.Context { return ctx }
func (noopTracer) End(context.Context, error)                                 {}
func (noopTracer) Inject(context.Context, nats.Header)                        {}
func (noopTracer) Extract(ctx context.Context, _ nats.Header) context.Context {
	return ctx
}

// tracer starts a producer span on Publish and a consumer span on
// Subscribe using the global TracerProvider so it sees StartTraces
// after client construction.
type tracer struct {
	log *zap.Logger
}

func (t tracer) logger() *zap.Logger {
	if t.log == nil {
		return zap.NewNop()
	}
	return t.log
}

func hasTrace(ctx context.Context) bool {
	return ctx != nil && trace.SpanContextFromContext(ctx).IsValid()
}

func (t tracer) StartPublish(ctx context.Context, subject string) context.Context {
	if gtk.ReduceInstrumentation(ctx) {
		return ctx
	}
	if !hasTrace(ctx) {
		t.logger().Warn("skipped nats publish span: no parent trace", zap.String("subject", subject))
		return ctx
	}
	return t.startSpan(ctx, "publish", subject, trace.SpanKindProducer, semconv.MessagingOperationTypePublish)
}

func (t tracer) StartConsume(ctx context.Context, subject string) context.Context {
	if gtk.ReduceInstrumentation(ctx) || !hasTrace(ctx) {
		return ctx
	}
	return t.startSpan(ctx, "receive", subject, trace.SpanKindConsumer, semconv.MessagingOperationTypeReceive)
}

func (t tracer) startSpan(ctx context.Context, op, subject string, kind trace.SpanKind, opType attribute.KeyValue) context.Context {
	ctx, span := otel.GetTracerProvider().Tracer(natsTracerName).Start(
		ctx,
		op,
		trace.WithSpanKind(kind),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("nats"),
			semconv.MessagingDestinationName(subject),
			opType,
			semconv.MessagingOperationName(op),
		),
	)
	return context.WithValue(ctx, spanKey{}, span)
}

func (t tracer) End(ctx context.Context, err error) {
	if ctx == nil {
		return
	}
	span, _ := ctx.Value(spanKey{}).(trace.Span)
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func (t tracer) Inject(ctx context.Context, header nats.Header) {
	if header == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(header))
}

func (t tracer) Extract(ctx context.Context, header nats.Header) context.Context {
	if len(header) == 0 {
		t.logger().Warn("skipped nats consume span: message header is missing")
		return ctx
	}
	out := otel.GetTextMapPropagator().Extract(ctx, natsHeaderCarrier(header))
	if !hasTrace(out) {
		t.logger().Warn("skipped nats consume span: no trace in message header")
	}
	return out
}

type natsHeaderCarrier nats.Header

func (c natsHeaderCarrier) Get(key string) string {
	return nats.Header(c).Get(key)
}

func (c natsHeaderCarrier) Set(key, value string) {
	if c == nil {
		return
	}
	nats.Header(c).Set(key, value)
}

func (c natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = natsHeaderCarrier{}
var _ spanTracer = tracer{}
var _ spanTracer = noopTracer{}
