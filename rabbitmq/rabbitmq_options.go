package rabbitmq

import (
	"fmt"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
)

const (
	defaultRabbitMQConnectTimeout    = 15 * time.Second
	defaultRabbitMQReconnectInterval = 500 * time.Millisecond
)

// Option configures NewClient.
type Option interface {
	apply(*rabbitMQConfig)
}

type rabbitMQConfig struct {
	shared            gtk.Shared
	connectTimeout    time.Duration
	reconnectInterval time.Duration
	topology          Topology
	connectOpts       []gtk.BackoffOption
}

type rabbitMQOptionFunc func(*rabbitMQConfig)

func (f rabbitMQOptionFunc) apply(c *rabbitMQConfig) { f(c) }

// WithConnectTimeout sets the per-dial budget in the reconnect loop.
// Zero or omitted uses 15s.
func WithConnectTimeout(d time.Duration) Option {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		if d > 0 {
			c.connectTimeout = d
		}
	})
}

// WithReconnectInterval sets the delay between reconnect attempts.
// Zero or omitted uses 500ms.
func WithReconnectInterval(d time.Duration) Option {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		if d > 0 {
			c.reconnectInterval = d
		}
	})
}

// WithTopology declares exchanges, queues, and bindings on every new channel.
func WithTopology(t Topology) Option {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		c.topology = t
	})
}

// WithBackoff sets backoff options for connectWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithBackoff(opts ...gtk.BackoffOption) Option {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		c.connectOpts = opts
	})
}

func applyOptions(opts []any) rabbitMQConfig {
	cfg := rabbitMQConfig{shared: gtk.DefaultShared()}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		switch v := opt.(type) {
		case gtk.Option:
			v.Apply(&cfg.shared)
		case Option:
			v.apply(&cfg)
		default:
			panic(fmt.Sprintf("rabbitmq: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	if cfg.connectTimeout == 0 {
		cfg.connectTimeout = defaultRabbitMQConnectTimeout
	}
	if cfg.reconnectInterval == 0 {
		cfg.reconnectInterval = defaultRabbitMQReconnectInterval
	}
	return cfg
}
