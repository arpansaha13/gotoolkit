package tracer

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const tracerKey contextKey = "tracer"

// Middleware injects t into every request's context.
// Handlers retrieve it via FromContext to create child spans.
func Middleware(t trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), tracerKey, t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext retrieves the tracer from ctx.
// Returns a no-op tracer if none is stored.
func FromContext(ctx context.Context) trace.Tracer {
	t, ok := ctx.Value(tracerKey).(trace.Tracer)
	if !ok {
		return trace.NewNoopTracerProvider().Tracer("")
	}
	return t
}
