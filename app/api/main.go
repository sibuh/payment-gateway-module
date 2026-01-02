package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	http "pgm/internal/handler/payment"
	rabbitmq "pgm/internal/queue"
	"pgm/internal/service"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	"pgm/internal/repo"
	"pgm/internal/repo/db"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Database
	pool, err := repo.NewPool(context.Background(), "")
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := runMigrations("", "", ""); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// RabbitMQ Publisher
	publisher, err := rabbitmq.NewRabbitMQPublisher()
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
	g := e.Group("/v1")

	// Handlers
	http.NewPaymentHandler(g, uc)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
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
