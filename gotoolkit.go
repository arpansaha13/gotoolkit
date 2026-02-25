package gotoolkit

import (
	"context"

	"github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"github.com/arpansaha13/gotoolkit/internal/connect"
)

// BackoffOption is a functional option for configuring backoff behavior in connect utilities
type BackoffOption = connect.BackoffOption

// Connect utility options
var (
	WithMaxRetries             = connect.WithMaxRetries
	WithPermanentErrorLogLevel = connect.WithPermanentErrorLogLevel
)

// ConnectPostgresWithBackoff connects to PostgreSQL with exponential backoff retry logic
func ConnectPostgresWithBackoff(ctx context.Context, dsn string, opts ...BackoffOption) (*gorm.DB, error) {
	return connect.ConnectPostgresWithBackoff(ctx, dsn, opts...)
}

// ConnectRabbitMQWithBackoff connects to RabbitMQ with exponential backoff retry logic
func ConnectRabbitMQWithBackoff(ctx context.Context, url string, opts ...BackoffOption) (*amqp091.Connection, error) {
	return connect.ConnectRabbitMQWithBackoff(ctx, url, opts...)
}

// ConnectKafkaWithBackoff connects to Kafka with exponential backoff retry logic
func ConnectKafkaWithBackoff(ctx context.Context, cfg kafka.WriterConfig, opts ...BackoffOption) (*kafka.Writer, error) {
	return connect.ConnectKafkaWithBackoff(ctx, cfg, opts...)
}
