package gtk

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
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

func TestSharedOptionsApplyPerClient(t *testing.T) {
	stub := stubCircuit{}
	log := zap.NewNop()

	mc := applyMemcachedOptions([]MemcachedOption{
		WithCircuit(nil),
	})
	if _, ok := mc.shared.circuit.(NoopCircuit); !ok {
		t.Fatalf("nil WithCircuit = %T, want NoopCircuit", mc.shared.circuit)
	}

	mc = applyMemcachedOptions([]MemcachedOption{
		WithCircuit(stub),
		WithLogger(log),
		WithStartTimeout(time.Second),
	})
	if mc.shared.circuit != stub {
		t.Fatalf("memcached circuit not applied")
	}
	if mc.shared.logger != log {
		t.Fatalf("memcached logger not applied")
	}
	if mc.startTimeout != time.Second {
		t.Fatalf("startTimeout = %s, want 1s", mc.startTimeout)
	}

	rc := applyRabbitMQOptions([]RabbitMQOption{
		WithCircuit(stub),
		WithLogger(log),
		WithConnectTimeout(2 * time.Second),
		WithTopology(RabbitMQTopology{Exchanges: []ExchangeDecl{{Name: "ex"}}}),
	})
	if rc.shared.circuit != stub {
		t.Fatalf("rabbitmq circuit not applied")
	}
	if rc.shared.logger != log {
		t.Fatalf("rabbitmq logger not applied")
	}
	if rc.connectTimeout != 2*time.Second {
		t.Fatalf("connectTimeout = %s, want 2s", rc.connectTimeout)
	}
	if len(rc.topology.Exchanges) != 1 || rc.topology.Exchanges[0].Name != "ex" {
		t.Fatalf("topology not applied")
	}

	pc := applyPostgresOptions([]PostgresOption{WithCircuit(stub)})
	if pc.shared.circuit != stub {
		t.Fatalf("postgres circuit not applied")
	}
}

type stubCircuit struct{}

func (stubCircuit) Execute(fn func() (any, error)) (any, error) {
	return fn()
}
