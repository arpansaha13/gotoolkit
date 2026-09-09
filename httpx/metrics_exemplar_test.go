package httpx

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestHttpAttachExemplar(t *testing.T) {
	tests := []struct {
		name   string
		status int
		d      time.Duration
		want   bool
	}{
		{"fast 200", http.StatusOK, 50 * time.Millisecond, false},
		{"slow 200", http.StatusOK, time.Second, true},
		{"fast 404", http.StatusNotFound, 50 * time.Millisecond, false},
		{"fast 500", http.StatusInternalServerError, 10 * time.Millisecond, true},
		{"fast 503", http.StatusServiceUnavailable, 10 * time.Millisecond, true},
		{"just under slow", http.StatusOK, time.Second - time.Nanosecond, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attachHTTPExemplar(tt.status, tt.d); got != tt.want {
				t.Fatalf("attachHTTPExemplar(%d, %s) = %v, want %v", tt.status, tt.d, got, tt.want)
			}
		})
	}
}

func TestMetricsExemplarContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "keep")
	if metricsExemplarContext(ctx, true) != ctx {
		t.Fatal("attach=true must keep the request context")
	}
	got := metricsExemplarContext(ctx, false)
	if got == ctx {
		t.Fatal("attach=false must drop the request context")
	}
	if got.Value(key{}) != nil {
		t.Fatal("attach=false context must not carry request values")
	}
}
