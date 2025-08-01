package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
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
	RedisClient          *redis.Client
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

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: "", // no password set
		DB: 0,  // use default DB
	})

	// Ping the Redis server to ensure connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully!")
}