package memcached

import (
	"context"
	"errors"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/bradfitz/gomemcache/memcache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const memcachedTracerName = "github.com/arpansaha13/gotoolkit/memcached"

type spanKey struct{}

type spanTracer interface {
	Start(ctx context.Context, op, key string) context.Context
	End(ctx context.Context, err error)
}

type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _, _ string) context.Context { return ctx }
func (noopTracer) End(context.Context, error)                             {}

// tracer starts a client span per Get/Set/Delete using the global
// TracerProvider so it sees StartTraces after client construction.
type tracer struct {
	log *zap.Logger
}

func (t tracer) logger() *zap.Logger {
	if t.log == nil {
		return zap.NewNop()
	}
	return t.log
}

func (t tracer) Start(ctx context.Context, op, key string) context.Context {
	if gtk.ReduceInstrumentation(ctx) {
		return ctx
	}
	if ctx == nil || !trace.SpanContextFromContext(ctx).IsValid() {
		t.logger().Warn("skipped memcached span: no parent trace",
			zap.String("op", op),
			zap.String("key", key),
		)
		return ctx
	}
	ctx, span := otel.GetTracerProvider().Tracer(memcachedTracerName).Start(
		ctx,
		op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemMemcached,
			semconv.DBOperationName(op),
			semconv.DBQueryText(key),
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
	if err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

var _ spanTracer = tracer{}
var _ spanTracer = noopTracer{}
