package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arpansaha13/gotoolkit/gtk"
)

func namedMW(name string, order *[]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*order = append(*order, name)
			next.ServeHTTP(w, r)
		})
	}
}

func TestHttpRouterHandlerAppliesGlobalMiddleware(t *testing.T) {
	var order []string
	rt := NewRouter(namedMW("g1", &order), namedMW("g2", &order))
	rt.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "h")
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	rt.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	want := []string{"g1", "g2", "h"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestHttpRouterChildPrependsRouteMiddleware(t *testing.T) {
	var order []string
	root := NewRouter(namedMW("g", &order))
	child := root.Child(namedMW("p", &order))
	grand := child.Child(namedMW("c", &order))
	grand.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "h")
		w.WriteHeader(http.StatusNoContent)
	})

	root.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	want := []string{"g", "p", "c", "h"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestHttpRouterHandlerPanicsOnChild(t *testing.T) {
	root := NewRouter()
	child := root.Child()
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic")
		}
	}()
	child.Handler()
}

func TestHttpRouterControllerMapsErrorWithoutPanic(t *testing.T) {
	rt := NewRouter()
	rt.Controller("GET /boom", func(http.ResponseWriter, *http.Request) (*ControllerResponse, error) {
		return nil, &gtk.ValidationError{Message: "bad"}
	})

	rec := httptest.NewRecorder()
	rt.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	var er ErrorResponse
	if err := json.Unmarshal(body, &er); err != nil {
		t.Fatal(err)
	}
	if er.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q", er.Code)
	}
}

func TestHttpRouterControllerSuccess(t *testing.T) {
	rt := NewRouter()
	rt.Controller("GET /ok", func(http.ResponseWriter, *http.Request) (*ControllerResponse, error) {
		return &ControllerResponse{StatusCode: http.StatusCreated, Body: map[string]string{"id": "1"}}, nil
	})

	rec := httptest.NewRecorder()
	rt.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
}
