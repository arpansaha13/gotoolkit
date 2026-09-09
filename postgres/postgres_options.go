package postgres

import (
	"fmt"

	"github.com/arpansaha13/gotoolkit/gtk"
)

// Option configures NewClient.
type Option interface {
	apply(*postgresConfig)
}

type postgresConfig struct {
	shared      gtk.Shared
	connectOpts []gtk.BackoffOption
}

type postgresOptionFunc func(*postgresConfig)

func (f postgresOptionFunc) apply(c *postgresConfig) { f(c) }

// WithBackoff sets backoff options for connectWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithBackoff(opts ...gtk.BackoffOption) Option {
	return postgresOptionFunc(func(c *postgresConfig) {
		c.connectOpts = opts
	})
}

func applyOptions(opts []any) postgresConfig {
	cfg := postgresConfig{shared: gtk.DefaultShared()}
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
			panic(fmt.Sprintf("postgres: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	return cfg
}
