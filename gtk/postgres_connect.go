package gtk

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// connectPostgresWithBackoff opens a pgx pool and pings it with exponential
// backoff retry logic.
//
// MaxOpenConns > 0 is applied to the pool config. Zero leaves the pgxpool default.
// StartTimeout is ignored; the caller bounds ctx.
//
// The operation is retried with exponential backoff until:
//   - Success (returns *pgxpool.Pool)
//   - MaxElapsedTime exhausted (default 15 minutes)
//   - Context cancelled
//   - maxRetries exceeded (if WithMaxRetries(n) is set)
//
// Per-attempt logging:
//   - attempt <= 3: Warn level
//   - attempt > 3: Error level
//   - On permanent failure: logs at permanentErrorLogLevel (default: Fatal)
//
// The logger comes from WithBackoffLogger. Omitted uses zap.NewNop.
func connectPostgresWithBackoff(ctx context.Context, cfg PostgresClientConfig, opts ...BackoffOption) (*pgxpool.Pool, error) {
	backoffCfg := applyOptions(opts)
	l := backoffCfg.logger

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	}

	var attempt int
	operation := func() (*pgxpool.Pool, error) {
		attempt++
		pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}

		if attempt <= 3 {
			l.Warn("failed to connect to postgres", zap.Int("attempt", attempt), zap.Error(err))
		} else {
			l.Error("failed to connect to postgres", zap.Int("attempt", attempt), zap.Error(err))
		}
		if backoffCfg.maxRetries > 0 && attempt >= backoffCfg.maxRetries {
			return nil, backoff.Permanent(err)
		}
		return nil, err
	}

	retryOpts := []backoff.RetryOption{
		backoff.WithNotify(func(err error, d time.Duration) {}),
	}
	if backoffCfg.maxRetries > 0 {
		retryOpts = append(retryOpts, backoff.WithMaxTries(uint(backoffCfg.maxRetries)))
	}

	pool, retryErr := backoff.Retry(ctx, operation, retryOpts...)
	if retryErr != nil {
		l.Log(backoffCfg.permanentErrorLogLevel, "permanently failed to connect to postgres", zap.Error(retryErr))
		return nil, retryErr
	}
	return pool, nil
}
