package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arpansaha13/gotoolkit/gtk"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggerMiddlewareSkipsWhenReduced(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	h := LoggerMiddleware(zap.New(core))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/api/livez", nil)
	req = req.WithContext(gtk.WithReduceInstrumentation(req.Context()))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if logs.FilterMessage("incoming request").Len() != 0 {
		t.Fatalf("ReduceInstrumentation must not log incoming request, got %d", logs.Len())
	}
}

func TestLoggerMiddlewareLogsIncoming(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	h := LoggerMiddleware(zap.New(core))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/messages", nil))
	if logs.FilterMessage("incoming request").Len() != 1 {
		t.Fatalf("incoming request count = %d, want 1", logs.FilterMessage("incoming request").Len())
	}
}

func TestReadyzKeeps503Log(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	r := NewRouter(LoggerMiddleware(zap.New(core)))
	r.Controller("GET /readyz", func(http.ResponseWriter, *http.Request) (*ControllerResponse, error) {
		return nil, &gtk.ServiceUnavailableError{Message: "postgres unhealthy"}
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req = req.WithContext(gtk.WithReduceInstrumentation(req.Context()))
	r.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if logs.FilterMessage("incoming request").Len() != 0 {
		t.Fatal("ReduceInstrumentation must not log incoming request")
	}
	if logs.FilterMessage("error response").Len() != 1 {
		t.Fatalf("error response count = %d, want 1", logs.FilterMessage("error response").Len())
	}
}
