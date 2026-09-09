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

// Shared is the logger/circuit set applied by Option.
type Shared struct {
	Circuit Circuit
	Logger  *zap.Logger
}

// Option configures Shared. The same values work on every managed client.
type Option interface {
	Apply(*Shared)
}

type option struct {
	circuit Circuit
	logger  *zap.Logger
}

// WithCircuit sets the circuit used by client I/O methods.
// A nil circuit is ignored so the default NoopCircuit stays in place.
func WithCircuit(c Circuit) Option {
	return option{circuit: c}
}

// WithLogger sets the client logger. Nil is ignored. Omitted uses zap.NewNop.
func WithLogger(log *zap.Logger) Option {
	return option{logger: log}
}

func (o option) Apply(s *Shared) {
	if o.circuit != nil {
		s.Circuit = o.circuit
	}
	if o.logger != nil {
		s.Logger = o.logger
	}
}

// DefaultShared is the zero option set (noop circuit, nop logger).
func DefaultShared() Shared {
	return Shared{Circuit: NoopCircuit{}, Logger: zap.NewNop()}
}

// Finalize fills nil circuit/logger with defaults.
func Finalize(s *Shared) {
	if s.Circuit == nil {
		s.Circuit = NoopCircuit{}
	}
	if s.Logger == nil {
		s.Logger = zap.NewNop()
	}
}

func circuitOrNoop(c Circuit) Circuit {
	if c == nil {
		return NoopCircuit{}
	}
	return c
}

// ExecErr runs fn through the circuit and returns only the error.
func ExecErr(c Circuit, fn func() error) error {
	_, err := circuitOrNoop(c).Execute(func() (any, error) {
		return nil, fn()
	})
	return err
}

// ExecVal runs fn through the circuit and returns the typed result.
func ExecVal[T any](c Circuit, fn func() (T, error)) (T, error) {
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
var _ Option = option{}
