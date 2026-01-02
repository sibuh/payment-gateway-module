package repo

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a connection pool and runs migrations.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	// 1. Parse the connection string into a config struct
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 2. Set the custom pool settings
	// (Example settings - tune these for your application)

	// MaxConns: Set to 10 or 4x NumCPU, whichever is greater
	config.MaxConns = int32(max(10, runtime.NumCPU()*4))

	config.MinConns = int32(2)                 // Warm the pool with 2 connections
	config.MaxConnLifetime = 15 * time.Minute  // Recycle connections every 15 mins
	config.MaxConnIdleTime = 5 * time.Minute   // Close idle connections after 5 mins
	config.HealthCheckPeriod = 1 * time.Minute // Ping idle conns every minute

	// You can also set connection-level settings
	config.ConnConfig.ConnectTimeout = 5 * time.Second

	log.Printf("Creating pool with MaxConns=%d, MinConns=%d", config.MaxConns, config.MinConns)

	// 3. Create the pool using the modified config
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// 4. Ping the database to verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close() // Close the pool if ping fails
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
