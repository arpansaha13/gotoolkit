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

// RabbitMQClientConfig holds settings used by Start.
// Topology is declared on every new channel, before OnConnect.
// OnConnect runs after the channel is open and topology is applied.
// OnDisconnect runs before the channel and connection are closed.
// Zero ConnectTimeout / ReconnectInterval use 15s / 500ms.
type RabbitMQClientConfig struct {
	URL               string
	ConnectTimeout    time.Duration
	ReconnectInterval time.Duration
	Topology          RabbitMQTopology
	OnConnect         func() error
	OnDisconnect      func()
}

// RabbitMQClient is a thread-safe wrapper around an AMQP connection and channel.
// Construct with NewRabbitMQClient (unconnected), then Start.
// Start reconnects on AMQP NotifyClose until Stop or ctx cancel.
type RabbitMQClient struct {
	mu         sync.RWMutex
	chMu       sync.Mutex
	conn       *amqp091.Connection
	channel    *amqp091.Channel
	cfg        RabbitMQClientConfig
	ctx        context.Context
	log        *zap.Logger
	done       chan struct{}
	connClosed chan struct{}
	chanClosed chan struct{}
	running    bool
}

// NewRabbitMQClient creates an unconnected client. Call Start to connect.
// ctx is the parent for connect/reconnect. Nil means context.Background.
func NewRabbitMQClient(ctx context.Context, cfg RabbitMQClientConfig, log *zap.Logger) *RabbitMQClient {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = LoggerFromContext(ctx)
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaultRabbitMQConnectTimeout
	}
	if cfg.ReconnectInterval == 0 {
		cfg.ReconnectInterval = defaultRabbitMQReconnectInterval
	}
	return &RabbitMQClient{ctx: ctx, cfg: cfg, log: log}
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
			connectCtx, cancel := context.WithTimeout(r.ctx, r.cfg.ConnectTimeout)
			err := r.dial(connectCtx)
			cancel()
			if err != nil {
				r.log.Error("rabbitmq connect failed", zap.Error(err))
				if !r.wait(r.cfg.ReconnectInterval) {
					return
				}
				continue
			}
			r.log.Info("rabbitmq connected")
		}

		if err := r.openChannel(); err != nil {
			r.log.Error("rabbitmq channel open failed", zap.Error(err))
			r.teardown()
			if !r.wait(r.cfg.ReconnectInterval) {
				return
			}
			continue
		}

		if r.cfg.OnConnect != nil {
			if err := r.cfg.OnConnect(); err != nil {
				r.log.Error("rabbitmq on-connect failed", zap.Error(err))
				r.closeChannel()
				if !r.wait(r.cfg.ReconnectInterval) {
					return
				}
				continue
			}
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

	if err := declareTopology(ch, r.cfg.Topology); err != nil {
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
	if r.cfg.OnDisconnect != nil {
		r.cfg.OnDisconnect()
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
	return connectRabbitMQWithBackoff(ctx, r.cfg.URL, WithPermanentErrorLogLevel(zapcore.ErrorLevel))
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
}

// QueueBind adds a routing-key binding on the current channel.
func (r *RabbitMQClient) QueueBind(queue, key, exchange string) error {
	r.chMu.Lock()
	defer r.chMu.Unlock()
	ch := r.currentChannel()
	if ch == nil {
		return fmt.Errorf("rabbitmq channel not initialized")
	}
	return ch.QueueBind(queue, key, exchange, false, nil)
}

// QueueUnbind removes a routing-key binding on the current channel.
func (r *RabbitMQClient) QueueUnbind(queue, key, exchange string) error {
	r.chMu.Lock()
	defer r.chMu.Unlock()
	ch := r.currentChannel()
	if ch == nil {
		return fmt.Errorf("rabbitmq channel not initialized")
	}
	return ch.QueueUnbind(queue, key, exchange, nil)
}

// Consume starts a consumer on queue. The caller must Ack/Nack deliveries.
func (r *RabbitMQClient) Consume(queue string) (<-chan amqp091.Delivery, error) {
	r.chMu.Lock()
	defer r.chMu.Unlock()
	ch := r.currentChannel()
	if ch == nil {
		return nil, fmt.Errorf("rabbitmq channel not initialized")
	}
	return ch.Consume(queue, "", false, false, false, false, nil)
}

// Ack acknowledges a delivery on the current channel.
func (r *RabbitMQClient) Ack(d amqp091.Delivery) error {
	r.chMu.Lock()
	defer r.chMu.Unlock()
	return d.Ack(false)
}

// Nack negatively acknowledges a delivery on the current channel.
func (r *RabbitMQClient) Nack(d amqp091.Delivery, requeue bool) error {
	r.chMu.Lock()
	defer r.chMu.Unlock()
	return d.Nack(false, requeue)
}

var _ ManagedClient = (*RabbitMQClient)(nil)
