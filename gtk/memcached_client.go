package gtk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MemcachedOption configures NewMemcachedClient.
type MemcachedOption interface {
	applyMemcached(*memcachedConfig)
}

type memcachedConfig struct {
	shared       sharedConfig
	startTimeout time.Duration
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

// MemcachedClient is a thread-safe wrapper around memcache.Client.
// Construct with NewMemcachedClient (unconnected), then Start.
type MemcachedClient struct {
	mu           sync.RWMutex
	client       *memcache.Client
	address      string
	startTimeout time.Duration
	ctx          context.Context
	log          *zap.Logger
	circuit      Circuit
}

// NewMemcachedClient creates an unconnected client. Call Start to connect.
// ctx is the parent for connect/backoff in Start. Nil means context.Background.
func NewMemcachedClient(ctx context.Context, address string, opts ...MemcachedOption) *MemcachedClient {
	if ctx == nil {
		ctx = context.Background()
	}
	o := applyMemcachedOptions(opts)
	return &MemcachedClient{
		ctx:          ctx,
		address:      address,
		startTimeout: o.startTimeout,
		log:          o.shared.logger,
		circuit:      o.shared.circuit,
	}
}

// Start connects with backoff and stores the handle.
func (m *MemcachedClient) Start() error {
	if m == nil {
		return fmt.Errorf("memcached client is nil")
	}
	if m.address == "" {
		return fmt.Errorf("memcached address is required")
	}
	ctx := m.ctx
	if m.startTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.startTimeout)
		defer cancel()
	}
	if err := m.connect(ctx); err != nil {
		return err
	}
	return nil
}

// Stop clears the handle.
func (m *MemcachedClient) Stop() error {
	if m == nil {
		return nil
	}
	m.SetClient(nil)
	m.log.Info("memcached disconnected")
	return nil
}

func (m *MemcachedClient) connect(ctx context.Context) error {
	client, err := m.connectWithBackoff(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to memcached: %w", err)
	}
	m.SetClient(client)
	m.log.Info("memcached connected", zap.String("address", m.address))
	return nil
}

func (m *MemcachedClient) connectWithBackoff(ctx context.Context) (*memcache.Client, error) {
	return connectMemcachedWithBackoff(ctx, m.address,
		WithPermanentErrorLogLevel(zapcore.ErrorLevel),
		WithBackoffLogger(m.log),
	)
}

// SetClient updates the underlying memcached client.
func (m *MemcachedClient) SetClient(client *memcache.Client) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client = client
}

// GetClient safely retrieves the current memcached client.
func (m *MemcachedClient) GetClient() *memcache.Client {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// Get retrieves an item from memcached (delegates to underlying client).
func (m *MemcachedClient) Get(key string) (*memcache.Item, error) {
	return execVal(m.circuit, func() (*memcache.Item, error) {
		client := m.GetClient()
		if client == nil {
			return nil, memcache.ErrCacheMiss
		}
		return client.Get(key)
	})
}

// Set stores an item in memcached (delegates to underlying client).
func (m *MemcachedClient) Set(item *memcache.Item) error {
	return execErr(m.circuit, func() error {
		client := m.GetClient()
		if client == nil {
			return nil
		}
		return client.Set(item)
	})
}

// Delete removes an item from memcached (delegates to underlying client).
func (m *MemcachedClient) Delete(key string) error {
	return execErr(m.circuit, func() error {
		client := m.GetClient()
		if client == nil {
			return nil
		}
		return client.Delete(key)
	})
}

var _ ManagedClient = (*MemcachedClient)(nil)
