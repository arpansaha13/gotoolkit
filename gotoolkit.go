package gotoolkit

import (
	"context"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/arpansaha13/gotoolkit/connect"
	"github.com/arpansaha13/gotoolkit/logger"
)

// BackoffOption is a functional option for configuring backoff behavior in connect utilities
type BackoffOption = connect.BackoffOption

// Connect utility options
var (
	WithMaxRetries             = connect.WithMaxRetries
	WithPermanentErrorLogLevel = connect.WithPermanentErrorLogLevel
)

// ConnectPostgresWithBackoff connects to PostgreSQL with exponential backoff retry logic
func ConnectPostgresWithBackoff(ctx context.Context, dsn string, gormCfg *gorm.Config, opts ...BackoffOption) (*gorm.DB, error) {
	return connect.ConnectPostgresWithBackoff(ctx, dsn, gormCfg, opts...)
}

// ConnectRabbitMQWithBackoff connects to RabbitMQ with exponential backoff retry logic
func ConnectRabbitMQWithBackoff(ctx context.Context, url string, opts ...BackoffOption) (*amqp091.Connection, error) {
	return connect.ConnectRabbitMQWithBackoff(ctx, url, opts...)
}

// ConnectKafkaWithBackoff connects to Kafka with exponential backoff retry logic
func ConnectKafkaWithBackoff(ctx context.Context, cfg kafka.WriterConfig, opts ...BackoffOption) (*kafka.Writer, error) {
	return connect.ConnectKafkaWithBackoff(ctx, cfg, opts...)
}

// ConnectMemcachedWithBackoff creates a Memcached client and verifies connectivity
// with exponential backoff retry logic
func ConnectMemcachedWithBackoff(ctx context.Context, address string, opts ...BackoffOption) (*memcache.Client, error) {
	return connect.ConnectMemcachedWithBackoff(ctx, address, opts...)
}

type GormLogger = logger.GormLogger

// NewGormLogger creates a GormLogger backed by l. A Warn level will log slow
// queries and errors; Info level additionally logs every query.
func NewGormLogger(l *zap.Logger, level gormlogger.LogLevel) *GormLogger {
	return logger.NewGormLogger(l, level)
}
