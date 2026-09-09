package gtk

import "context"

type reduceInstrumentationKey struct{}

// WithReduceInstrumentation marks ctx so HTTP middleware and client tracers
// drop the usual per-request instrumentation.
//
// Use it for high-frequency, low-value traffic such as kube livez/readyz
// probes and /metrics scrapes. Those calls would otherwise:
//   - emit an Info log on every hit
//   - inflate HTTP request-rate and latency metrics (livez/readyz still
//     record http.server.healthcheck.*)
//   - create a root HTTP span, plus child spans for probe I/O (postgres Ping)
//
// Downstream tracers (postgres, memcached, nats) honor the flag by skipping
// span start without the "no parent trace" warning. That warning is meant
// for real request I/O that lost its span, not for traffic that opted out.
//
// Error logs still fire. A failing readyz (HTTP 503) is logged as usual.
func WithReduceInstrumentation(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, reduceInstrumentationKey{}, true)
}

// ReduceInstrumentation reports whether ctx was marked by WithReduceInstrumentation.
func ReduceInstrumentation(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(reduceInstrumentationKey{}).(bool)
	return v
}
