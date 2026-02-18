package logger

// ChannelWriter is a custom io.Writer that writes log entries to a buffered channel.
// It implements the io.Writer interface to be compatible with zap's WriteSyncer.
type ChannelWriter struct {
	logChan chan<- []byte
}

// NewChannelWriter creates a new channel writer that writes to the provided buffered channel.
// The channel should be buffered to avoid blocking writes.
func NewChannelWriter(logChan chan<- []byte) *ChannelWriter {
	return &ChannelWriter{
		logChan: logChan,
	}
}

// Write implements the io.Writer interface.
// It sends the provided bytes to the log channel.
func (cw *ChannelWriter) Write(p []byte) (n int, err error) {
	// Make a copy of the log entry to avoid data races
	logEntry := make([]byte, len(p))
	copy(logEntry, p)

	// Send to channel
	select {
	case cw.logChan <- logEntry:
		return len(p), nil
	default:
		// Channel is full, drop the log entry to avoid blocking
		return len(p), nil
	}
}

// Sync implements the zapcore.WriteSyncer interface.
// For channel writes, sync is a no-op.
func (cw *ChannelWriter) Sync() error {
	return nil
}
