package gtk

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

// ExchangeDecl is an AMQP exchange to declare on each new channel.
type ExchangeDecl struct {
	Name       string
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
}

// QueueDecl is an AMQP queue to declare on each new channel.
type QueueDecl struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
}

// BindingDecl binds a queue to an exchange with a routing key.
type BindingDecl struct {
	Queue    string
	Key      string
	Exchange string
	NoWait   bool
}

// RabbitMQTopology is applied after every successful Channel() open.
type RabbitMQTopology struct {
	Exchanges []ExchangeDecl
	Queues    []QueueDecl
	Bindings  []BindingDecl
}

func declareTopology(ch *amqp091.Channel, topo RabbitMQTopology) error {
	for _, e := range topo.Exchanges {
		kind := e.Kind
		if kind == "" {
			kind = "direct"
		}
		if err := ch.ExchangeDeclare(e.Name, kind, e.Durable, e.AutoDelete, e.Internal, e.NoWait, nil); err != nil {
			return fmt.Errorf("declare exchange %q: %w", e.Name, err)
		}
	}
	for _, q := range topo.Queues {
		if _, err := ch.QueueDeclare(q.Name, q.Durable, q.AutoDelete, q.Exclusive, q.NoWait, nil); err != nil {
			return fmt.Errorf("declare queue %q: %w", q.Name, err)
		}
	}
	for _, b := range topo.Bindings {
		if err := ch.QueueBind(b.Queue, b.Key, b.Exchange, b.NoWait, nil); err != nil {
			return fmt.Errorf("bind queue %q to exchange %q key %q: %w", b.Queue, b.Exchange, b.Key, err)
		}
	}
	return nil
}
