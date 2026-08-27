package gtk

import "go.uber.org/zap"

// ManagedClient is a long-lived dependency that connects on Start and
// releases resources on Stop.
type ManagedClient interface {
	Start() error
	Stop() error
}

// Circuit wraps an operation so callers can plug in a breaker (or anything
// with the same Execute shape). *gobreaker.CircuitBreaker[any] implements this.
type Circuit interface {
	Execute(func() (any, error)) (any, error)
}

// NoopCircuit runs the operation directly. Default when WithCircuit is omitted.
type NoopCircuit struct{}

// Execute runs fn with no trip logic.
func (NoopCircuit) Execute(fn func() (any, error)) (any, error) {
	return fn()
}

type sharedConfig struct {
	circuit Circuit
	logger  *zap.Logger
}

// sharedOption is a WithCircuit / WithLogger value. It implements every
// client option interface so the same call works on all managed clients.
type sharedOption struct {
	circuit Circuit
	logger  *zap.Logger
}

// WithCircuit sets the circuit used by client I/O methods.
// A nil circuit is ignored so the default NoopCircuit stays in place.
func WithCircuit(c Circuit) sharedOption {
	return sharedOption{circuit: c}
}

// WithLogger sets the client logger. Nil is ignored. Omitted uses LoggerFromContext.
func WithLogger(log *zap.Logger) sharedOption {
	return sharedOption{logger: log}
}

func (o sharedOption) applyTo(s *sharedConfig) {
	if o.circuit != nil {
		s.circuit = o.circuit
	}
	if o.logger != nil {
		s.logger = o.logger
	}
}

func (o sharedOption) applyMemcached(c *memcachedConfig) { o.applyTo(&c.shared) }
func (o sharedOption) applyRabbitMQ(c *rabbitMQConfig)   { o.applyTo(&c.shared) }
func (o sharedOption) applyPostgres(c *postgresConfig)   { o.applyTo(&c.shared) }

func defaultShared() sharedConfig {
	return sharedConfig{circuit: NoopCircuit{}}
}

func finalizeShared(s *sharedConfig) {
	if s.circuit == nil {
		s.circuit = NoopCircuit{}
	}
}

func circuitOrNoop(c Circuit) Circuit {
	if c == nil {
		return NoopCircuit{}
	}
	return c
}

func execErr(c Circuit, fn func() error) error {
	_, err := circuitOrNoop(c).Execute(func() (any, error) {
		return nil, fn()
	})
	return err
}

func execVal[T any](c Circuit, fn func() (T, error)) (T, error) {
	var zero T
	v, err := circuitOrNoop(c).Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		return zero, err
	}
	if v == nil {
		return zero, nil
	}
	typed, ok := v.(T)
	if !ok {
		return zero, nil
	}
	return typed, nil
}

var _ Circuit = NoopCircuit{}
