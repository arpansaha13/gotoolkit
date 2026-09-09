package memcached

import (
	"context"
	"errors"
	"testing"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/bradfitz/gomemcache/memcache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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

func TestTraceChildOfRequestSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	parentCtx, parent := tp.Tracer("test").Start(context.Background(), "GET /api")
	tr := tracer{}
	ctx := tr.Start(parentCtx, "GET", "session:abc")
	tr.End(ctx, errors.New("boom"))
	parent.End()

	var child sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Name() == "GET" {
			child = s
			break
		}
	}
	if child == nil {
		t.Fatal("missing GET span")
	}
	if child.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("parent span id = %s, want %s", child.Parent().SpanID(), parent.SpanContext().SpanID())
	}
	if child.SpanKind() != trace.SpanKindClient {
		t.Fatalf("span kind = %s, want client", child.SpanKind())
	}
	if child.Status().Code != codes.Error {
		t.Fatalf("status = %s, want error", child.Status().Code)
	}
}

func TestTraceCacheMissIsNotError(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	parentCtx, parent := tp.Tracer("test").Start(context.Background(), "GET /api")
	tr := tracer{}
	ctx := tr.Start(parentCtx, "GET", "missing")
	tr.End(ctx, memcache.ErrCacheMiss)
	parent.End()

	var child sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Name() == "GET" {
			child = s
			break
		}
	}
	if child == nil {
		t.Fatal("missing GET span")
	}
	if child.Status().Code != codes.Unset {
		t.Fatalf("cache miss status = %s, want unset", child.Status().Code)
	}
}

func TestTraceSkipsWithoutParent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	core, logs := observer.New(zapcore.WarnLevel)
	tr := tracer{log: zap.New(core)}
	ctx := tr.Start(context.Background(), "GET", "k")
	tr.End(ctx, nil)
	for _, s := range sr.Ended() {
		if s.Name() == "GET" {
			t.Fatal("no parent must not start a span")
		}
	}
	if logs.FilterMessage("skipped memcached span: no parent trace").Len() != 1 {
		t.Fatalf("warn count = %d, want 1", logs.Len())
	}
}

func TestMemcachedTracerReduceInstrumentationSilent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	core, logs := observer.New(zapcore.WarnLevel)
	tr := tracer{log: zap.New(core)}
	ctx := tr.Start(gtk.WithReduceInstrumentation(context.Background()), "GET", "k")
	tr.End(ctx, nil)
	if len(sr.Ended()) != 0 {
		t.Fatal("ReduceInstrumentation must not start a span")
	}
	if logs.Len() != 0 {
		t.Fatalf("ReduceInstrumentation must not warn, got %d", logs.Len())
	}
}
