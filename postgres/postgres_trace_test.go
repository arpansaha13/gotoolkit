package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestSQLVerb(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "QUERY"},
		{"SELECT 1", "SELECT"},
		{"  insert into t values (1)", "INSERT"},
		{"delete", "DELETE"},
	}
	for _, tt := range tests {
		if got := sqlVerb(tt.in); got != tt.want {
			t.Fatalf("sqlVerb(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWithTracingAppliesToPostgres(t *testing.T) {
	off := applyOptions(nil)
	if _, ok := off.tracer.(noopQueryTracer); !ok {
		t.Fatalf("default tracer = %T, want noopQueryTracer", off.tracer)
	}
	justTracing := applyOptions([]Option{WithTracing()})
	if tr, ok := justTracing.tracer.(pgxQueryTracer); !ok || tr.log == nil {
		t.Fatalf("WithTracing without logger = %v, want non-nil log", justTracing.tracer)
	}
	log := zap.NewNop()
	on := applyOptions([]Option{WithLogger(log), WithTracing()})
	tr, ok := on.tracer.(pgxQueryTracer)
	if !ok {
		t.Fatalf("WithTracing tracer = %T, want pgxQueryTracer", on.tracer)
	}
	if tr.log != log {
		t.Fatal("WithLogger must be copied onto the tracer")
	}
}

func TestClientOptionsApply(t *testing.T) {
	off := applyOptions(nil)
	if off.maxOpenConns != 0 {
		t.Fatalf("omitted maxOpenConns = %d, want 0", off.maxOpenConns)
	}
	if off.startTimeout != 0 {
		t.Fatalf("omitted startTimeout = %v, want 0", off.startTimeout)
	}

	on := applyOptions([]Option{
		WithMaxOpenConns(16),
		WithStartTimeout(30 * time.Second),
		WithMaxOpenConns(0),
		WithStartTimeout(0),
	})
	if on.maxOpenConns != 16 {
		t.Fatalf("maxOpenConns = %d, want 16", on.maxOpenConns)
	}
	if on.startTimeout != 30*time.Second {
		t.Fatalf("startTimeout = %v, want 30s", on.startTimeout)
	}
}

func TestPgxQueryTracerChildOfRequestSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	parentCtx, parent := tp.Tracer("test").Start(context.Background(), "GET /api")
	tr := pgxQueryTracer{}
	ctx := tr.TraceQueryStart(parentCtx, nil, pgx.TraceQueryStartData{SQL: "SELECT id FROM users"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("boom")})
	parent.End()

	ended := sr.Ended()
	if len(ended) < 2 {
		t.Fatalf("ended spans = %d, want at least 2", len(ended))
	}
	var child sdktrace.ReadOnlySpan
	for _, s := range ended {
		if s.Name() == "SELECT" {
			child = s
			break
		}
	}
	if child == nil {
		t.Fatal("missing SELECT span")
	}
	if child.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatalf("parent span id = %s, want %s", child.Parent().SpanID(), parent.SpanContext().SpanID())
	}
	if child.SpanKind() != trace.SpanKindClient {
		t.Fatalf("span kind = %s, want client", child.SpanKind())
	}
}

func TestPgxQueryTracerSkipsWithoutParent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	core, logs := observer.New(zapcore.WarnLevel)
	tr := pgxQueryTracer{log: zap.New(core)}
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	for _, s := range sr.Ended() {
		if s.Name() == "SELECT" {
			t.Fatal("no parent must not start a query span")
		}
	}
	if logs.FilterMessage("skipped query span: no parent trace").Len() != 1 {
		t.Fatalf("warn count = %d, want 1", logs.Len())
	}
}

func TestPgxQueryTracerReduceInstrumentationSilent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	core, logs := observer.New(zapcore.WarnLevel)
	tr := pgxQueryTracer{log: zap.New(core)}
	ctx := gtk.WithReduceInstrumentation(context.Background())
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	if len(sr.Ended()) != 0 {
		t.Fatal("ReduceInstrumentation must not start a query span")
	}
	if logs.Len() != 0 {
		t.Fatalf("ReduceInstrumentation must not warn, got %d", logs.Len())
	}
}
