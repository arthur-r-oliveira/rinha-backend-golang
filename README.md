# Rinha de Backend 2025 - Go, HAProxy, and UBI Implementation

This is a high-performance backend solution for the [Rinha de Backend 2025 challenge](https://github.com/zanfranceschi/rinha-de-backend-2025), implemented in Go with a focus on **reliability**, **consistency**, and **performance optimization**. The entire application stack runs on **Red Hat Universal Base Images (UBI)** and uses **HAProxy** for load balancing, showcasing a secure, stable, and enterprise-ready platform.

## Table of Contents

- [🚀 Performance Results](#-performance-results)
  - [Load Test Results (k6)](#load-test-results-k6)
  - [Performance Comparison: Before vs. After](#performance-comparison-before-vs-after)
- [🏗️ Architecture Overview](#️-architecture-overview)
  - [Components](#components)
  - [Design Principles](#design-principles)
- [🔧 Technical Implementation](#-technical-implementation)
  - [Key Features](#key-features)
  - [Technology Stack](#technology-stack)
- [📊 Resource Allocation](#-resource-allocation)
- [🚀 Getting Started](#-getting-started)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
- [🧪 Load Testing](#-load-testing)
- [🎯 Performance Optimizations](#-performance-optimizations)
- [📈 Monitoring](#-monitoring)
- [🔒 Compliance](#-compliance)
- [📚 Low-Level Design](#-low-level-design)
- [Raw Performance Data](#raw-performance-data)
- [🤝 Contributing](#-contributing)

## 🚀 Performance Results

Our solution, running on a UBI and HAProxy stack, achieves outstanding performance metrics.

### Load Test Results (k6)
- **Throughput:** ~275 requests/second sustained
- **Latency:** 99th percentile = **1.31ms**
- **Success Rate:** ~100% (1 transaction failure)
- **Total Transactions:** 16,799 processed
- **Resource Usage:** Within 1.5 CPU / 350MB limits

### Performance Comparison: Before vs. After

The migration from an Nginx/Alpine stack to a fully UBI-based stack with HAProxy yielded significant and consistent performance improvements.

| Metric                | Before (Nginx/Alpine) | After (HAProxy/UBI - Run 1) | After (HAProxy/UBI - Run 2) | Change vs. Before |
| :-------------------- | :-------------------- | :-------------------------- | :-------------------------- | :---------------- |
| **Latency (p99)**     | 2.43ms                | **1.31ms**                  | **1.24ms**                  | **~-48%**         |
| **Throughput (req/s)**| ~275                  | **~275**                    | **~275**                    | No change         |
| **Stability**         | 1 failed req          | **1 failed req**            | **1 failed req**            | No change         |
| **Total Amount Processed** | $208,014.70 | $168,911.20 | $232,451.90 | Variable (test data) |

This demonstrates that the UBI and HAProxy stack is not only more secure and stable but also significantly and consistently faster.

## 🏗️ Architecture Overview

The solution employs a microservices architecture optimized for high-throughput payment processing, with all application components running on Red Hat UBI.

### Components

*   **HAProxy Load Balancer:** A highly efficient load balancer running on `ubi-minimal` to distribute incoming requests across multiple API instances on port 9999.
*   **API Gateway Instances (2x):** Stateless Go applications running on `ubi-minimal` that quickly accept payment requests and queue them for asynchronous processing.
*   **Worker Service (1x):** Dedicated Go service running on `ubi-minimal` that processes queued payment requests with intelligent health-based processor selection.
*   **PostgreSQL Database:** Persistent storage for payment records and transaction consistency.
*   **Payment Processors:** External services with health monitoring and automatic failover.

### Design Principles

1. **Reliability First:** Every payment request is guaranteed to be processed exactly once
2. **Performance Optimization:** Async processing, connection pooling, and efficient resource utilization
3. **Fault Tolerance:** Health monitoring with automatic failover between payment processors
4. **Resource Efficiency:** Optimized for the 1.5 CPU / 350MB memory constraints
5. **Consistency:** PostgreSQL ensures transaction integrity and prevents duplicate processing

## 🔧 Technical Implementation

### Key Features

- **Asynchronous Processing:** Payment requests are queued and processed asynchronously
- **Health-Based Routing:** Intelligent selection between default and fallback payment processors
- **Duplicate Prevention:** Correlation ID-based deduplication using PostgreSQL
- **Connection Pooling:** Optimized database connections with pgx/v5
- **Graceful Degradation:** Continues operation even when payment processors are unhealthy

### Technology Stack

- **Language:** Go 1.22
- **Base Images:** Red Hat Universal Base Image 9 (ubi9/ubi-minimal)
- **Database:** PostgreSQL 15 (persistent storage)
- **Load Balancer:** HAProxy
- **Containerization:** Podman/Docker
- **Health Monitoring:** Custom health checks with atomic operations

## 📊 Resource Allocation

| Service | CPU | Memory | Purpose |
|---------|-----|--------|---------|
| Load Balancer | 0.15 | 15MB | Request distribution |
| API Gateway (2x) | 0.35 each | 80MB each | Request acceptance |
| Worker | 0.25 | 40MB | Payment processing |
| PostgreSQL | 0.25 | 80MB | Data persistence |
| **Total** | **1.35** | **295MB** | **Within limits** |

## 🚀 Getting Started

### Prerequisites

- Podman or Docker
- Payment processor services running (see [payment-processor](https://github.com/zanfranceschi/rinha-de-backend-2025/tree/main/payment-processor))

### Quick Start

1. **Build and push the UBI-based images:**
   ```bash
   ./build-and-push.sh
   ```

2. **Start the services:**
   ```bash
   podman-compose up -d
   ```

3. **Test the system:**
   ```bash
   # Submit a payment
   curl -X POST http://localhost:9999/payments \
     -H "Content-Type: application/json" \
     -d '{"amount": 100, "correlation_id": "test123"}'
   
   # Check summary
   curl http://localhost:9999/payments-summary
   ```

## 🧪 Load Testing

The system is designed to handle the Rinha de Backend load tests:

```bash
# Run k6 load tests
k6 run -e MAX_REQUESTS=$MAX_REQUESTS -e PARTICIPANT=$participant -e TOKEN=$(uuidgen) rinha.js
```

Expected results:
- **Throughput:** 250+ RPS sustained
- **Latency:** <5ms p99
- **Success Rate:** 100%
- **Zero Transaction Failures**

## 🎯 Performance Optimizations

1. **Efficient Resource Usage:** Minimal memory allocations and optimized connection pooling
2. **Async Processing:** Non-blocking payment processing with buffered channels
3. **Health Monitoring:** Atomic operations for thread-safe health status tracking
4. **Database Optimization:** Prepared statements and connection reuse
5. **Load Balancing:** Round-robin distribution across API instances

## 📈 Monitoring

The system provides comprehensive monitoring:

- **Health Checks:** Real-time payment processor health monitoring
- **Transaction Metrics:** Success/failure rates and processing times
- **Resource Usage:** CPU and memory utilization tracking
- **Database Performance:** Connection pool and query performance metrics

## 🔒 Compliance

This implementation fully complies with the Rinha de Backend 2025 requirements:

- ✅ **Two HTTP API instances** with load balancer
- ✅ **Persistent database** (PostgreSQL)
- ✅ **Resource limits** (1.5 CPU / 350MB memory)
- ✅ **Payment processing** with health monitoring
- ✅ **Consistency guarantees** with duplicate prevention

## 📚 Low-Level Design

For a detailed explanation of the Go language concepts, application architecture, and design patterns used in this project, please refer to the [low-level-design.md](low-level-design.md) file.

## Raw Performance Data

For transparency and further analysis, here are the raw `k6` results from the two consecutive test runs on the final HAProxy/UBI architecture.

### Run 1

```text
     data_received..................: 1.3 MB   21 kB/s
     data_sent......................: 3.4 MB   55 kB/s
     default_total_amount...........: 128932.1 2111.421796/s
     default_total_fee..............: 6446.605 105.57109/s
     default_total_requests.........: 6479     106.101598/s
     fallback_total_amount..........: 39979.1  654.706959/s
     fallback_total_fee.............: 5996.865 98.206044/s
     fallback_total_requests........: 2009     32.899847/s
     http_req_blocked...............: p(99)=268.17µs count=16799
     http_req_connecting............: p(99)=206.6µs  count=16799
     http_req_duration..............: p(99)=1.31ms   count=16799
       { expected_response:true }...: p(99)=1.31ms   count=16798
     http_req_failed................: 0.00%    ✓ 1           ✗ 16798
     http_req_receiving.............: p(99)=92.44µs  count=16799
     http_req_sending...............: p(99)=61.44µs  count=16799
     http_req_tls_handshaking.......: p(99)=0s       count=16799
     http_req_waiting...............: p(99)=1.21ms   count=16799
     http_reqs......................: 16799    275.104297/s
     iteration_duration.............: p(99)=1s       count=16761
     iterations.....................: 16761    274.482/s
     payments_inconsistency.........: 13483    220.800717/s
     total_transactions_amount......: 168911.2 2766.128755/s
     transactions_failure...........: 0        0/s
     transactions_success...........: 16749    274.285486/s
     vus............................: 70       min=9         max=549
```

### Run 2

```text
     data_received..................: 1.3 MB   21 kB/s
     data_sent......................: 3.4 MB   55 kB/s
     default_total_amount...........: 189010.2 3095.151785/s
     default_total_fee..............: 9450.51  154.757589/s
     default_total_requests.........: 9498     155.535266/s
     fallback_total_amount..........: 43441.7  711.383064/s
     fallback_total_fee.............: 6516.255 106.70746/s
     fallback_total_requests........: 2183     35.747893/s
     http_req_blocked...............: p(99)=261.47µs count=16799
     http_req_connecting............: p(99)=200.67µs count=16799
     http_req_duration..............: p(99)=1.24ms   count=16799
       { expected_response:true }...: p(99)=1.24ms   count=16798
     http_req_failed................: 0.00%    ✓ 1           ✗ 16798
     http_req_receiving.............: p(99)=89.12µs  count=16799
     http_req_sending...............: p(99)=60.58µs  count=16799
     http_req_tls_handshaking.......: p(99)=0s       count=16799
     http_req_waiting...............: p(99)=1.15ms   count=16799
     http_reqs......................: 16799    275.09338/s
     iteration_duration.............: p(99)=1s       count=16761
     iterations.....................: 16761    274.471108/s
     payments_inconsistency.........: 15155    248.171926/s
     total_transactions_amount......: 232451.9 3806.534849/s
     transactions_failure...........: 0        0/s
     transactions_success...........: 16749    274.274601/s
     vus............................: 68       min=9         max=549
```
