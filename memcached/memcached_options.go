package memcached

import (
	"fmt"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
)

// MemcachedOption configures NewMemcachedClient.
type MemcachedOption interface {
	applyMemcached(*memcachedConfig)
}

type memcachedConfig struct {
	shared       gtk.Shared
	startTimeout time.Duration
	connectOpts  []gtk.BackoffOption
}

type memcachedOptionFunc func(*memcachedConfig)

func (f memcachedOptionFunc) applyMemcached(c *memcachedConfig) { f(c) }

// WithStartTimeout bounds Start connect/backoff. Zero or omitted uses the
// constructor context as-is.
func WithStartTimeout(d time.Duration) MemcachedOption {
	return memcachedOptionFunc(func(c *memcachedConfig) {
		if d > 0 {
			c.startTimeout = d
		}
	})
}

// WithMemcachedBackoff sets backoff options for connectMemcachedWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithMemcachedBackoff(opts ...gtk.BackoffOption) MemcachedOption {
	return memcachedOptionFunc(func(c *memcachedConfig) {
		c.connectOpts = opts
	})
}

func applyMemcachedOptions(opts []any) memcachedConfig {
	cfg := memcachedConfig{shared: gtk.DefaultShared()}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		switch v := opt.(type) {
		case gtk.Option:
			v.Apply(&cfg.shared)
		case MemcachedOption:
			v.applyMemcached(&cfg)
		default:
			panic(fmt.Sprintf("memcached: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	return cfg
}
