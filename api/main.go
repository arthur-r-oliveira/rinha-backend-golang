package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	redisClient *redis.Client
	defaultProcessorURL string
	fallbackProcessorURL string
	processorHealth = make(map[string]bool)
	healthMutex sync.RWMutex
	ctx = context.Background()
)

const (
	defaultProcessor = "default"
	fallbackProcessor = "fallback"
	healthCheckInterval = 5 * time.Second
)

// PaymentRequest represents the incoming payment request
type PaymentRequest struct {
	CorrelationID string `json:"correlationId"`
	Amount        int64  `json:"amount"`
	ProcessorType string `json:"processorType"` // This will be set internally
}

// PaymentSummaryResponse represents the summary of payments
type PaymentSummaryResponse struct {
	Default  Summary `json:"default"`
	Fallback Summary `json:"fallback"`
}

// Summary represents the aggregated data for a processor
type Summary struct {
	TotalRequests int64 `json:"totalRequests"`
	TotalAmount   int64 `json:"totalAmount"`
}

// ServiceHealthResponse represents the health check response from a processor
type ServiceHealthResponse struct {
	Failing bool `json:"failing"`
	MinResponseTime int `json:"minResponseTime"`
}

func init() {
	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default for local development
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: "", // no password set
		DB: 0,        // use default DB
	})

	// Ping Redis to ensure connection
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully!")

	// Initialize processor URLs
	defaultProcessorURL = os.Getenv("DEFAULT_PROCESSOR_URL")
	fallbackProcessorURL = os.Getenv("FALLBACK_PROCESSOR_URL")

	if defaultProcessorURL == "" || fallbackProcessorURL == "" {
		log.Fatal("DEFAULT_PROCESSOR_URL and FALLBACK_PROCESSOR_URL must be set")
	}

	// Initialize processor health status
	healthMutex.Lock()
	processorHealth[defaultProcessor] = true // Assume healthy initially
	processorHealth[fallbackProcessor] = true // Assume healthy initially
	healthMutex.Unlock()

	// Start background health checks
	go startHealthChecks()
}

func main() {
	http.HandleFunc("/payments", handlePayments)
	http.HandleFunc("/payments-summary", handlePaymentsSummary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handlePayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check for duplicate correlationId (idempotency)
	exists, err := redisClient.SIsMember(ctx, "processed_payments", req.CorrelationID).Result()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Redis error checking correlationId: %v", err)
		return
	}
	if exists {
		w.WriteHeader(http.StatusOK) // Idempotent success
		return
	}

	// Determine which processor to use
	healthMutex.RLock()
	isDefaultHealthy := processorHealth[defaultProcessor]
	isFallbackHealthy := processorHealth[fallbackProcessor]
	healthMutex.RUnlock()

	var selectedProcessor string
	var selectedProcessorURL string

	if isDefaultHealthy {
		selectedProcessor = defaultProcessor
		selectedProcessorURL = defaultProcessorURL
	} else if isFallbackHealthy {
		selectedProcessor = fallbackProcessor
		selectedProcessorURL = fallbackProcessorURL
	} else {
		http.Error(w, "No available payment processors", http.StatusServiceUnavailable)
		return
	}

	// Call external payment processor
	processorReq, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Error marshalling processor request: %v", err)
		return
	}

	resp, err := http.Post(selectedProcessorURL+"/payments", "application/json", bytes.NewBuffer(processorReq))
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Payment processor failed", http.StatusServiceUnavailable)
		log.Printf("Payment processor %s failed: %v, status: %d", selectedProcessor, err, resp.StatusCode)
		return
	}
	defer resp.Body.Close()

	// Atomically update summary in Redis
	_, err = redisClient.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.IncrBy(ctx, selectedProcessor+"_total_requests", 1)
		pipe.IncrBy(ctx, selectedProcessor+"_total_amount", req.Amount)
		pipe.SAdd(ctx, "processed_payments", req.CorrelationID) // Mark as processed
		return nil
	})

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Redis transaction failed: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handlePaymentsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fetch default summary
	defaultRequests, err := redisClient.Get(ctx, defaultProcessor+"_total_requests").Int64()
	if err == redis.Nil {
		defaultRequests = 0
	} else if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Redis error fetching default requests: %v", err)
		return
	}

	defaultAmount, err := redisClient.Get(ctx, defaultProcessor+"_total_amount").Int64()
	if err == redis.Nil {
		defaultAmount = 0
	} else if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Redis error fetching default amount: %v", err)
		return
	}

	// Fetch fallback summary
	fallbackRequests, err := redisClient.Get(ctx, fallbackProcessor+"_total_requests").Int64()
	if err == redis.Nil {
		fallbackRequests = 0
	} else if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Redis error fetching fallback requests: %v", err)
		return
	}

	fallbackAmount, err := redisClient.Get(ctx, fallbackProcessor+"_total_amount").Int64()
	if err == redis.Nil {
		fallbackAmount = 0
	} else if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Redis error fetching fallback amount: %v", err)
		return
	}

	summary := PaymentSummaryResponse{
		Default: Summary{
			TotalRequests: defaultRequests,
			TotalAmount:   defaultAmount,
		},
		Fallback: Summary{
			TotalRequests: fallbackRequests,
			TotalAmount:   fallbackAmount,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func startHealthChecks() {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkProcessorHealth(defaultProcessor, defaultProcessorURL)
		checkProcessorHealth(fallbackProcessor, fallbackProcessorURL)
	}
}

func checkProcessorHealth(processorName, url string) {
	resp, err := http.Get(url + "/payments/service-health")
	isHealthy := false
	if err == nil && resp.StatusCode == http.StatusOK {
		var healthResp ServiceHealthResponse
		if err := json.NewDecoder(resp.Body).Decode(&healthResp); err == nil {
			isHealthy = !healthResp.Failing
		}
		resp.Body.Close()
	}

	healthMutex.Lock()
	processorHealth[processorName] = isHealthy
	healthMutex.Unlock()

	log.Printf("Health check for %s (%s): %t", processorName, url, isHealthy)
}
