package gtk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutePatternUnknown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	if got := RoutePattern(req); got != "unknown" {
		t.Fatalf("RoutePattern() = %q, want unknown", got)
	}
}

func TestRoutePatternFromServeMux(t *testing.T) {
	var got string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		got = RoutePattern(r)
	})
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if got != "GET /users/{id}" {
		t.Fatalf("RoutePattern() = %q, want GET /users/{id}", got)
	}
}

func TestChainOrder(t *testing.T) {
	var order []string
	mw := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "h")
	}), mw("a"), mw("b"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	want := []string{"a", "b", "h"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
