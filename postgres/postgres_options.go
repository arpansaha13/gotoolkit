package postgres

import (
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Option configures NewClient.
type Option func(*postgresConfig)

type postgresConfig struct {
	circuit      gtk.Circuit
	logger       *zap.Logger
	connectOpts  []gtk.BackoffOption
	tracer       pgx.QueryTracer
	maxOpenConns int
	startTimeout time.Duration
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

// WithMaxOpenConns sets the pool max. Zero or omitted leaves the pgxpool default.
func WithMaxOpenConns(n int) Option {
	return func(c *postgresConfig) {
		if n > 0 {
			c.maxOpenConns = n
		}
	}
}

// WithStartTimeout bounds Start connect/backoff. Zero or omitted uses the
// constructor context as-is.
func WithStartTimeout(d time.Duration) Option {
	return func(c *postgresConfig) {
		if d > 0 {
			c.startTimeout = d
		}
	}
}

// WithTracing records queries as children of the span in ctx.
// Spans use otel.GetTracerProvider() at query time. Omitted is off.
func WithTracing() Option {
	return func(c *postgresConfig) {
		c.tracer = pgxQueryTracer{}
	}
}

func applyOptions(opts []Option) postgresConfig {
	cfg := postgresConfig{
		circuit: gtk.NoopCircuit{},
		logger:  zap.NewNop(),
		tracer:  noopQueryTracer{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if t, ok := cfg.tracer.(pgxQueryTracer); ok {
		t.log = cfg.logger
		cfg.tracer = t
	}
	return cfg
}
