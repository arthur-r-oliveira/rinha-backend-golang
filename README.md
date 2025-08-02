# Rinha de Backend 2025 - Go, HAProxy, and UBI Implementation

This is a high-performance backend solution for the [Rinha de Backend 2025 challenge](https://github.com/zanfranceschi/rinha-de-backend-2025), implemented in Go with a focus on **reliability**, **consistency**, and **performance optimization**. The entire application stack runs on **Red Hat Universal Base Images (UBI)** and uses **HAProxy** for load balancing, showcasing a secure, stable, and enterprise-ready platform.

## Table of Contents

- [🚀 Performance Results](#-performance-results)
  - [Load Test Results (k6)](#load-test-results-k6)
  - [Transaction Distribution](#transaction-distribution)
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

The migration from an Nginx/Alpine stack to a fully UBI-based stack with HAProxy yielded significant performance improvements, particularly in latency.

| Metric                | Before (Nginx/Alpine) | After (HAProxy/UBI) | Change     |
| :-------------------- | :-------------------- | :------------------ | :--------- |
| **Latency (p99)**     | 2.43ms                | **1.31ms**          | **-46%**   |
| **Throughput (req/s)**| ~275                  | **~275**            | No change  |
| **Stability**         | 1 failed req          | **1 failed req**    | No change  |

This demonstrates that the UBI and HAProxy stack is not only more secure and stable but also significantly faster.

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
