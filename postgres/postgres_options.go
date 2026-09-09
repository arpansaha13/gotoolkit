package postgres

import (
	"fmt"

	"github.com/arpansaha13/gotoolkit/gtk"
)

// PostgresOption configures NewPostgresClient.
type PostgresOption interface {
	applyPostgres(*postgresConfig)
}

type postgresConfig struct {
	shared      gtk.Shared
	connectOpts []gtk.BackoffOption
}

type postgresOptionFunc func(*postgresConfig)

func (f postgresOptionFunc) applyPostgres(c *postgresConfig) { f(c) }

// WithPostgresBackoff sets backoff options for connectPostgresWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithPostgresBackoff(opts ...gtk.BackoffOption) PostgresOption {
	return postgresOptionFunc(func(c *postgresConfig) {
		c.connectOpts = opts
	})
}

func applyPostgresOptions(opts []any) postgresConfig {
	cfg := postgresConfig{shared: gtk.DefaultShared()}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		switch v := opt.(type) {
		case gtk.Option:
			v.Apply(&cfg.shared)
		case PostgresOption:
			v.applyPostgres(&cfg)
		default:
			panic(fmt.Sprintf("postgres: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	return cfg
}
