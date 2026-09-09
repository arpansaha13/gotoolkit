package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arpansaha13/gotoolkit/gtk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestHealthcheckPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/livez", true},
		{"/api/livez", true},
		{"/ws/livez", true},
		{"/readyz", true},
		{"/api/readyz", true},
		{"/ws/readyz", true},
		{"/metrics", false},
		{"/api/messages", false},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		if got := healthcheckPath(req); got != tt.want {
			t.Fatalf("healthcheckPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
	if healthcheckPath(nil) {
		t.Fatal("nil request must not be a healthcheck")
	}
}

func TestMetricsHealthcheckSeparateFromRequests(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() {
		_ = mp.Shutdown(t.Context())
		otel.SetMeterProvider(noop.NewMeterProvider())
	})

	r := NewRouter(MetricsMiddleware(RoutePattern))
	r.Controller("GET /api/livez", func(http.ResponseWriter, *http.Request) (*ControllerResponse, error) {
		return &ControllerResponse{StatusCode: http.StatusOK}, nil
	})
	r.Controller("GET /api/messages", func(http.ResponseWriter, *http.Request) (*ControllerResponse, error) {
		return &ControllerResponse{StatusCode: http.StatusOK}, nil
	})
	h := r.Handler()

	probe := httptest.NewRequest(http.MethodGet, "/api/livez", nil)
	probe = probe.WithContext(gtk.WithReduceInstrumentation(probe.Context()))
	h.ServeHTTP(httptest.NewRecorder(), probe)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/messages", nil))

	var got metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &got); err != nil {
		t.Fatal(err)
	}
	if n := metricSum(got, "http.server.healthcheck.count"); n != 1 {
		t.Fatalf("healthcheck.count = %d, want 1", n)
	}
	if n := metricSum(got, "http.server.request.count"); n != 1 {
		t.Fatalf("request.count = %d, want 1", n)
	}
	if !metricHasHistogram(got, "http.server.healthcheck.duration") {
		t.Fatal("missing http.server.healthcheck.duration")
	}
}

func TestMetricsSkipsScrapePath(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() {
		_ = mp.Shutdown(t.Context())
		otel.SetMeterProvider(noop.NewMeterProvider())
	})

	h := MetricsMiddleware(RoutePattern)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req = req.WithContext(gtk.WithReduceInstrumentation(req.Context()))
	h.ServeHTTP(httptest.NewRecorder(), req)

	var got metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &got); err != nil {
		t.Fatal(err)
	}
	if n := metricSum(got, "http.server.healthcheck.count"); n != 0 {
		t.Fatalf("healthcheck.count = %d, want 0", n)
	}
	if n := metricSum(got, "http.server.request.count"); n != 0 {
		t.Fatalf("request.count = %d, want 0", n)
	}
}

func metricSum(rm metricdata.ResourceMetrics, name string) int64 {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				return 0
			}
			var n int64
			for _, dp := range sum.DataPoints {
				n += dp.Value
			}
			return n
		}
	}
	return 0
}

func metricHasHistogram(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				_, ok := m.Data.(metricdata.Histogram[float64])
				return ok
			}
		}
	}
	return false
}
