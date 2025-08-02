package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Configuration constants
const (
	HealthCheckInterval = 5 * time.Second
	PaymentTimeout      = 3 * time.Second
	QueueSize           = 10000
	NumWorkers          = 100
	RingBufferSize      = 50000
)

// Global Variables
var (
	DefaultProcessorURL  string
	FallbackProcessorURL string
	WorkerURL            string
	PostgresDSN          string
	PostgresPool         *pgxpool.Pool
)

func Init() {
	DefaultProcessorURL = os.Getenv("DEFAULT_PROCESSOR_URL")
	FallbackProcessorURL = os.Getenv("FALLBACK_PROCESSOR_URL")
	workerHost := os.Getenv("WORKER_HOST")
	if workerHost == "" {
		workerHost = "worker"
	}
	workerPort := os.Getenv("WORKER_PORT")
	if workerPort == "" {
		workerPort = "8081"
	}
	WorkerURL = fmt.Sprintf("http://%s:%s", workerHost, workerPort)

	PostgresDSN = os.Getenv("POSTGRES_DSN")

	if PostgresDSN == "" {
		log.Println("POSTGRES_DSN not set; skipping Postgres connection in config")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(PostgresDSN)
	if err != nil {
		log.Fatalf("Invalid POSTGRES_DSN: %v", err)
	}
	cfg.MinConns = 1
	cfg.MaxConns = 4

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("Could not connect to Postgres: %v", err)
	}
	PostgresPool = pool

	// Retry table creation with backoff
	for i := 0; i < 5; i++ {
		if _, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS payments (
            correlation_id TEXT PRIMARY KEY,
            amount NUMERIC,
            processor TEXT,
            created_at TIMESTAMPTZ DEFAULT now()
        )`); err != nil {
			log.Printf("Attempt %d: Could not ensure payments table: %v", i+1, err)
			if i < 4 {
				time.Sleep(time.Duration(i+1) * time.Second)
				continue
			}
			log.Printf("Failed to create payments table after 5 attempts, continuing anyway: %v", err)
		} else {
			break
		}
	}

	log.Println("Connected to Postgres successfully!")
}
