package main

import (
	"context"
	"fmt"
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
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "root"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5433"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "password"
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	log.Printf("Connecting to database with DSN: %s", dsn)
	pool, err := repo.NewPool(context.Background(), dsn)
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
