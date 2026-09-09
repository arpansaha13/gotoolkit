package nats

import (
	"context"
	"testing"

	"github.com/arpansaha13/gotoolkit/gtk"
	natslib "github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWithTracingApplies(t *testing.T) {
	off := applyOptions(nil)
	if _, ok := off.tracer.(noopTracer); !ok {
		t.Fatalf("default tracer = %T, want noopTracer", off.tracer)
	}
	justTracing := applyOptions([]Option{WithTracing()})
	if tr, ok := justTracing.tracer.(tracer); !ok || tr.log == nil {
		t.Fatalf("WithTracing without logger = %v, want non-nil log", justTracing.tracer)
	}
	log := zap.NewNop()
	on := applyOptions([]Option{WithLogger(log), WithTracing()})
	tr, ok := on.tracer.(tracer)
	if !ok {
		t.Fatalf("WithTracing tracer = %T, want tracer", on.tracer)
	}
	if tr.log != log {
		t.Fatal("WithLogger must be copied onto the tracer")
	}
}

func TestPublishReceiveShareTrace(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	})

	tr := tracer{}
	parentCtx, parent := tp.Tracer("test").Start(context.Background(), "POST /api/messages")
	pubCtx := tr.StartPublish(parentCtx, "channel.1")
	header := natslib.Header{}
	tr.Inject(pubCtx, header)
	tr.End(pubCtx, nil)
	parent.End()

	recvCtx := tr.Extract(context.Background(), header)
	recvCtx = tr.StartConsume(recvCtx, "channel.1")
	tr.End(recvCtx, nil)

	var pub, recv sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		switch s.Name() {
		case "publish":
			pub = s
		case "receive":
			recv = s
		}
	}
	if pub == nil {
		t.Fatal("missing publish span")
	}
	if recv == nil {
		t.Fatal("missing receive span")
	}
	if pub.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("publish parent = %s, want %s", pub.Parent().SpanID(), parent.SpanContext().SpanID())
	}
	if pub.SpanKind() != trace.SpanKindProducer {
		t.Fatalf("publish kind = %s, want producer", pub.SpanKind())
	}
	if recv.Parent().SpanID() != pub.SpanContext().SpanID() {
		t.Fatalf("receive parent = %s, want publish %s", recv.Parent().SpanID(), pub.SpanContext().SpanID())
	}
	if recv.SpanKind() != trace.SpanKindConsumer {
		t.Fatalf("receive kind = %s, want consumer", recv.SpanKind())
	}
	if recv.SpanContext().TraceID() != parent.SpanContext().TraceID() {
		t.Fatalf("receive trace = %s, want %s", recv.SpanContext().TraceID(), parent.SpanContext().TraceID())
	}
}

func TestNoParentDoesNotStartSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	core, logs := observer.New(zapcore.WarnLevel)
	tr := tracer{log: zap.New(core)}
	pubCtx := tr.StartPublish(context.Background(), "channel.1")
	tr.End(pubCtx, nil)
	recvCtx := tr.Extract(context.Background(), nil)
	recvCtx = tr.StartConsume(recvCtx, "channel.1")
	tr.End(recvCtx, nil)
	for _, s := range sr.Ended() {
		switch s.Name() {
		case "publish", "receive":
			t.Fatalf("no parent started %s", s.Name())
		}
	}
	if logs.FilterMessage("skipped nats publish span: no parent trace").Len() != 1 {
		t.Fatalf("publish warn count = %d, want 1", logs.Len())
	}
	if logs.FilterMessage("skipped nats consume span: message header is missing").Len() != 1 {
		t.Fatalf("missing-header warn count = %d, want 1", logs.FilterMessage("skipped nats consume span: message header is missing").Len())
	}
	if logs.FilterMessage("skipped nats consume span: no parent trace").Len() != 0 {
		t.Fatal("StartConsume must not warn after Extract already explained the skip")
	}
	noTraceCtx := tr.Extract(context.Background(), natslib.Header{"X-Foo": []string{"1"}})
	tr.End(tr.StartConsume(noTraceCtx, "channel.1"), nil)
	if logs.FilterMessage("skipped nats consume span: no trace in message header").Len() != 1 {
		t.Fatalf("no-trace-header warn count = %d, want 1", logs.FilterMessage("skipped nats consume span: no trace in message header").Len())
	}
	if logs.FilterMessage("skipped nats consume span: no parent trace").Len() != 0 {
		t.Fatal("no-trace-in-header must not also log no parent")
	}
}

func TestNATSTracerReduceInstrumentationSilent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	core, logs := observer.New(zapcore.WarnLevel)
	tr := tracer{log: zap.New(core)}
	ctx := gtk.WithReduceInstrumentation(context.Background())
	pubCtx := tr.StartPublish(ctx, "channel.1")
	tr.End(pubCtx, nil)
	recvCtx := tr.StartConsume(ctx, "channel.1")
	tr.End(recvCtx, nil)
	if len(sr.Ended()) != 0 {
		t.Fatal("ReduceInstrumentation must not start a span")
	}
	if logs.Len() != 0 {
		t.Fatalf("ReduceInstrumentation must not warn, got %d", logs.Len())
	}
}
