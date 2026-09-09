package memcached

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/bradfitz/gomemcache/memcache"
	"go.uber.org/zap"
)

// Client is a thread-safe wrapper around memcache.Client.
// Construct with NewClient (unconnected), then Start.
type Client struct {
	mu           sync.RWMutex
	client       *memcache.Client
	address      string
	startTimeout time.Duration
	ctx          context.Context
	log          *zap.Logger
	circuit      gtk.Circuit
	connectOpts  []gtk.BackoffOption
}

// NewClient creates an unconnected client. Call Start to connect.
// ctx is the parent for connect/backoff in Start. Nil means context.Background.
func NewClient(ctx context.Context, address string, opts ...Option) *Client {
	if ctx == nil {
		ctx = context.Background()
	}
	o := applyOptions(opts)
	return &Client{
		ctx:          ctx,
		address:      address,
		startTimeout: o.startTimeout,
		log:          o.logger,
		circuit:      o.circuit,
		connectOpts:  o.connectOpts,
	}
}

// Start connects with backoff and stores the handle.
func (m *Client) Start() error {
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
	if err := m.dial(ctx); err != nil {
		return err
	}
	return nil
}

// Stop clears the handle.
func (m *Client) Stop() error {
	if m == nil {
		return nil
	}
	m.SetClient(nil)
	m.log.Info("memcached disconnected")
	return nil
}

func (m *Client) dial(ctx context.Context) error {
	client, err := m.connectWithBackoff(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to memcached: %w", err)
	}
	m.SetClient(client)
	m.log.Info("memcached connected", zap.String("address", m.address))
	return nil
}

func (m *Client) connectWithBackoff(ctx context.Context) (*memcache.Client, error) {
	return connectWithBackoff(ctx, m.address, gtk.DefaultConnectBackoff(m.log, m.connectOpts...)...)
}

// SetClient updates the underlying memcached client.
func (m *Client) SetClient(client *memcache.Client) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client = client
}

// GetClient safely retrieves the current memcached client.
func (m *Client) GetClient() *memcache.Client {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// Get retrieves an item from memcached (delegates to underlying client).
func (m *Client) Get(key string) (*memcache.Item, error) {
	return gtk.ExecVal(m.circuit, func() (*memcache.Item, error) {
		client := m.GetClient()
		if client == nil {
			return nil, memcache.ErrCacheMiss
		}
		return client.Get(key)
	})
}

// Set stores an item in memcached (delegates to underlying client).
func (m *Client) Set(item *memcache.Item) error {
	return gtk.ExecErr(m.circuit, func() error {
		client := m.GetClient()
		if client == nil {
			return nil
		}
		return client.Set(item)
	})
}

// Delete removes an item from memcached (delegates to underlying client).
func (m *Client) Delete(key string) error {
	return gtk.ExecErr(m.circuit, func() error {
		client := m.GetClient()
		if client == nil {
			return nil
		}
		return client.Delete(key)
	})
}

var _ gtk.ManagedClient = (*Client)(nil)
