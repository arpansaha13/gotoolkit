package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ClientConfig holds settings used by Start. Zero MaxOpenConns leaves pgxpool default.
// Zero StartTimeout means Start uses the caller's context as-is.
type ClientConfig struct {
	DatabaseURL  string
	MaxOpenConns int
	StartTimeout time.Duration
}

// Tx is a postgres transaction.
type Tx = pgx.Tx

// Querier is the pgx surface shared by the pool and a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Client is a thread-safe wrapper around *pgxpool.Pool.
// Construct with NewClient (unconnected), then Start. Repos hold this
// type so they stay valid when the handle is set after connect.
type Client struct {
	mu          sync.RWMutex
	pool        *pgxpool.Pool
	cfg         ClientConfig
	ctx         context.Context
	circuit     gtk.Circuit
	log         *zap.Logger
	connectOpts []gtk.BackoffOption
}

// NewClient creates an unconnected client. Call Start to open the DB.
// ctx is the parent for connect/backoff in Start. Nil means context.Background.
func NewClient(ctx context.Context, cfg ClientConfig, opts ...any) *Client {
	o := applyOptions(opts)
	return &Client{
		ctx:         ctx,
		cfg:         cfg,
		circuit:     o.shared.Circuit,
		log:         o.shared.Logger,
		connectOpts: o.connectOpts,
	}
}

// Start connects with backoff and stores the handle. Safe to call once.
func (p *Client) Start() error {
	if p == nil {
		return fmt.Errorf("postgres client is nil")
	}
	ctx := p.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if p.cfg.StartTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.cfg.StartTimeout)
		defer cancel()
	}
	pool, err := p.connectWithBackoff(ctx)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	p.SetPool(pool)
	return nil
}

// Stop closes the underlying connection and clears the handle.
func (p *Client) Stop() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pool == nil {
		return nil
	}
	p.pool.Close()
	p.pool = nil
	return nil
}

// SetPool updates the underlying pool (nil clears it on disconnect).
func (p *Client) SetPool(pool *pgxpool.Pool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool = pool
}

// Pool returns the current pool, or nil if disconnected.
func (p *Client) Pool() *pgxpool.Pool {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pool
}

// Ping checks that the pool is connected and accepting queries.
func (p *Client) Ping(ctx context.Context) error {
	return gtk.ExecErr(p.circuit, func() error {
		pool := p.Pool()
		if pool == nil {
			return &NotConnectedError{}
		}
		return pool.Ping(ctx)
	})
}

// Q returns tx when non-nil, otherwise the pool.
func (p *Client) Q(tx Tx) Querier {
	if tx != nil {
		return tx
	}
	pool := p.Pool()
	if pool == nil {
		return disconnectedQuerier{}
	}
	return pool
}

// Exec runs a statement on the pool.
func (p *Client) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return gtk.ExecVal(p.circuit, func() (pgconn.CommandTag, error) {
		pool := p.Pool()
		if pool == nil {
			return pgconn.CommandTag{}, &NotConnectedError{}
		}
		return pool.Exec(ctx, sql, arguments...)
	})
}

// Query runs a query on the pool.
func (p *Client) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return gtk.ExecVal(p.circuit, func() (pgx.Rows, error) {
		pool := p.Pool()
		if pool == nil {
			return nil, &NotConnectedError{}
		}
		return pool.Query(ctx, sql, args...)
	})
}

// QueryRow runs a single-row query on the pool.
func (p *Client) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return circuitRow{
		circuit: p.circuit,
		scan: func(dest ...any) error {
			pool := p.Pool()
			if pool == nil {
				return &NotConnectedError{}
			}
			return pool.QueryRow(ctx, sql, args...).Scan(dest...)
		},
	}
}

// Begin starts a transaction on the pool.
func (p *Client) Begin(ctx context.Context) (pgx.Tx, error) {
	return gtk.ExecVal(p.circuit, func() (pgx.Tx, error) {
		pool := p.Pool()
		if pool == nil {
			return nil, &NotConnectedError{}
		}
		return pool.Begin(ctx)
	})
}

// Transaction runs fn inside a database transaction.
func (p *Client) Transaction(ctx context.Context, fn func(tx Tx) error) error {
	pool := p.Pool()
	if pool == nil {
		return &NotConnectedError{}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Client) connectWithBackoff(ctx context.Context) (*pgxpool.Pool, error) {
	return connectWithBackoff(ctx, p.cfg, gtk.DefaultConnectBackoff(p.log, p.connectOpts...)...)
}

type disconnectedQuerier struct{}

func (disconnectedQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, &NotConnectedError{}
}
func (disconnectedQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, &NotConnectedError{}
}
func (disconnectedQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return errRow{err: &NotConnectedError{}}
}

type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

type circuitRow struct {
	circuit gtk.Circuit
	scan    func(dest ...any) error
}

func (r circuitRow) Scan(dest ...any) error {
	return gtk.ExecErr(r.circuit, func() error {
		return r.scan(dest...)
	})
}

var _ gtk.ManagedClient = (*Client)(nil)
