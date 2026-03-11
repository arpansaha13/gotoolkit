package connect

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ReconnectConfig holds configuration for ConnectionManager reconnection behavior.
type ReconnectConfig struct {
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	ConnectTimeout    time.Duration
	ReconnectInterval time.Duration
}

// DefaultReconnectConfig provides sensible defaults for reconnection.
var DefaultReconnectConfig = ReconnectConfig{
	InitialBackoff:    1 * time.Second,
	MaxBackoff:        30 * time.Second,
	ConnectTimeout:    15 * time.Second,
	ReconnectInterval: 500 * time.Millisecond,
}

// ConnectionManager manages resilient reconnection logic for any stateful connection.
// It monitors connection state and automatically attempts to reconnect on failure.
//
// Usage:
//
//	mgr := NewConnectionManager(
//	    DefaultReconnectConfig,
//	    logger,
//	    func(ctx context.Context) error {
//	        conn, err := setupConnection(ctx)
//	        if err != nil {
//	            return err
//	        }
//	        service.SetConnection(conn)
//	        // Monitor for disconnection
//	        go func() {
//	            <-conn.NotifyClose(make(chan error, 1))
//	            mgr.Signal()
//	        }()
//	        return nil
//	    },
//	    func() {
//	        service.ClearConnection()
//	    },
//	)
//	mgr.Start(ctx)
//	defer mgr.Stop()
type ConnectionManager struct {
	config      ReconnectConfig
	logger      *zap.Logger
	connectFn   func(ctx context.Context) error // User-supplied connect logic
	disconnectFn func()                           // User-supplied cleanup logic

	signalChan chan struct{}
	done       chan struct{}

	mu        sync.RWMutex
	isRunning bool
	backoff   time.Duration
}

// NewConnectionManager creates a new ConnectionManager with the given configuration.
// connectFn is called to establish the connection; it should also set up monitoring
// for connection close events and call manager.Signal() when disconnection is detected.
// disconnectFn is called during cleanup to release any resources.
func NewConnectionManager(
	config ReconnectConfig,
	logger *zap.Logger,
	connectFn func(ctx context.Context) error,
	disconnectFn func(),
) *ConnectionManager {
	if config.InitialBackoff == 0 {
		config.InitialBackoff = DefaultReconnectConfig.InitialBackoff
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = DefaultReconnectConfig.MaxBackoff
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = DefaultReconnectConfig.ConnectTimeout
	}
	if config.ReconnectInterval == 0 {
		config.ReconnectInterval = DefaultReconnectConfig.ReconnectInterval
	}

	return &ConnectionManager{
		config:       config,
		logger:       logger,
		connectFn:    connectFn,
		disconnectFn: disconnectFn,
		signalChan:   make(chan struct{}, 1),
		done:         make(chan struct{}),
		backoff:      config.InitialBackoff,
	}
}

// Start begins the connection manager's main loop. It should be called after
// creating the ConnectionManager. The first connection attempt is triggered immediately.
// Returns an error if already running.
func (m *ConnectionManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return ErrConnectionManagerAlreadyRunning
	}
	m.isRunning = true
	m.mu.Unlock()

	// Trigger initial connection attempt
	select {
	case m.signalChan <- struct{}{}:
	default:
	}

	// Start the reconnection loop
	go m.run(ctx)

	return nil
}

// Stop gracefully shuts down the connection manager. It waits up to the configured
// ConnectTimeout for any pending connection attempt to complete, then cleans up resources.
func (m *ConnectionManager) Stop() error {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return ErrConnectionManagerNotRunning
	}
	m.isRunning = false
	m.mu.Unlock()

	// Signal the run loop to exit
	close(m.done)

	// Wait for cleanup to complete (with timeout to prevent indefinite hang)
	select {
	case <-m.done:
	case <-time.After(m.config.ConnectTimeout):
		m.logger.Warn("connection manager stop timeout exceeded")
	}

	// Clean up resources
	if m.disconnectFn != nil {
		m.disconnectFn()
	}

	return nil
}

// Signal triggers a reconnection attempt. This is called when a disconnection
// is detected. It's safe to call from multiple goroutines.
func (m *ConnectionManager) Signal() {
	m.mu.RLock()
	isRunning := m.isRunning
	m.mu.RUnlock()

	if !isRunning {
		return
	}

	select {
	case m.signalChan <- struct{}{}:
	default:
		// Channel already has a pending signal; no need to add another
	}
}

// IsRunning reports whether the connection manager is currently running.
func (m *ConnectionManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// run is the main loop that processes reconnection signals and attempts connections.
func (m *ConnectionManager) run(ctx context.Context) {
	defer close(m.done)

	for {
		select {
		case <-m.done:
			return
		case <-m.signalChan:
			// Attempt to reconnect
			m.attemptConnect(ctx)
		}
	}
}

// attemptConnect tries to establish the connection with exponential backoff.
// On success, it resets the backoff. On failure, it increases backoff and retries.
func (m *ConnectionManager) attemptConnect(ctx context.Context) {
	attempt := 0

	for {
		select {
		case <-m.done:
			return
		default:
		}

		attempt++
		m.logger.Info("attempting connection", zap.Int("attempt", attempt), zap.Duration("backoff", m.backoff))

		// Create a timeout context for this connection attempt
		connectCtx, cancel := context.WithTimeout(ctx, m.config.ConnectTimeout)
		err := m.connectFn(connectCtx)
		cancel()

		if err == nil {
			// Success: reset backoff
			m.logger.Info("connection established successfully")
			m.backoff = m.config.InitialBackoff
			return
		}

		// Failure: log and increase backoff
		m.logger.Error("connection attempt failed", zap.Error(err), zap.Int("attempt", attempt))

		// Calculate next backoff with exponential increase
		nextBackoff := m.backoff * 2
		if nextBackoff > m.config.MaxBackoff {
			nextBackoff = m.config.MaxBackoff
		}
		m.backoff = nextBackoff

		// Wait before retrying
		select {
		case <-m.done:
			return
		case <-time.After(m.config.ReconnectInterval):
		}
	}
}
