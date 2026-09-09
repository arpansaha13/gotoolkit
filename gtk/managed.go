package gtk

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
