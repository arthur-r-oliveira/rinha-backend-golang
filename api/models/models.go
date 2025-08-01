package models

import "time"

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