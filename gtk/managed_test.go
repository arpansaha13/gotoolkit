package gtk

import (
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNoopCircuitExecute(t *testing.T) {
	c := NoopCircuit{}
	v, err := c.Execute(func() (any, error) {
		return 7, nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v != 7 {
		t.Fatalf("got %v want 7", v)
	}

	want := errors.New("boom")
	_, err = c.Execute(func() (any, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v want %v", err, want)
	}
}

func TestSharedOptionsApply(t *testing.T) {
	s := DefaultShared()
	if _, ok := s.Circuit.(NoopCircuit); !ok {
		t.Fatalf("default circuit = %T, want NoopCircuit", s.Circuit)
	}
	WithCircuit(nil).Apply(&s)
	if _, ok := s.Circuit.(NoopCircuit); !ok {
		t.Fatalf("nil WithCircuit = %T, want NoopCircuit", s.Circuit)
	}

	stub := stubCircuit{}
	log := zap.NewNop()
	s = DefaultShared()
	WithCircuit(stub).Apply(&s)
	WithLogger(log).Apply(&s)
	if s.Circuit != stub {
		t.Fatal("circuit not applied")
	}
	if s.Logger != log {
		t.Fatal("logger not applied")
	}
}

func TestDefaultConnectBackoff(t *testing.T) {
	log := zap.NewNop()
	cfg := ApplyBackoff(DefaultConnectBackoff(log, WithMaxRetries(8)))
	if cfg.MaxRetries != 8 {
		t.Fatalf("maxRetries = %d, want 8", cfg.MaxRetries)
	}
	if cfg.PermanentErrorLogLevel != zapcore.ErrorLevel {
		t.Fatalf("permanentErrorLogLevel = %v, want Error", cfg.PermanentErrorLogLevel)
	}
	if cfg.Logger != log {
		t.Fatal("logger not applied")
	}

	app := zap.NewExample()
	overridden := ApplyBackoff(DefaultConnectBackoff(log, WithBackoffLogger(app)))
	if overridden.Logger != app {
		t.Fatal("app WithBackoffLogger should override ctor logger")
	}
}

type stubCircuit struct{}

func (stubCircuit) Execute(fn func() (any, error)) (any, error) {
	return fn()
}
