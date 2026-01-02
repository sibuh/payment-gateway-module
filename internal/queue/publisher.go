package rabbitmq

import (
	"context"
	"fmt"
	"os"

	"pgm/internal/domain"

	"github.com/streadway/amqp"
)

type rabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

func NewRabbitMQPublisher() (domain.MessagePublisher, error) {
	url := os.Getenv("RABBITMQ_URL")
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}
	queueName := os.Getenv("MESSAGE_QUEUE")
	if queueName == "" {
		return nil, fmt.Errorf("MESSAGE_QUEUE environment variable not set")
	}

	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	return &rabbitMQPublisher{
		conn:    conn,
		channel: ch,
		queue:   q.Name,
	}, nil
}

func (p *rabbitMQPublisher) PublishPaymentCreated(ctx context.Context, paymentID string) error {
	err := p.channel.Publish(
		"",      // exchange
		p.queue, // routing key
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(paymentID),
		})
	if err != nil {
		return fmt.Errorf("failed to publish a message: %w", err)
	}
	return nil
}

func (p *rabbitMQPublisher) Close() {
	p.channel.Close()
	p.conn.Close()
}
