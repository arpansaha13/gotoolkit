package postgres

import (
	"github.com/arpansaha13/gotoolkit/gtk"
	"go.uber.org/zap"
)

// Option configures NewClient.
type Option func(*postgresConfig)

type postgresConfig struct {
	circuit     gtk.Circuit
	logger      *zap.Logger
	connectOpts []gtk.BackoffOption
}

// WithCircuit sets the circuit used by client I/O methods.
// A nil circuit is ignored so the default NoopCircuit stays in place.
func WithCircuit(c gtk.Circuit) Option {
	return func(cfg *postgresConfig) {
		if c != nil {
			cfg.circuit = c
		}
	}
}

// WithLogger sets the client logger. Nil is ignored. Omitted uses zap.NewNop.
func WithLogger(log *zap.Logger) Option {
	return func(cfg *postgresConfig) {
		if log != nil {
			cfg.logger = log
		}
	}
}

// WithBackoff sets backoff options for connectWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithBackoff(opts ...gtk.BackoffOption) Option {
	return func(c *postgresConfig) {
		c.connectOpts = opts
	}
}

func applyOptions(opts []Option) postgresConfig {
	cfg := postgresConfig{
		circuit: gtk.NoopCircuit{},
		logger:  zap.NewNop(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}
