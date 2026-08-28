package gtk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	defaultRabbitMQConnectTimeout    = 15 * time.Second
	defaultRabbitMQReconnectInterval = 500 * time.Millisecond
)

// RabbitMQOption configures NewRabbitMQClient.
type RabbitMQOption interface {
	applyRabbitMQ(*rabbitMQConfig)
}

type rabbitMQConfig struct {
	shared            sharedConfig
	connectTimeout    time.Duration
	reconnectInterval time.Duration
	topology          RabbitMQTopology
}

type rabbitMQOptionFunc func(*rabbitMQConfig)

func (f rabbitMQOptionFunc) applyRabbitMQ(c *rabbitMQConfig) { f(c) }

// WithConnectTimeout sets the per-dial budget in the reconnect loop.
// Zero or omitted uses 15s.
func WithConnectTimeout(d time.Duration) RabbitMQOption {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		if d > 0 {
			c.connectTimeout = d
		}
	})
}

// WithReconnectInterval sets the delay between reconnect attempts.
// Zero or omitted uses 500ms.
func WithReconnectInterval(d time.Duration) RabbitMQOption {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		if d > 0 {
			c.reconnectInterval = d
		}
	})
}

// WithTopology declares exchanges, queues, and bindings on every new channel.
func WithTopology(t RabbitMQTopology) RabbitMQOption {
	return rabbitMQOptionFunc(func(c *rabbitMQConfig) {
		c.topology = t
	})
}

func applyRabbitMQOptions(opts []RabbitMQOption) rabbitMQConfig {
	cfg := rabbitMQConfig{shared: defaultShared()}
	for _, opt := range opts {
		if opt != nil {
			opt.applyRabbitMQ(&cfg)
		}
	}
	finalizeShared(&cfg.shared)
	if cfg.connectTimeout == 0 {
		cfg.connectTimeout = defaultRabbitMQConnectTimeout
	}
	if cfg.reconnectInterval == 0 {
		cfg.reconnectInterval = defaultRabbitMQReconnectInterval
	}
	return cfg
}

// RabbitMQClient is a thread-safe wrapper around an AMQP connection and channel.
// Construct with NewRabbitMQClient (unconnected), then Start.
// Start reconnects on AMQP NotifyClose until Stop or ctx cancel.
type RabbitMQClient struct {
	mu                sync.RWMutex
	chMu              sync.Mutex
	conn              *amqp091.Connection
	channel           *amqp091.Channel
	url               string
	connectTimeout    time.Duration
	reconnectInterval time.Duration
	topology          RabbitMQTopology
	Connected         *EventBusTopic[struct{}]
	Disconnected      *EventBusTopic[struct{}]
	ctx               context.Context
	log               *zap.Logger
	circuit           Circuit
	done              chan struct{}
	connClosed        chan struct{}
	chanClosed        chan struct{}
	running           bool
}

// NewRabbitMQClient creates an unconnected client. Call Start to connect.
// ctx is the parent for connect/reconnect. Nil means context.Background.
func NewRabbitMQClient(ctx context.Context, url string, opts ...RabbitMQOption) *RabbitMQClient {
	if ctx == nil {
		ctx = context.Background()
	}
	o := applyRabbitMQOptions(opts)
	log := o.shared.logger
	if log == nil {
		log = LoggerFromContext(ctx)
	}
	return &RabbitMQClient{
		ctx:               ctx,
		url:               url,
		connectTimeout:    o.connectTimeout,
		reconnectInterval: o.reconnectInterval,
		topology:          o.topology,
		Connected:         NewEventBusTopic[struct{}](ctx),
		Disconnected:      NewEventBusTopic[struct{}](ctx),
		log:               log,
		circuit:           o.shared.circuit,
	}
}

// Start begins the reconnect loop. The first dial runs in the background.
func (r *RabbitMQClient) Start() error {
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
func (r *RabbitMQClient) Stop() error {
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

func (r *RabbitMQClient) reconnectLoop() {
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

func (r *RabbitMQClient) dial(ctx context.Context) error {
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

func (r *RabbitMQClient) openChannel() error {
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

func (r *RabbitMQClient) watchClose(notify <-chan *amqp091.Error, closed chan struct{}, kind string) {
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

func (r *RabbitMQClient) closeChannel() {
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

func (r *RabbitMQClient) dropConnection() {
	r.mu.Lock()
	conn := r.conn
	r.conn = nil
	r.mu.Unlock()
	if conn != nil && !conn.IsClosed() {
		_ = conn.Close()
	}
}

func (r *RabbitMQClient) teardown() {
	r.closeChannel()
	r.dropConnection()
	r.log.Info("rabbitmq disconnected")
}

func (r *RabbitMQClient) connAlive() bool {
	r.mu.RLock()
	conn := r.conn
	r.mu.RUnlock()
	return conn != nil && !conn.IsClosed()
}

func (r *RabbitMQClient) connectWithBackoff(ctx context.Context) (*amqp091.Connection, error) {
	return connectRabbitMQWithBackoff(ctx, r.url, WithPermanentErrorLogLevel(zapcore.ErrorLevel))
}

func (r *RabbitMQClient) stopped() bool {
	select {
	case <-r.done:
		return true
	case <-r.ctx.Done():
		return true
	default:
		return false
	}
}

func (r *RabbitMQClient) wait(d time.Duration) bool {
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
func (r *RabbitMQClient) Connection() *amqp091.Connection {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn
}

// GetChannel returns the current AMQP channel, or nil if disconnected.
func (r *RabbitMQClient) GetChannel() *amqp091.Channel {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel
}

// IsConnected reports whether a live connection and channel are available.
func (r *RabbitMQClient) IsConnected() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel != nil && r.conn != nil && !r.conn.IsClosed()
}

func (r *RabbitMQClient) currentChannel() *amqp091.Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel
}

// PublishJSON marshals message and publishes it as a persistent JSON AMQP message.
func (r *RabbitMQClient) PublishJSON(exchange, routingKey string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return execErr(r.circuit, func() error {
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
func (r *RabbitMQClient) QueueBind(queue, key, exchange string) error {
	return execErr(r.circuit, func() error {
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
func (r *RabbitMQClient) QueueUnbind(queue, key, exchange string) error {
	return execErr(r.circuit, func() error {
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
func (r *RabbitMQClient) Consume(queue string) (<-chan amqp091.Delivery, error) {
	return execVal(r.circuit, func() (<-chan amqp091.Delivery, error) {
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
func (r *RabbitMQClient) Ack(d amqp091.Delivery) error {
	return execErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		return d.Ack(false)
	})
}

// Nack negatively acknowledges a delivery on the current channel.
func (r *RabbitMQClient) Nack(d amqp091.Delivery, requeue bool) error {
	return execErr(r.circuit, func() error {
		r.chMu.Lock()
		defer r.chMu.Unlock()
		return d.Nack(false, requeue)
	})
}

var _ ManagedClient = (*RabbitMQClient)(nil)
