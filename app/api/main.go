package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"pgm/internal/domain"
	pmt "pgm/internal/handler/payment"
	q "pgm/internal/queue"
	"pgm/internal/repo"
	"pgm/internal/repo/db"
	"pgm/internal/service"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

	// Run migrations
	if err := runMigrations("file:///app/internal/repo/schema", dbName, dsn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// RabbitMQ Publisher
	publisher, err := q.NewRabbitMQPublisher()
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	// Note: In a real app, we'd handle closing the publisher gracefully

	// Repository
	queries := db.New(pool)

	// service
	uc := service.NewPaymentService(queries, publisher)

	// Echo
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout:      time.Minute,
		ErrorMessage: "Request timeout",
	}))
	e.HTTPErrorHandler = domain.ErrorHandler
	g := e.Group("/v1")
	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handlers
	pmt.NewPaymentHandler(g, uc)

	// Start server
	e.StartServer(srv)
}

// RunMigrations automatically applies migrations on startup.
func runMigrations(filePath, dbname string, dsn string) error {
	log.Println("Running migrations...")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to open temp DB for migrations: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping temp DB for migrations: %v", err)
	}

	// 2. Create a new "postgres" driver instance for migrate
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("Failed to create migrate driver instance: %v", err)
	}

	// 3. Create the migrate instance
	// Point to your migrations directory
	m, err := migrate.NewWithDatabaseInstance(
		filePath, // Source URL
		dbname,   // Database name
		driver,   // The driver instance
	)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	// 4. Run the migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while running migrations: %v", err)
	}

	log.Println("Migrations applied successfully!")
	return nil
}
