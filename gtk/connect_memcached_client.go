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

// MemcachedClientConfig holds settings used by Start.
// Empty Address makes Start a no-op (optional cache).
// Zero StartTimeout means Start uses the caller's context as-is.
type MemcachedClientConfig struct {
	Address      string
	StartTimeout time.Duration
}

// MemcachedClient is a thread-safe wrapper around memcache.Client.
// Construct with NewMemcachedClient (unconnected), then Start.
type MemcachedClient struct {
	mu     sync.RWMutex
	client *memcache.Client
	cfg    MemcachedClientConfig
	ctx    context.Context
	log    *zap.Logger
}

// NewMemcachedClient creates an unconnected client. Call Start to connect.
// ctx is the parent for connect/backoff in Start. Nil means context.Background.
func NewMemcachedClient(ctx context.Context, cfg MemcachedClientConfig, log *zap.Logger) *MemcachedClient {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = LoggerFromContext(ctx)
	}
	return &MemcachedClient{ctx: ctx, cfg: cfg, log: log}
}

// Enabled reports whether an address was configured.
func (m *MemcachedClient) Enabled() bool {
	return m != nil && m.cfg.Address != ""
}

// Start connects with backoff and stores the handle. No-op if Address is empty.
func (m *MemcachedClient) Start() error {
	if m == nil {
		return fmt.Errorf("memcached client is nil")
	}
	if m.cfg.Address == "" {
		if m.log != nil {
			m.log.Info("memcached not configured, skipping start")
		}
		return nil
	}
	ctx := m.ctx
	if m.cfg.StartTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.cfg.StartTimeout)
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
	m.log.Info("memcached connected", zap.String("address", m.cfg.Address))
	return nil
}

func (m *MemcachedClient) connectWithBackoff(ctx context.Context) (*memcache.Client, error) {
	return connectMemcachedWithBackoff(ctx, m.cfg.Address, WithPermanentErrorLogLevel(zapcore.ErrorLevel))
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
	client := m.GetClient()
	if client == nil {
		return nil, memcache.ErrCacheMiss
	}
	return client.Get(key)
}

// Set stores an item in memcached (delegates to underlying client).
func (m *MemcachedClient) Set(item *memcache.Item) error {
	client := m.GetClient()
	if client == nil {
		return nil
	}
	return client.Set(item)
}

// Delete removes an item from memcached (delegates to underlying client).
func (m *MemcachedClient) Delete(key string) error {
	client := m.GetClient()
	if client == nil {
		return nil
	}
	return client.Delete(key)
}

var _ ManagedClient = (*MemcachedClient)(nil)
