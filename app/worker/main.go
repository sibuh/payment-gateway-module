package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	rabbitmq "pgm/internal/queue"
	"pgm/internal/repo"
	"pgm/internal/repo/db"
	service "pgm/internal/service"
	"syscall"
)

func main() {
	// Database
	pool, err := repo.NewPool(context.Background(), "")
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Repository
	queries := db.New(pool)

	// UseCase
	// Worker doesn't need to publish messages, so we can pass nil for publisher
	// or a mock if needed. In our case, Process doesn't use publisher.
	uc := service.NewPaymentService(queries, nil)

	// RabbitMQ Consumer
	consumer, err := rabbitmq.NewRabbitMQConsumer(uc)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer consumer.Close()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down worker...")
		cancel()
	}()

	// Start consumer
	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("failed to start consumer: %v", err)
	}
}
