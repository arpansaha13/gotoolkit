package gtk

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const httpMetricsMeter = "github.com/arpansaha13/gotoolkit/gtk/http"

// HttpRouteFunc returns a low-cardinality route label for r (for example a path template).
type HttpRouteFunc func(r *http.Request) string

// MuxRouteTemplate is the gorilla/mux path template for r, or "unknown".
func MuxRouteTemplate(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return "unknown"
	}
	tpl, err := route.GetPathTemplate()
	if err != nil || tpl == "" {
		return "unknown"
	}
	return tpl
}

// HttpMetricsMiddleware records request count, duration, and in-flight requests.
// Instruments bind to the global MeterProvider on the first request.
func HttpMetricsMiddleware(routeOf HttpRouteFunc) func(http.Handler) http.Handler {
	if routeOf == nil {
		routeOf = func(*http.Request) string { return "unknown" }
	}
	m := &httpMetrics{routeOf: routeOf}
	return m.wrap
}

type httpMetrics struct {
	once     sync.Once
	routeOf  HttpRouteFunc
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
		attrs := metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("route", m.routeOf(r)),
		)
		if m.inflight != nil {
			m.inflight.Add(r.Context(), 1, attrs)
			defer m.inflight.Add(r.Context(), -1, attrs)
		}
		start := time.Now()
		next.ServeHTTP(rec, r)
		statusAttrs := metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("route", m.routeOf(r)),
			attribute.String("status", strconv.Itoa(rec.status)),
		)
		if m.requests != nil {
			m.requests.Add(r.Context(), 1, statusAttrs)
		}
		if m.duration != nil {
			m.duration.Record(r.Context(), time.Since(start).Seconds(), statusAttrs)
		}
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
