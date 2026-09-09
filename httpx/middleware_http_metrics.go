package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const httpMetricsMeter = "github.com/arpansaha13/gotoolkit/gtk/http"

// RouteFunc returns a low-cardinality route label for r (for example a path template).
type RouteFunc func(r *http.Request) string

// RoutePattern is the path template from the ServeMux pattern that matched r,
// or "unknown". The HTTP method is stripped so it can live only on the
// "method" metric attribute (r.Method).
func RoutePattern(r *http.Request) string {
	if r.Pattern == "" {
		return "unknown"
	}
	if _, path, ok := strings.Cut(r.Pattern, " "); ok && path != "" {
		return path
	}
	return r.Pattern
}

// MetricsMiddleware records request count, duration, and in-flight requests.
// Instruments bind to the global MeterProvider on the first request.
// Count and duration keep the request span only for status >= 500 or
// duration >= 1s so the SDK's default TraceBasedFilter attaches exemplars
// only to slow or failed traces.
//
// Count and duration are recorded in a defer, so they still fire if next panics.
// Place RecoveryMiddleware both before and after this middleware:
//   - after: recovers handler/inner-MW panics, writes 500 through the status
//     recorder, and lets this middleware record status=500
//   - before: recovers a panic inside this middleware itself (init, route
//     label, or the metrics defer)
//
// A single recovery only on one side either misses panic requests in metrics
// or lets a metrics panic escape.
func MetricsMiddleware(routeOf RouteFunc) func(http.Handler) http.Handler {
	if routeOf == nil {
		routeOf = func(*http.Request) string { return "unknown" }
	}
	m := &httpMetrics{routeOf: routeOf}
	return m.wrap
}

type httpMetrics struct {
	once     sync.Once
	routeOf  RouteFunc
	requests metric.Int64Counter
	duration metric.Float64Histogram
	inflight metric.Int64UpDownCounter
}

func (m *httpMetrics) init() {
	meter := otel.Meter(httpMetricsMeter)
	m.requests, _ = meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("HTTP requests"),
	)
	m.duration, _ = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	m.inflight, _ = meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("In-flight HTTP requests"),
	)
}

func (m *httpMetrics) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.once.Do(m.init)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		if m.inflight != nil {
			m.inflight.Add(r.Context(), 1)
			defer m.inflight.Add(r.Context(), -1)
		}
		start := time.Now()
		defer func() {
			elapsed := time.Since(start)
			statusAttrs := metric.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("route", m.routeOf(r)),
				attribute.String("status", strconv.Itoa(rec.status)),
			)
			recordCtx := metricsExemplarContext(r.Context(), attachHTTPExemplar(rec.status, elapsed))
			if m.requests != nil {
				m.requests.Add(recordCtx, 1, statusAttrs)
			}
			if m.duration != nil {
				m.duration.Record(recordCtx, elapsed.Seconds(), statusAttrs)
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }
