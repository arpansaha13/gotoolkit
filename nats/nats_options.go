package nats

import (
	"github.com/arpansaha13/gotoolkit/gtk"
	"go.uber.org/zap"
)

// Option configures NewClient.
type Option func(*natsConfig)

type natsConfig struct {
	circuit gtk.Circuit
	logger  *zap.Logger
}

// WithCircuit sets the circuit used by client I/O methods.
// A nil circuit is ignored so the default NoopCircuit stays in place.
func WithCircuit(c gtk.Circuit) Option {
	return func(cfg *natsConfig) {
		if c != nil {
			cfg.circuit = c
		}
	}
}

// WithLogger sets the client logger. Nil is ignored. Omitted uses zap.NewNop.
func WithLogger(log *zap.Logger) Option {
	return func(cfg *natsConfig) {
		if log != nil {
			cfg.logger = log
		}
	}
}

func applyOptions(opts []Option) natsConfig {
	cfg := natsConfig{
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
