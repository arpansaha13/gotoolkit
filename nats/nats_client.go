package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/cenkalti/backoff/v5"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// ErrAlreadySubscribed is returned when Subscribe is called for a subject
// that this client already owns.
var ErrAlreadySubscribed = errors.New("already subscribed")

// Client is a thread-safe wrapper around a Core NATS connection.
// Construct with NewClient (unconnected), then Start.
// Reconnect is handled by nats.go; CustomReconnectDelay uses exponential backoff.
type Client struct {
	mu           sync.RWMutex
	nc           *nats.Conn
	url          string
	ctx          context.Context
	log          *zap.Logger
	circuit      gtk.Circuit
	Connected    *gtk.EventBusTopic[struct{}]
	Disconnected *gtk.EventBusTopic[struct{}]
	subs         map[string]*nats.Subscription
	reconnect    *backoff.ExponentialBackOff
	done         chan struct{}
	running      bool
}

// NewClient creates an unconnected client. Call Start to connect.
// ctx is the parent for shutdown. Nil means context.Background.
func NewClient(ctx context.Context, url string, opts ...Option) *Client {
	if ctx == nil {
		ctx = context.Background()
	}
	o := applyOptions(opts)
	bo := backoff.NewExponentialBackOff()
	bo.Reset()
	return &Client{
		ctx:          ctx,
		url:          url,
		log:          o.logger,
		circuit:      o.circuit,
		Connected:    gtk.NewEventBusTopic[struct{}](ctx),
		Disconnected: gtk.NewEventBusTopic[struct{}](ctx),
		subs:         make(map[string]*nats.Subscription),
		reconnect:    bo,
	}
}

// Start connects to NATS. RetryOnFailedConnect is enabled so a down server
// does not fail Start; nats.go keeps trying in the background.
func (c *Client) Start() error {
	if c == nil {
		return fmt.Errorf("nats client is nil")
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("nats client already started")
	}
	c.running = true
	c.done = make(chan struct{})
	c.mu.Unlock()

	nc, err := nats.Connect(c.url,
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
		nats.CustomReconnectDelay(func(int) time.Duration {
			return c.nextReconnectDelay()
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				c.log.Warn("nats disconnected", zap.Error(err))
			}
			if c.Disconnected != nil {
				c.Disconnected.Publish(struct{}{})
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			c.resetReconnectBackoff()
			c.log.Info("nats reconnected")
			if c.Connected != nil {
				c.Connected.Publish(struct{}{})
			}
		}),
		nats.ConnectHandler(func(_ *nats.Conn) {
			c.resetReconnectBackoff()
			c.log.Info("nats connected")
			if c.Connected != nil {
				c.Connected.Publish(struct{}{})
			}
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			c.log.Info("nats connection closed")
		}),
	)
	if err != nil {
		c.mu.Lock()
		c.running = false
		close(c.done)
		c.mu.Unlock()
		return fmt.Errorf("failed to connect to nats: %w", err)
	}

	c.mu.Lock()
	c.nc = nc
	c.mu.Unlock()

	go c.watchContext()
	return nil
}

// Stop drains in-flight messages and closes the connection.
func (c *Client) Stop() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	close(c.done)
	nc := c.nc
	c.nc = nil
	c.subs = make(map[string]*nats.Subscription)
	c.mu.Unlock()
	if nc == nil {
		return nil
	}
	if err := nc.Drain(); err != nil {
		nc.Close()
	}
	return nil
}

func (c *Client) watchContext() {
	select {
	case <-c.ctx.Done():
		_ = c.Stop()
	case <-c.done:
	}
}

// IsConnected reports whether the NATS connection is live.
func (c *Client) IsConnected() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	nc := c.nc
	c.mu.RUnlock()
	return nc != nil && nc.IsConnected()
}

// Conn returns the current NATS connection, or nil if disconnected.
func (c *Client) Conn() *nats.Conn {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nc
}

// Publish sends data on subject.
func (c *Client) Publish(subject string, data []byte) error {
	return gtk.ExecErr(c.circuit, func() error {
		c.mu.RLock()
		nc := c.nc
		c.mu.RUnlock()
		if nc == nil || !nc.IsConnected() {
			return fmt.Errorf("nats not connected")
		}
		return nc.Publish(subject, data)
	})
}

// PublishJSON marshals message and publishes it on subject.
func (c *Client) PublishJSON(subject string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return c.Publish(subject, body)
}

// Subscribe registers handler for subject. One subscription per subject.
func (c *Client) Subscribe(subject string, handler func([]byte)) error {
	if c == nil {
		return fmt.Errorf("nats client is nil")
	}
	if handler == nil {
		return fmt.Errorf("nats subscribe handler is nil")
	}
	return gtk.ExecErr(c.circuit, func() error {
		c.mu.Lock()
		if _, ok := c.subs[subject]; ok {
			c.mu.Unlock()
			return fmt.Errorf("%w: %q", ErrAlreadySubscribed, subject)
		}
		nc := c.nc
		c.mu.Unlock()
		if nc == nil {
			return fmt.Errorf("nats not connected")
		}
		sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			if msg != nil {
				handler(msg.Data)
			}
		})
		if err != nil {
			return fmt.Errorf("subscribe %q: %w", subject, err)
		}
		c.mu.Lock()
		if _, ok := c.subs[subject]; ok {
			c.mu.Unlock()
			_ = sub.Unsubscribe()
			return fmt.Errorf("%w: %q", ErrAlreadySubscribed, subject)
		}
		c.subs[subject] = sub
		c.mu.Unlock()
		return nil
	})
}

// Unsubscribe removes the subscription for subject. No-op if not subscribed.
func (c *Client) Unsubscribe(subject string) error {
	if c == nil {
		return nil
	}
	return gtk.ExecErr(c.circuit, func() error {
		c.mu.Lock()
		sub, ok := c.subs[subject]
		if ok {
			delete(c.subs, subject)
		}
		c.mu.Unlock()
		if !ok || sub == nil {
			return nil
		}
		if err := sub.Unsubscribe(); err != nil {
			return fmt.Errorf("unsubscribe %q: %w", subject, err)
		}
		return nil
	})
}

func (c *Client) nextReconnectDelay() time.Duration {
	c.mu.Lock()
	d := c.reconnect.NextBackOff()
	max := c.reconnect.MaxInterval
	c.mu.Unlock()
	if d <= 0 {
		d = max
	}
	c.log.Info("nats reconnect backoff", zap.Duration("delay", d))
	return d
}

func (c *Client) resetReconnectBackoff() {
	c.mu.Lock()
	c.reconnect.Reset()
	c.mu.Unlock()
}

var _ gtk.ManagedClient = (*Client)(nil)
