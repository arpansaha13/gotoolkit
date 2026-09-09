package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const postgresTracerName = "github.com/arpansaha13/gotoolkit/postgres"

type pgxSpanKey struct{}

type noopQueryTracer struct{}

func (noopQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}

func (noopQueryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

var _ pgx.QueryTracer = noopQueryTracer{}
var _ pgx.QueryTracer = pgxQueryTracer{}

// pgxQueryTracer starts a client span per Query/QueryRow/Exec using the
// global TracerProvider so it sees StartTraces after client construction.
type pgxQueryTracer struct {
	log *zap.Logger
}

func (t pgxQueryTracer) logger() *zap.Logger {
	if t.log == nil {
		return zap.NewNop()
	}
	return t.log
}

func (t pgxQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
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
	return context.WithValue(ctx, pgxSpanKey{}, span)
}

func (pgxQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, _ := ctx.Value(pgxSpanKey{}).(trace.Span)
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
