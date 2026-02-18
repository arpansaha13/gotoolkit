package logger

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

// KafkaLogProducer consumes log entries from a channel and produces them to a Kafka topic.
// This function is designed to run as a background goroutine.
// It will continue reading from the logChan until it is closed.
//
// Parameters:
//   - ctx: Context for managing the producer lifecycle. When cancelled, the producer will stop
//   - logChan: Buffered channel that receives log entries ([]byte)
//   - writer: Kafka writer configured with the target topic and brokers
//
// The function will log any errors that occur during Kafka writes but will not panic.
func KafkaLogProducer(ctx context.Context, logChan <-chan []byte, writer *kafka.Writer) {
	for {
		select {
		case <-ctx.Done():
			// Shutdown signal received
			if err := writer.Close(); err != nil {
				log.Printf("error closing kafka writer: %v", err)
			}
			return

		case logEntry, ok := <-logChan:
			if !ok {
				// Channel closed
				if err := writer.Close(); err != nil {
					log.Printf("error closing kafka writer: %v", err)
				}
				return
			}

			// Write log entry to Kafka
			msg := kafka.Message{
				Value: logEntry,
			}

			if err := writer.WriteMessages(ctx, msg); err != nil {
				log.Printf("error writing to kafka: %v", err)
				// Continue without panicking - logs may be lost but system continues
			}
		}
	}
}
