package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"pgm/internal/domain"

	"github.com/avast/retry-go"
	"github.com/streadway/amqp"
)

type RabbitMQConsumer struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	queue       string
	svc         domain.PaymentService
	retryOpts   []retry.Option
	maxAttempts uint
}

func NewRabbitMQConsumer(svc domain.PaymentService) (*RabbitMQConsumer, error) {
	// read env variables and set default values
	var (
		url          string
		messageQueue string
	)
	url = os.Getenv("RABBITMQ_URL")
	messageQueue = os.Getenv("MESSAGE_QUEUE")
	if messageQueue == "" {
		messageQueue = "payment_processing"
	}
	// Parse retry attempts
	attemptsStr := os.Getenv("RETRY_ATTEMPTS")
	if attemptsStr == "" {
		attemptsStr = "3" // Default to 3 attempts if not specified
	}
	attempts, err := strconv.Atoi(attemptsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RETRY_ATTEMPTS value: %v", err)
	}

	// Parse delay type
	delayType := os.Getenv("RETRY_DELAY_TYPE")
	if delayType == "" {
		delayType = "fixed" // Default to fixed delay if not specified
	}

	// Parse initial delay
	delayStr := os.Getenv("RETRY_DELAY")
	if delayStr == "" {
		delayStr = "500ms" // Default to 500ms if not specified
	}
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RETRY_DELAY value: %v", err)
	}

	// Parse max delay
	maxDelayStr := os.Getenv("RETRY_MAX_DELAY")
	if maxDelayStr == "" {
		maxDelayStr = "5s" // Default to 5s if not specified
	}
	maxDelay, err := time.ParseDuration(maxDelayStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RETRY_MAX_DELAY value: %v", err)
	}

	// Configure retry options
	retryOpts := []retry.Option{
		retry.Attempts(uint(attempts)),
	}

	switch delayType {
	case "fixed":
		retryOpts = append(retryOpts, retry.Delay(delay))
	case "backoff":
		retryOpts = append(retryOpts,
			retry.Delay(delay),
			retry.DelayType(retry.BackOffDelay),
		)
	default:
		return nil, fmt.Errorf("invalid RETRY_DELAY_TYPE: %s. Must be 'fixed' or 'backoff'", delayType)
	}

	if maxDelay > 0 {
		retryOpts = append(retryOpts, retry.MaxDelay(maxDelay))
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	q, err := ch.QueueDeclare(
		messageQueue, // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
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
		conn:        conn,
		channel:     ch,
		queue:       q.Name,
		svc:         svc,
		retryOpts:   retryOpts,
		maxAttempts: uint(attempts),
	}, nil
}

func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.queue,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	go func() {
		for d := range msgs {
			paymentID := string(d.Body)

			// Create a copy of retry options and add the dynamic ones
			opts := make([]retry.Option, len(c.retryOpts), len(c.retryOpts)+2)
			copy(opts, c.retryOpts)

			// Add dynamic options
			opts = append(opts,
				retry.RetryIf(IsRetryable),
				retry.OnRetry(func(n uint, err error) {
					log.Printf(
						"retry %d/%d for payment %s failed: %v",
						n+1,
						c.maxAttempts,
						paymentID,
						err,
					)
				}),
			)

			err := retry.Do(
				func() error {
					return c.svc.ProcessPayment(ctx, paymentID)
				},
				opts...,
			)

			if err != nil {
				log.Printf("payment %s failed permanently: %v", paymentID, err)

				//Fatal or retries exhausted → send to DLQ
				_ = d.Nack(false, false)
				continue
			}

			//Success
			_ = d.Ack(false)
		}
	}()

	log.Println(" [*] Waiting for messages")
	<-ctx.Done()
	return nil
}

func (c *RabbitMQConsumer) Close() {
	c.channel.Close()
	c.conn.Close()
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	e, ok := err.(domain.Error)

	if !ok {
		return false
	}

	if e.Code == 500 {
		return true
	}
	return false
}
