package memcached

import (
	"fmt"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
)

// Option configures NewClient.
type Option interface {
	apply(*memcachedConfig)
}

type memcachedConfig struct {
	shared       gtk.Shared
	startTimeout time.Duration
	connectOpts  []gtk.BackoffOption
}

type memcachedOptionFunc func(*memcachedConfig)

func (f memcachedOptionFunc) apply(c *memcachedConfig) { f(c) }

// WithStartTimeout bounds Start connect/backoff. Zero or omitted uses the
// constructor context as-is.
func WithStartTimeout(d time.Duration) Option {
	return memcachedOptionFunc(func(c *memcachedConfig) {
		if d > 0 {
			c.startTimeout = d
		}
	})
}

// WithBackoff sets backoff options for connectWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithBackoff(opts ...gtk.BackoffOption) Option {
	return memcachedOptionFunc(func(c *memcachedConfig) {
		c.connectOpts = opts
	})
}

func applyOptions(opts []any) memcachedConfig {
	cfg := memcachedConfig{shared: gtk.DefaultShared()}
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
			panic(fmt.Sprintf("memcached: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	return cfg
}
