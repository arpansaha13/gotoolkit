package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/arpansaha13/gotoolkit/gtk"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Client is a thread-safe wrapper around an AMQP connection and channel.
// Construct with NewClient (unconnected), then Start.
// Start reconnects on AMQP NotifyClose until Stop or ctx cancel.
type Client struct {
	mu                sync.RWMutex
	chMu              sync.Mutex
	conn              *amqp091.Connection
	channel           *amqp091.Channel
	url               string
	connectTimeout    time.Duration
	reconnectInterval time.Duration
	topology          Topology
	Connected         *gtk.EventBusTopic[struct{}]
	Disconnected      *gtk.EventBusTopic[struct{}]
	ctx               context.Context
	log               *zap.Logger
	circuit           gtk.Circuit
	connectOpts       []gtk.BackoffOption
	done              chan struct{}
	connClosed        chan struct{}
	chanClosed        chan struct{}
	running           bool
}

// NewClient creates an unconnected client. Call Start to connect.
// ctx is the parent for connect/reconnect. Nil means context.Background.
func NewClient(ctx context.Context, url string, opts ...Option) *Client {
	if ctx == nil {
		ctx = context.Background()
	}
	o := applyOptions(opts)
	return &Client{
		ctx:               ctx,
		url:               url,
		connectTimeout:    o.connectTimeout,
		reconnectInterval: o.reconnectInterval,
		topology:          o.topology,
		Connected:         gtk.NewEventBusTopic[struct{}](ctx),
		Disconnected:      gtk.NewEventBusTopic[struct{}](ctx),
		log:               o.logger,
		circuit:           o.circuit,
		connectOpts:       o.connectOpts,
	}
}

// Start begins the reconnect loop. The first dial runs in the background.
func (r *Client) Start() error {
	if r == nil {
		return fmt.Errorf("rabbitmq client is nil")
	}
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("rabbitmq client already started")
	}
	r.running = true
	r.done = make(chan struct{})
	r.mu.Unlock()
	go r.reconnectLoop()
	return nil
}

// Stop shuts down the reconnect loop and closes the connection.
func (r *Client) Stop() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	close(r.done)
	r.mu.Unlock()
	r.teardown()
	return nil
}

func (r *Client) reconnectLoop() {
	for {
		if r.stopped() {
			return
		}

		if !r.connAlive() {
			r.dropConnection()
			connectCtx, cancel := context.WithTimeout(r.ctx, r.connectTimeout)
			err := r.dial(connectCtx)
			cancel()
			if err != nil {
				r.log.Error("rabbitmq connect failed", zap.Error(err))
				if !r.wait(r.reconnectInterval) {
					return
				}
				continue
			}
			r.log.Info("rabbitmq connected")
		}

		if err := r.openChannel(); err != nil {
			r.log.Error("rabbitmq channel open failed", zap.Error(err))
			r.teardown()
			if !r.wait(r.reconnectInterval) {
				return
			}
			continue
		}

		if r.Connected != nil {
			r.Connected.Publish(struct{}{})
		}

		r.mu.RLock()
		connClosed := r.connClosed
		chanClosed := r.chanClosed
		r.mu.RUnlock()

		select {
		case <-r.done:
			return
		case <-r.ctx.Done():
			return
		case <-connClosed:
			r.log.Warn("rabbitmq connection closed, reconnecting")
			r.teardown()
		case <-chanClosed:
			select {
			case <-connClosed:
				r.log.Warn("rabbitmq connection closed, reconnecting")
				r.teardown()
			default:
				r.log.Warn("rabbitmq channel closed, reopening")
				r.closeChannel()
			}
		}
	}
}

func (r *Client) dial(ctx context.Context) error {
	conn, err := r.connectWithBackoff(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	connClosed := make(chan struct{})
	r.mu.Lock()
	r.conn = conn
	r.connClosed = connClosed
	r.mu.Unlock()

	r.watchClose(conn.NotifyClose(make(chan *amqp091.Error, 1)), connClosed, "connection")
	return nil
}

func (r *Client) openChannel() error {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()
	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection not available")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}

	if err := declareTopology(ch, r.topology); err != nil {
		_ = ch.Close()
		return err
	}

	chanClosed := make(chan struct{})
	r.mu.Lock()
	r.channel = ch
	r.chanClosed = chanClosed
	r.mu.Unlock()

	r.watchClose(ch.NotifyClose(make(chan *amqp091.Error, 1)), chanClosed, "channel")
	return nil
}

func (r *Client) watchClose(notify <-chan *amqp091.Error, closed chan struct{}, kind string) {
	go func() {
		err := <-notify
		if r.stopped() {
			return
		}
		if err != nil {
			r.log.Warn("rabbitmq "+kind+" closed", zap.Error(err))
		}
		select {
		case <-closed:
		default:
			close(closed)
		}
	}()
}

func (r *Client) closeChannel() {
	if r.Disconnected != nil {
		r.Disconnected.Publish(struct{}{})
	}
	r.mu.Lock()
	ch := r.channel
	r.channel = nil
	r.mu.Unlock()
	if ch != nil {
		_ = ch.Close()
	}
}

func (r *Client) dropConnection() {
	r.mu.Lock()
	conn := r.conn
	r.conn = nil
	r.mu.Unlock()
	if conn != nil && !conn.IsClosed() {
		_ = conn.Close()
	}
}

func (r *Client) teardown() {
	r.closeChannel()
	r.dropConnection()
	r.log.Info("rabbitmq disconnected")
}

func (r *Client) connAlive() bool {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()
	return conn != nil && !conn.IsClosed()
}

func (r *Client) connectWithBackoff(ctx context.Context) (*amqp091.Connection, error) {
	return connectWithBackoff(ctx, r.url, gtk.DefaultConnectBackoff(r.log, r.connectOpts...)...)
}

func (r *Client) stopped() bool {
	select {
	case <-r.done:
		return true
	case <-r.ctx.Done():
		return true
	default:
		return false
	}
}

func (r *Client) wait(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-r.done:
		return false
	case <-r.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Connection returns the current AMQP connection, or nil if disconnected.
func (r *Client) Connection() *amqp091.Connection {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn
}

// GetChannel returns the current AMQP channel, or nil if disconnected.
func (r *Client) GetChannel() *amqp091.Channel {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel
}

// IsConnected reports whether a live connection and channel are available.
func (r *Client) IsConnected() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel != nil && r.conn != nil && !r.conn.IsClosed()
}

func (r *Client) currentChannel() *amqp091.Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel
}

// PublishJSON marshals message and publishes it as a persistent JSON AMQP message.
func (r *Client) PublishJSON(exchange, routingKey string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return gtk.ExecErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		ch := r.currentChannel()
		if ch == nil {
			return fmt.Errorf("rabbitmq channel not initialized")
		}
		return ch.Publish(exchange, routingKey, false, false, amqp091.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp091.Persistent,
		})
	})
}

// QueueBind adds a routing-key binding on the current channel.
func (r *Client) QueueBind(queue, key, exchange string) error {
	return gtk.ExecErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		ch := r.currentChannel()
		if ch == nil {
			return fmt.Errorf("rabbitmq channel not initialized")
		}
		return ch.QueueBind(queue, key, exchange, false, nil)
	})
}

// QueueUnbind removes a routing-key binding on the current channel.
func (r *Client) QueueUnbind(queue, key, exchange string) error {
	return gtk.ExecErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		ch := r.currentChannel()
		if ch == nil {
			return fmt.Errorf("rabbitmq channel not initialized")
		}
		return ch.QueueUnbind(queue, key, exchange, nil)
	})
}

// Consume starts a consumer on queue. The caller must Ack/Nack deliveries.
func (r *Client) Consume(queue string) (<-chan amqp091.Delivery, error) {
	return gtk.ExecVal(r.circuit, func() (<-chan amqp091.Delivery, error) {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		ch := r.currentChannel()
		if ch == nil {
			return nil, fmt.Errorf("rabbitmq channel not initialized")
		}
		return ch.Consume(queue, "", false, false, false, false, nil)
	})
}

// Ack acknowledges a delivery on the current channel.
func (r *Client) Ack(d amqp091.Delivery) error {
	return gtk.ExecErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		return d.Ack(false)
	})
}

// Nack negatively acknowledges a delivery on the current channel.
func (r *Client) Nack(d amqp091.Delivery, requeue bool) error {
	return gtk.ExecErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		return d.Nack(false, requeue)
	})
}

var _ gtk.ManagedClient = (*Client)(nil)
