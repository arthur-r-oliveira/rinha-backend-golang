package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

// Configuration constants
const (
	healthCheckInterval = 5 * time.Second
	paymentTimeout      = 3 * time.Second
	queueSize           = 10000
	numWorkers          = 100
	ringBufferSize      = 50000
)

// Structs
type PaymentRequest struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	Timestamp     time.Time `json:"timestamp,omitempty"`
	Processor     string    `json:"processor,omitempty"`
}

type PaymentSummaryResponse struct {
	Default  Summary `json:"default"`
	Fallback Summary `json:"fallback"`
}

type Summary struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type ServiceHealthResponse struct {
	Failing bool `json:"failing"`
}

// --- Global Variables ---
var (
	paymentQueue          chan PaymentRequest
	processedIDs          []PaymentRequest // Store full request for filtering
	processedIDsIdx       atomic.Int64
	processedIDsMap       sync.Map
	defaultTotalRequests  atomic.Int64
	defaultTotalAmount    float64 // Changed to float64
	fallbackTotalRequests atomic.Int64
	fallbackTotalAmount   float64 // Changed to float64
	processorHealth       map[string]bool
	healthMutex           sync.RWMutex
	httpClient            *http.Client
	defaultProcessorURL   string
	fallbackProcessorURL  string
	workerURL             string // Declared globally

	// Mutexes for float64 atomic operations
	defaultAmountMutex sync.Mutex
	fallbackAmountMutex sync.Mutex

	redisClient *redis.Client
)

// --- API Gateway ---
type APIGateway struct {
	paymentQueue chan PaymentRequest
	httpClient   *http.Client
}

func NewAPIGateway() *APIGateway {
	return &APIGateway{
		paymentQueue: make(chan PaymentRequest, queueSize),
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

func (api *APIGateway) Start() {
	for i := 0; i < numWorkers; i++ {
		go api.paymentForwarder()
	}
	http.HandleFunc("/payments", api.handlePayments)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("API Gateway starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (api *APIGateway) handlePayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	select {
	case api.paymentQueue <- req:
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}
}

func (api *APIGateway) paymentForwarder() {
	for req := range api.paymentQueue {
		api.forwardPayment(req)
	}
}

func (api *APIGateway) forwardPayment(req PaymentRequest) {
	reqBody, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", workerURL+"/process-payment", bytes.NewReader(reqBody))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := api.httpClient.Do(httpReq)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// --- Worker ---
type Worker struct {
	httpClient *http.Client
}

func NewWorker() *Worker {
	return &Worker{
		httpClient: &http.Client{
			Timeout: paymentTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

func (w *Worker) Start() {
	go w.startHealthChecks()
	http.HandleFunc("/process-payment", w.handleProcessPayment)
	http.HandleFunc("/payments-summary", w.handlePaymentsSummary)
	http.HandleFunc("/purge-payments", w.handlePurgePayments)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("Worker starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (w *Worker) handleProcessPayment(wr http.ResponseWriter, r *http.Request) {
	log.Println("Worker received process-payment request")
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Worker: Invalid request body: %v", err)
		http.Error(wr, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Timestamp = time.Now()
	log.Printf("Worker processing payment: %s, Amount: %.2f", req.CorrelationID, req.Amount)
	go w.processPayment(req)
	wr.WriteHeader(http.StatusOK)
}

func (w *Worker) processPayment(req PaymentRequest) {
	ctx := context.Background()

	// Check if already processed
	if processed, err := redisClient.Exists(ctx, req.CorrelationID).Result(); err != nil {
		log.Printf("Worker: Error checking if correlation ID %s exists in Redis: %v", req.CorrelationID, err)
		return
	} else if processed == 1 {
		log.Printf("Worker: Correlation ID %s already processed, skipping.", req.CorrelationID)
		return
	}

	// Get processor health from Redis
	isDefaultHealthy, err := redisClient.Get(ctx, "health:default").Bool()
	if err != nil && err != redis.Nil {
		log.Printf("Worker: Error getting default processor health from Redis: %v", err)
		isDefaultHealthy = true // Assume healthy if Redis read fails
	}
	isFallbackHealthy, err := redisClient.Get(ctx, "health:fallback").Bool()
	if err != nil && err != redis.Nil {
		log.Printf("Worker: Error getting fallback processor health from Redis: %v", err)
		isFallbackHealthy = true // Assume healthy if Redis read fails
	}

	log.Printf("Worker: Health status - Default: %t, Fallback: %t", isDefaultHealthy, isFallbackHealthy)

	if isDefaultHealthy {
		log.Printf("Worker: Attempting to call default processor for payment %s", req.CorrelationID)
		if w.callProcessor(defaultProcessorURL, req) {
			req.Processor = "default"
			if err := redisClient.Incr(ctx, "summary:default:requests").Err(); err != nil {
				log.Printf("Worker: Error incrementing default requests in Redis: %v", err)
			}
			if err := redisClient.IncrByFloat(ctx, "summary:default:amount", req.Amount).Err(); err != nil {
				log.Printf("Worker: Error incrementing default amount in Redis: %v", err)
			}
			if err := redisClient.Set(ctx, req.CorrelationID, "processed", 0).Err(); err != nil { // Store correlation ID to prevent duplicates
				log.Printf("Worker: Error setting processed ID %s in Redis: %v", req.CorrelationID, err)
			}
			log.Printf("Worker: Successfully processed payment %s with default processor and updated Redis.", req.CorrelationID)
			return
		} else {
			log.Printf("Worker: Failed to process payment %s with default processor.", req.CorrelationID)
		}
	}

	if isFallbackHealthy {
		log.Printf("Worker: Attempting to call fallback processor for payment %s", req.CorrelationID)
		if w.callProcessor(fallbackProcessorURL, req) {
			req.Processor = "fallback"
			if err := redisClient.Incr(ctx, "summary:fallback:requests").Err(); err != nil {
				log.Printf("Worker: Error incrementing fallback requests in Redis: %v", err)
			}
			if err := redisClient.IncrByFloat(ctx, "summary:fallback:amount", req.Amount).Err(); err != nil {
				log.Printf("Worker: Error incrementing fallback amount in Redis: %v", err)
			}
			if err := redisClient.Set(ctx, req.CorrelationID, "processed", 0).Err(); err != nil { // Store correlation ID to prevent duplicates
				log.Printf("Worker: Error setting processed ID %s in Redis: %v", req.CorrelationID, err)
			}
			log.Printf("Worker: Successfully processed payment %s with fallback processor and updated Redis.", req.CorrelationID)
			return
		} else {
			log.Printf("Worker: Failed to process payment %s with fallback processor.", req.CorrelationID)
		}
	}

	log.Printf("Worker: No healthy processor found or payment %s could not be processed.", req.CorrelationID)
}

func (w *Worker) callProcessor(url string, req PaymentRequest) bool {
	ctx, cancel := context.WithTimeout(context.Background(), paymentTimeout)
	defer cancel()
	reqBody, err := json.Marshal(req)
	if err != nil {
		log.Printf("Worker: Error marshalling payment request %s for processor %s: %v", req.CorrelationID, url, err)
		return false
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url+"/payments", bytes.NewReader(reqBody))
	if err != nil {
		log.Printf("Worker: Error creating request for payment %s to processor %s: %v", req.CorrelationID, url, err)
		return false
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("Worker: Error calling processor %s for payment %s: %v", url, req.CorrelationID, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Worker: Processor %s returned non-OK status %d for payment %s", url, resp.StatusCode, req.CorrelationID)
		return false
	}

	// Decode response body to check for success message
	var processorResp struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&processorResp); err != nil {
		log.Printf("Worker: Error decoding processor response for %s: %v", url, err)
		return false
	}

	if processorResp.Message != "payment processed successfully" {
		log.Printf("Worker: Processor %s returned unexpected message '%s' for payment %s", url, processorResp.Message, req.CorrelationID)
		return false
	}

	log.Printf("Worker: Successfully processed payment %s with processor %s", req.CorrelationID, url)
	return true
}

func (w *Worker) handlePaymentsSummary(wr http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	defaultRequests, _ := redisClient.Get(ctx, "summary:default:requests").Int64()
	defaultAmount, _ := redisClient.Get(ctx, "summary:default:amount").Float64()
	fallbackRequests, _ := redisClient.Get(ctx, "summary:fallback:requests").Int64()
	fallbackAmount, _ := redisClient.Get(ctx, "summary:fallback:amount").Float64()

	defaultSummary := Summary{
		TotalRequests: defaultRequests,
		TotalAmount:   defaultAmount,
	}
	fallbackSummary := Summary{
		TotalRequests: fallbackRequests,
		TotalAmount:   fallbackAmount,
	}

	summary := PaymentSummaryResponse{
		Default:  defaultSummary,
		Fallback: fallbackSummary,
	}

	wr.Header().Set("Content-Type", "application/json")
	json.NewEncoder(wr).Encode(summary)
}

func (w *Worker) handlePurgePayments(wr http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	redisClient.Del(ctx, "summary:default:requests", "summary:default:amount", "summary:fallback:requests", "summary:fallback:amount")
	// Optionally, clear all processed IDs if needed, but be careful with large datasets
	// For now, we assume correlation IDs are unique per test run and don't need explicit purging

	wr.WriteHeader(http.StatusOK)
}

func (w *Worker) startHealthChecks() {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		w.checkProcessorHealth("default", defaultProcessorURL)
		w.checkProcessorHealth("fallback", fallbackProcessorURL)
	}
}

func (w *Worker) checkProcessorHealth(name, url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	log.Printf("Worker: Checking health for %s at %s/payments/service-health", name, url)
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/payments/service-health", nil)
	if err != nil {
		log.Printf("Worker: Error creating health check request for %s: %v", name, err)
		redisClient.Set(ctx, "health:"+name, false, 0)
		return
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		log.Printf("Worker: Error calling health check for %s: %v", name, err)
		redisClient.Set(ctx, "health:"+name, false, 0)
		return
	}
	defer resp.Body.Close()

	log.Printf("Worker: Health check for %s returned status: %d", name, resp.StatusCode)

	if resp.StatusCode == 200 {
		var healthResp ServiceHealthResponse
		if err := json.NewDecoder(resp.Body).Decode(&healthResp); err == nil {
			log.Printf("Worker: Health check for %s - Failing: %t", name, healthResp.Failing)
			redisClient.Set(ctx, "health:"+name, !healthResp.Failing, 0)
		} else {
			log.Printf("Worker: Error decoding health check response for %s: %v", name, err)
			redisClient.Set(ctx, "health:"+name, false, 0)
		}
	} else {
		log.Printf("Worker: Health check for %s failed with non-200 status: %d", name, resp.StatusCode)
		redisClient.Set(ctx, "health:"+name, false, 0)
	}
}

// --- Main & Initialization ---
func init() {
	defaultProcessorURL = os.Getenv("DEFAULT_PROCESSOR_URL")
	fallbackProcessorURL = os.Getenv("FALLBACK_PROCESSOR_URL")
	workerHost := os.Getenv("WORKER_HOST")
	if workerHost == "" {
		workerHost = "worker"
	}
	workerPort := os.Getenv("WORKER_PORT")
	if workerPort == "" {
		workerPort = "8081"
	}
	workerURL = fmt.Sprintf("http://%s:%s", workerHost, workerPort)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: "", // no password set
		DB: 0,  // use default DB
	})

	// Ping the Redis server to ensure connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully!")
}

func main() {
	mode := os.Getenv("MODE")
	if mode == "worker" {
		worker := NewWorker()
		worker.Start()
	} else {
		api := NewAPIGateway()
		api.Start()
	}
}