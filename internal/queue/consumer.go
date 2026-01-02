package rabbitmq

import (
	"context"
	"fmt"
	"os"

	"pgm/internal/domain"

	"github.com/streadway/amqp"
)

type RabbitMQConsumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
	useCase domain.PaymentService
}

func NewRabbitMQConsumer(useCase domain.PaymentService) (*RabbitMQConsumer, error) {
	url := os.Getenv("RABBITMQ_URL")
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	q, err := ch.QueueDeclare(
		"payment_processing", // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	// Set QoS to ensure fair dispatch among multiple workers
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return &RabbitMQConsumer{
		conn:    conn,
		channel: ch,
		queue:   q.Name,
		useCase: useCase,
	}, nil
}

func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.queue, // queue
		"",      // consumer
		false,   // auto-ack (set to false for manual acknowledgement)
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	go func() {
		for d := range msgs {
			paymentID := string(d.Body)
			fmt.Printf("Received a message: %s\n", paymentID)

			err := c.useCase.ProcessPayment(ctx, paymentID)
			if err != nil {
				fmt.Printf("Error processing payment %s: %v\n", paymentID, err)
				// In a real-world scenario, we might want to retry or move to a DLQ
				// For now, we'll just nack without requeue if it's a fatal error,
				// or requeue if it's transient.
				d.Nack(false, true)
			} else {
				d.Ack(false)
			}
		}
	}()

	fmt.Println(" [*] Waiting for messages. To exit press CTRL+C")
	<-ctx.Done()
	return nil
}

func (c *RabbitMQConsumer) Close() {
	c.channel.Close()
	c.conn.Close()
}
