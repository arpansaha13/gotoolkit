package memcached

import (
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"go.uber.org/zap"
)

// Option configures NewClient.
type Option func(*memcachedConfig)

type memcachedConfig struct {
	circuit      gtk.Circuit
	logger       *zap.Logger
	startTimeout time.Duration
	connectOpts  []gtk.BackoffOption
}

// WithCircuit sets the circuit used by client I/O methods.
// A nil circuit is ignored so the default NoopCircuit stays in place.
func WithCircuit(c gtk.Circuit) Option {
	return func(cfg *memcachedConfig) {
		if c != nil {
			cfg.circuit = c
		}
	}
}

// WithLogger sets the client logger. Nil is ignored. Omitted uses zap.NewNop.
func WithLogger(log *zap.Logger) Option {
	return func(cfg *memcachedConfig) {
		if log != nil {
			cfg.logger = log
		}
	}
}

// WithStartTimeout bounds Start connect/backoff. Zero or omitted uses the
// constructor context as-is.
func WithStartTimeout(d time.Duration) Option {
	return func(c *memcachedConfig) {
		if d > 0 {
			c.startTimeout = d
		}
	}
}

// WithBackoff sets backoff options for connectWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithBackoff(opts ...gtk.BackoffOption) Option {
	return func(c *memcachedConfig) {
		c.connectOpts = opts
	}
}

func applyOptions(opts []Option) memcachedConfig {
	cfg := memcachedConfig{
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
