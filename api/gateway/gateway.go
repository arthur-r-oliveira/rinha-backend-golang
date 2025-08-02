package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"rinha-backend-golang/config"
	"rinha-backend-golang/models"
)

// APIGateway handles incoming payment requests and forwards them to the worker.
type APIGateway struct {
	paymentQueue chan models.PaymentRequest
	httpClient   *http.Client
	logger       *PaymentLogger
}

// NewAPIGateway creates a new APIGateway instance.
func NewAPIGateway() *APIGateway {
	return &APIGateway{
		paymentQueue: make(chan models.PaymentRequest, config.QueueSize),
		httpClient: &http.Client{
			Timeout: config.PaymentTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     60 * time.Second,
			},
		},
		logger: NewPaymentLogger(),
	}
}

// Start initializes the API Gateway and starts listening for requests.
func (api *APIGateway) Start() {
	for i := 0; i < config.NumWorkers; i++ {
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
	var req models.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	select {
	case api.paymentQueue <- req:
		// Persist asynchronously
		api.logger.LogPayment(req)
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

func (api *APIGateway) forwardPayment(req models.PaymentRequest) {
	reqBody, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", config.WorkerURL+"/process-payment", bytes.NewReader(reqBody))
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
