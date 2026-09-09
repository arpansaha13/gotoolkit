package postgres

import (
	"context"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// connectWithBackoff opens a pgx pool and pings it with exponential
// backoff retry logic.
//
// maxOpenConns > 0 is applied to the pool config. Zero leaves the pgxpool default.
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
func (p *Client) connectWithBackoff(ctx context.Context) (*pgxpool.Pool, error) {
	backoffCfg := gtk.ApplyBackoff(gtk.DefaultConnectBackoff(p.log, p.connectOpts...))
	l := backoffCfg.Logger

	poolCfg, err := pgxpool.ParseConfig(p.databaseURL)
	if err != nil {
		return nil, err
	}
	if p.maxOpenConns > 0 {
		poolCfg.MaxConns = int32(p.maxOpenConns)
	}
	poolCfg.ConnConfig.Tracer = p.tracer

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
		if backoffCfg.MaxRetries > 0 && attempt >= backoffCfg.MaxRetries {
			return nil, backoff.Permanent(err)
		}
		return nil, err
	}

	retryOpts := []backoff.RetryOption{
		backoff.WithNotify(func(err error, d time.Duration) {}),
	}
	if backoffCfg.MaxRetries > 0 {
		retryOpts = append(retryOpts, backoff.WithMaxTries(uint(backoffCfg.MaxRetries)))
	}

	pool, retryErr := backoff.Retry(ctx, operation, retryOpts...)
	if retryErr != nil {
		l.Log(backoffCfg.PermanentErrorLogLevel, "permanently failed to connect to postgres", zap.Error(retryErr))
		return nil, retryErr
	}
	return pool, nil
}
