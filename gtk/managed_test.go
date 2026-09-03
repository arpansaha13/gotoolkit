package gtk

import (
	"errors"
	"testing"
	"time"

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
	if pc.shared.logger == nil {
		t.Fatal("omitted logger is nil, want Nop")
	}

	nc := applyNATSOptions([]NATSOption{
		WithCircuit(stub),
		WithLogger(log),
	})
	if nc.shared.circuit != stub {
		t.Fatalf("nats circuit not applied")
	}
	if nc.shared.logger != log {
		t.Fatalf("nats logger not applied")
	}

	ncNil := applyNATSOptions([]NATSOption{WithCircuit(nil)})
	if _, ok := ncNil.shared.circuit.(NoopCircuit); !ok {
		t.Fatalf("nil WithCircuit = %T, want NoopCircuit", ncNil.shared.circuit)
	}
}

func TestWithConnectOptionsStored(t *testing.T) {
	mc := applyMemcachedOptions([]MemcachedOption{
		WithMemcachedBackoff(WithMaxRetries(8)),
	})
	if len(mc.connectOpts) != 1 {
		t.Fatalf("memcached connectOpts len = %d, want 1", len(mc.connectOpts))
	}

	omitted := applyMemcachedOptions(nil)
	if omitted.connectOpts != nil {
		t.Fatal("omitted WithMemcachedBackoff should leave connectOpts unset")
	}

	rc := applyRabbitMQOptions([]RabbitMQOption{
		WithRabbitMQBackoff(WithMaxRetries(8)),
	})
	if len(rc.connectOpts) != 1 {
		t.Fatalf("rabbitmq connectOpts len = %d, want 1", len(rc.connectOpts))
	}

	pc := applyPostgresOptions([]PostgresOption{
		WithPostgresBackoff(WithMaxRetries(8)),
	})
	if len(pc.connectOpts) != 1 {
		t.Fatalf("postgres connectOpts len = %d, want 1", len(pc.connectOpts))
	}
}

func TestDefaultConnectBackoff(t *testing.T) {
	log := zap.NewNop()
	cfg := applyOptions(defaultConnectBackoff(log, WithMaxRetries(8)))
	if cfg.maxRetries != 8 {
		t.Fatalf("maxRetries = %d, want 8", cfg.maxRetries)
	}
	if cfg.permanentErrorLogLevel != zapcore.ErrorLevel {
		t.Fatalf("permanentErrorLogLevel = %v, want Error", cfg.permanentErrorLogLevel)
	}
	if cfg.logger != log {
		t.Fatal("logger not applied")
	}

	app := zap.NewExample()
	overridden := applyOptions(defaultConnectBackoff(log, WithBackoffLogger(app)))
	if overridden.logger != app {
		t.Fatal("app WithBackoffLogger should override ctor logger")
	}
}

type stubCircuit struct{}

func (stubCircuit) Execute(fn func() (any, error)) (any, error) {
	return fn()
}
