package gtk

// PostgresOption configures NewPostgresClient.
type PostgresOption interface {
	applyPostgres(*postgresConfig)
}

type postgresConfig struct {
	shared      sharedConfig
	connectOpts []BackoffOption
}

type postgresOptionFunc func(*postgresConfig)

func (f postgresOptionFunc) applyPostgres(c *postgresConfig) { f(c) }

// WithPostgresBackoff sets backoff options for connectPostgresWithBackoff.
// The constructor logger is prepended; a WithBackoffLogger here overrides it.
func WithPostgresBackoff(opts ...BackoffOption) PostgresOption {
	return postgresOptionFunc(func(c *postgresConfig) {
		c.connectOpts = opts
	})
}

func applyPostgresOptions(opts []PostgresOption) postgresConfig {
	cfg := postgresConfig{shared: defaultShared()}
	for _, opt := range opts {
		if opt != nil {
			opt.applyPostgres(&cfg)
		}
	}
	finalizeShared(&cfg.shared)
	return cfg
}
