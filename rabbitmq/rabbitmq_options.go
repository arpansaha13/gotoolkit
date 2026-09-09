package rabbitmq

import (
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"go.uber.org/zap"
)

const (
	defaultRabbitMQConnectTimeout    = 15 * time.Second
	defaultRabbitMQReconnectInterval = 500 * time.Millisecond
)

// Option configures NewClient.
type Option func(*rabbitMQConfig)

type rabbitMQConfig struct {
	circuit           gtk.Circuit
	logger            *zap.Logger
	connectTimeout    time.Duration
	reconnectInterval time.Duration
	topology          Topology
	connectOpts       []gtk.BackoffOption
}

// WithCircuit sets the circuit used by client I/O methods.
// A nil circuit is ignored so the default NoopCircuit stays in place.
func WithCircuit(c gtk.Circuit) Option {
	return func(cfg *rabbitMQConfig) {
		if c != nil {
			cfg.circuit = c
		}
	}
}

// WithLogger sets the client logger. Nil is ignored. Omitted uses zap.NewNop.
func WithLogger(log *zap.Logger) Option {
	return func(cfg *rabbitMQConfig) {
		if log != nil {
			cfg.logger = log
		}
	}
}

// WithConnectTimeout sets the per-dial budget in the reconnect loop.
// Zero or omitted uses 15s.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *rabbitMQConfig) {
		if d > 0 {
			c.connectTimeout = d
		}
	}
}

// WithReconnectInterval sets the delay between reconnect attempts.
// Zero or omitted uses 500ms.
func WithReconnectInterval(d time.Duration) Option {
	return func(c *rabbitMQConfig) {
		if d > 0 {
			c.reconnectInterval = d
		}
	}
}

// WithTopology declares exchanges, queues, and bindings on every new channel.
func WithTopology(t Topology) Option {
	return func(c *rabbitMQConfig) {
		c.topology = t
	}
}

// WithBackoff sets backoff options for connectWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithBackoff(opts ...gtk.BackoffOption) Option {
	return func(c *rabbitMQConfig) {
		c.connectOpts = opts
	}
}

func applyOptions(opts []Option) rabbitMQConfig {
	cfg := rabbitMQConfig{
		circuit: gtk.NoopCircuit{},
		logger:  zap.NewNop(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.connectTimeout == 0 {
		cfg.connectTimeout = defaultRabbitMQConnectTimeout
	}
	if cfg.reconnectInterval == 0 {
		cfg.reconnectInterval = defaultRabbitMQReconnectInterval
	}
	return cfg
}
