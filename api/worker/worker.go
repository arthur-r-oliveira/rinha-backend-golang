package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"

	"rinha-backend-golang/config"
	"rinha-backend-golang/models"
)

// Worker processes payment requests and interacts with external processors.
type Worker struct {
	httpClient *http.Client
}

// NewWorker creates a new Worker instance.
func NewWorker() *Worker {
	return &Worker{
		httpClient: &http.Client{
			Timeout: config.PaymentTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// Start initializes the Worker and starts listening for requests.
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
	var req models.PaymentRequest
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

func (w *Worker) processPayment(req models.PaymentRequest) {
	ctx := context.Background()

	// Check if already processed
	if processed, err := config.RedisClient.Exists(ctx, req.CorrelationID).Result(); err != nil {
		log.Printf("Worker: Error checking if correlation ID %s exists in Redis: %v", req.CorrelationID, err)
		return
	} else if processed == 1 {
		log.Printf("Worker: Correlation ID %s already processed, skipping.", req.CorrelationID)
		return
	}

	// Get processor health from Redis
	isDefaultHealthy, err := config.RedisClient.Get(ctx, "health:default").Bool()
	if err != nil && err != redis.Nil {
		log.Printf("Worker: Error getting default processor health from Redis: %v", err)
		isDefaultHealthy = true // Assume healthy if Redis read fails
	}
	isFallbackHealthy, err := config.RedisClient.Get(ctx, "health:fallback").Bool()
	if err != nil && err != redis.Nil {
		log.Printf("Worker: Error getting fallback processor health from Redis: %v", err)
		isFallbackHealthy = true // Assume healthy if Redis read fails
	}

	log.Printf("Worker: Health status - Default: %t, Fallback: %t", isDefaultHealthy, isFallbackHealthy)

	if isDefaultHealthy {
		log.Printf("Worker: Attempting to call default processor for payment %s", req.CorrelationID)
		if w.callProcessor(config.DefaultProcessorURL, req) {
			req.Processor = "default"
			if err := config.RedisClient.Incr(ctx, "summary:default:requests").Err(); err != nil {
				log.Printf("Worker: Error incrementing default requests in Redis: %v", err)
			}
			if err := config.RedisClient.IncrByFloat(ctx, "summary:default:amount", req.Amount).Err(); err != nil {
				log.Printf("Worker: Error incrementing default amount in Redis: %v", err)
			}
			if err := config.RedisClient.Set(ctx, req.CorrelationID, "processed", 0).Err(); err != nil { // Store correlation ID to prevent duplicates
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
		if w.callProcessor(config.FallbackProcessorURL, req) {
			req.Processor = "fallback"
			if err := config.RedisClient.Incr(ctx, "summary:fallback:requests").Err(); err != nil {
				log.Printf("Worker: Error incrementing fallback requests in Redis: %v", err)
			}
			if err := config.RedisClient.IncrByFloat(ctx, "summary:fallback:amount", req.Amount).Err(); err != nil {
				log.Printf("Worker: Error incrementing fallback amount in Redis: %v", err)
			}
			if err := config.RedisClient.Set(ctx, req.CorrelationID, "processed", 0).Err(); err != nil { // Store correlation ID to prevent duplicates
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

func (w *Worker) callProcessor(url string, req models.PaymentRequest) bool {
	ctx, cancel := context.WithTimeout(context.Background(), config.PaymentTimeout)
	defer cancel()

	var err error // Declare err once

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
	if err = json.NewDecoder(resp.Body).Decode(&processorResp); err != nil {
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

	defaultRequests, _ := config.RedisClient.Get(ctx, "summary:default:requests").Int64()
	defaultAmount, _ := config.RedisClient.Get(ctx, "summary:default:amount").Float64()
	fallbackRequests, _ := config.RedisClient.Get(ctx, "summary:fallback:requests").Int64()
	fallbackAmount, _ := config.RedisClient.Get(ctx, "summary:fallback:amount").Float64()

	defaultSummary := models.Summary{
		TotalRequests: defaultRequests,
		TotalAmount:   defaultAmount,
	}
	fallbackSummary := models.Summary{
		TotalRequests: fallbackRequests,
		TotalAmount:   fallbackAmount,
	}

	summary := models.PaymentSummaryResponse{
		Default:  defaultSummary,
		Fallback: fallbackSummary,
	}

	wr.Header().Set("Content-Type", "application/json")
	json.NewEncoder(wr).Encode(summary)
}

func (w *Worker) handlePurgePayments(wr http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	config.RedisClient.Del(ctx, "summary:default:requests", "summary:default:amount", "summary:fallback:requests", "summary:fallback:amount")
	// Optionally, clear all processed IDs if needed, but be careful with large datasets
	// For now, we assume correlation IDs are unique per test run and don't need explicit purging

	wr.WriteHeader(http.StatusOK)
}

func (w *Worker) startHealthChecks() {
	ticker := time.NewTicker(config.HealthCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		w.checkProcessorHealth("default", config.DefaultProcessorURL)
		w.checkProcessorHealth("fallback", config.FallbackProcessorURL)
	}
}

func (w *Worker) checkProcessorHealth(name, url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	log.Printf("Worker: Checking health for %s at %s/payments/service-health", name, url)
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/payments/service-health", nil)
	if err != nil {
		log.Printf("Worker: Error creating health check request for %s: %v", name, err)
		config.RedisClient.Set(ctx, "health:"+name, false, 0)
		return
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		log.Printf("Worker: Error calling health check for %s: %v", name, err)
		config.RedisClient.Set(ctx, "health:"+name, false, 0)
		return
	}
	defer resp.Body.Close()

	log.Printf("Worker: Health check for %s returned status: %d", name, resp.StatusCode)

	if resp.StatusCode == 200 {
		var healthResp models.ServiceHealthResponse
		if err := json.NewDecoder(resp.Body).Decode(&healthResp); err == nil {
			log.Printf("Worker: Health check for %s - Failing: %t", name, healthResp.Failing)
			config.RedisClient.Set(ctx, "health:"+name, !healthResp.Failing, 0)
		} else {
			log.Printf("Worker: Error decoding health check response for %s: %v", name, err)
			config.RedisClient.Set(ctx, "health:"+name, false, 0)
		}
	} else {
		log.Printf("Worker: Health check for %s failed with non-200 status: %d", name, resp.StatusCode)
		config.RedisClient.Set(ctx, "health:"+name, false, 0)
	}
}
