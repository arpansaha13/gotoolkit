package gtk

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// BackoffOption is a functional option for configuring backoff behavior
type BackoffOption func(*BackoffConfig)

// BackoffConfig holds the configuration for exponential backoff retry logic
type BackoffConfig struct {
	MaxRetries             int           // 0 = unlimited (bounded by MaxElapsedTime)
	PermanentErrorLogLevel zapcore.Level // default: zapcore.FatalLevel
	Logger                 *zap.Logger
}

func defaultBackoffConfig() *BackoffConfig {
	return &BackoffConfig{
		MaxRetries:             0,
		PermanentErrorLogLevel: zapcore.FatalLevel,
		Logger:                 zap.NewNop(),
	}
}

// WithMaxRetries sets the maximum number of retry attempts
// If maxRetries > 0, backoff will stop after that many attempts
// If 0, retry is bounded only by MaxElapsedTime (default 15 minutes)
func WithMaxRetries(n int) BackoffOption {
	return func(cfg *BackoffConfig) {
		cfg.MaxRetries = n
	}
}

// WithPermanentErrorLogLevel sets the log level for permanent errors
// (when MaxElapsedTime exhausted, ctx cancelled, or Permanent returned)
// Default is FatalLevel
func WithPermanentErrorLogLevel(level zapcore.Level) BackoffOption {
	return func(cfg *BackoffConfig) {
		cfg.PermanentErrorLogLevel = level
	}
}

// WithBackoffLogger sets the logger used by connect-with-backoff helpers.
// Nil is ignored. Omitted uses zap.NewNop.
func WithBackoffLogger(log *zap.Logger) BackoffOption {
	return func(cfg *BackoffConfig) {
		if log != nil {
			cfg.Logger = log
		}
	}
}

// ApplyBackoff applies all options to create a final BackoffConfig
func ApplyBackoff(opts []BackoffOption) *BackoffConfig {
	cfg := defaultBackoffConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return cfg
}

// DefaultConnectBackoff is what managed clients pass when the app does not
// supply With*Backoff options. Extra opts (from With*Backoff) override these.
func DefaultConnectBackoff(log *zap.Logger, extra ...BackoffOption) []BackoffOption {
	out := []BackoffOption{
		WithPermanentErrorLogLevel(zapcore.ErrorLevel),
		WithBackoffLogger(log),
	}
	return append(out, extra...)
}
