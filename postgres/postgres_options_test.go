package postgres

import (
	"testing"

	"github.com/arpansaha13/gotoolkit/gtk"
	"go.uber.org/zap"
)

func TestWithCircuitAndLogger(t *testing.T) {
	off := applyOptions(nil)
	if _, ok := off.circuit.(gtk.NoopCircuit); !ok {
		t.Fatalf("default circuit = %T, want NoopCircuit", off.circuit)
	}
	if off.logger == nil {
		t.Fatal("default logger is nil, want Nop")
	}

	ignored := applyOptions([]Option{WithCircuit(nil), WithLogger(nil)})
	if _, ok := ignored.circuit.(gtk.NoopCircuit); !ok {
		t.Fatalf("nil WithCircuit = %T, want NoopCircuit", ignored.circuit)
	}

	stub := stubCircuit{}
	log := zap.NewNop()
	on := applyOptions([]Option{WithCircuit(stub), WithLogger(log)})
	if on.circuit != stub {
		t.Fatal("circuit not applied")
	}
	if on.logger != log {
		t.Fatal("logger not applied")
	}
}

type stubCircuit struct{}

func (stubCircuit) Execute(fn func() (any, error)) (any, error) {
	return fn()
}
