package postgres

import (
	"context"
	"strings"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const postgresTracerName = "github.com/arpansaha13/gotoolkit/postgres"

type querySpanKey struct{}
type acquireSpanKey struct{}

type noopTracer struct{}

func (noopTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}

func (noopTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (noopTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	return ctx
}

func (noopTracer) TraceAcquireEnd(context.Context, *pgxpool.Pool, pgxpool.TraceAcquireEndData) {
}

var _ pgx.QueryTracer = noopTracer{}
var _ pgx.QueryTracer = tracer{}
var _ pgxpool.AcquireTracer = noopTracer{}
var _ pgxpool.AcquireTracer = tracer{}

// tracer starts a client span per Query/QueryRow/Exec and per
// pool Acquire using the global TracerProvider so it sees StartTraces
// after client construction. pgxpool type-asserts ConnConfig.Tracer
// for AcquireTracer.
type tracer struct {
	log *zap.Logger
}

func (t tracer) logger() *zap.Logger {
	if t.log == nil {
		return zap.NewNop()
	}
	return t.log
}

func (t tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if gtk.ReduceInstrumentation(ctx) {
		return ctx
	}
	if !trace.SpanContextFromContext(ctx).IsValid() {
		t.logger().Warn("skipped query span: no parent trace")
		return ctx
	}
	ctx, span := otel.GetTracerProvider().Tracer(postgresTracerName).Start(
		ctx,
		sqlVerb(data.SQL),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBQueryText(data.SQL),
		),
	)
	return context.WithValue(ctx, querySpanKey{}, span)
}

func (tracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, _ := ctx.Value(querySpanKey{}).(trace.Span)
	if span == nil {
		return
	}
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	}
	span.End()
}

func (t tracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	if gtk.ReduceInstrumentation(ctx) {
		return ctx
	}
	if !trace.SpanContextFromContext(ctx).IsValid() {
		t.logger().Warn("skipped acquire span: no parent trace")
		return ctx
	}
	ctx, span := otel.GetTracerProvider().Tracer(postgresTracerName).Start(
		ctx,
		"ACQUIRE",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	return context.WithValue(ctx, acquireSpanKey{}, span)
}

func (tracer) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	span, _ := ctx.Value(acquireSpanKey{}).(trace.Span)
	if span == nil {
		return
	}
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	}
	span.End()
}

func sqlVerb(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return "QUERY"
	}
	if i := strings.IndexAny(sql, " \t\n\r"); i > 0 {
		return strings.ToUpper(sql[:i])
	}
	return strings.ToUpper(sql)
}
