package gtk

import (
	"time"
)

// MemcachedOption configures NewMemcachedClient.
type MemcachedOption interface {
	applyMemcached(*memcachedConfig)
}

type memcachedConfig struct {
	shared       sharedConfig
	startTimeout time.Duration
	connectOpts  []BackoffOption
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
func WithMemcachedBackoff(opts ...BackoffOption) MemcachedOption {
	return memcachedOptionFunc(func(c *memcachedConfig) {
		c.connectOpts = opts
	})
}

func applyMemcachedOptions(opts []MemcachedOption) memcachedConfig {
	cfg := memcachedConfig{shared: defaultShared()}
	for _, opt := range opts {
		if opt != nil {
			opt.applyMemcached(&cfg)
		}
	}
	finalizeShared(&cfg.shared)
	return cfg
}
