# Rinha de Backend 2025 - Go Edition

This is a backend solution for the Rinha de Backend 2025 challenge, implemented in Go with a focus on **reliability**, **consistency**, and **exploring performance optimization strategies**.

## 🚀 Architecture Overview

The solution employs a microservices-like architecture to handle payment processing efficiently and robustly, especially given the instability of external payment processors.

### Components

*   **Nginx Load Balancer:** Distributes incoming requests across multiple API instances.
*   **API Gateway Instances (5x):** Stateless Go applications responsible for quickly accepting payment requests and queuing them for asynchronous processing.
*   **Worker Service (1x):** A dedicated Go service that processes queued payment requests, interacts with external payment processors, and maintains a consistent payment summary.
*   **Redis:** Used as a persistent and fast data store for:
    *   Storing payment summary data (`totalRequests`, `totalAmount`) to ensure consistency across worker restarts and for the `GET /payments-summary` endpoint.
    *   Tracking processed payment `correlationId`s to ensure idempotency and prevent duplicate processing.
    *   Caching the health status of external payment processors to avoid hitting rate limits on health checks.

### Architecture Flow

```
[Client] → [Nginx] → [API Instance (Load Balanced)] → [Worker (Async)] → [External Payment Processors]
                                                               ↓
                                                             [Redis]
```

## 🎯 Endpoints

*   **`POST /payments`:** Accepts payment requests. Handled by API instances, which then asynchronously forward to the worker.
*   **`GET /payments-summary`:** Returns an aggregated summary of processed payments. Handled by the worker, which retrieves data from Redis.
*   **`POST /purge-payments`:** Resets all payment data in Redis. Handled by the worker.
*   **`GET /healthz`:** Health check endpoint for all services.

## ⚡ Quick Start

### 1. Build and Deploy

Ensure you have `podman-compose` installed.

```bash
# Navigate to your participant directory
cd participantes/arthur-r-oliveira

# Build and start all services
podman-compose up --build -d

# Your API will be available at http://localhost:9999
```

### 2. Test the Implementation

You can use the `k6` tool provided in the `rinha-test` directory of the main challenge repository.

```bash
# Navigate to the rinha-test directory
cd ../../rinha-test

# Run the k6 test (replace $MAX_REQUESTS, $participant, $directory with your values)
k6 run -e MAX_REQUESTS=$MAX_REQUESTS -e PARTICIPANT=$participant -e TOKEN=$(uuidgen) --log-output=file=$directory/k6.logs rinha.js
```

### 3. Basic API Interaction

```bash
# Test payment creation
curl -X POST http://localhost:9999/payments \
  -H "Content-Type: application/json" \
  -d '{"correlationId": "test-001", "amount": 100.50}'

# Check payment summary
curl http://localhost:9999/payments-summary

# Purge all payments
curl -X POST http://localhost:9999/purge-payments
```

## ⚙️ Configuration

Environment variables for services (set in `docker-compose.yml`):

### API Instances
*   `MODE=api` - Specifies the service role as API gateway.
*   `WORKER_HOST=worker` - Hostname of the worker service for internal communication.
*   `WORKER_PORT=8081` - Port of the worker service.
*   `PORT=8080` - Port the API instance listens on.
*   `GOMAXPROCS=1` - Limits the number of OS threads that can execute Go code simultaneously.
*   `REDIS_ADDR=redis:6379` - Address of the Redis service.

### Worker Instance
*   `MODE=worker` - Specifies the service role as worker.
*   `DEFAULT_PROCESSOR_URL=http://payment-processor-default:8080` - URL for the default external payment processor.
*   `FALLBACK_PROCESSOR_URL=http://payment-processor-fallback:8080` - URL for the fallback external payment processor.
*   `PORT=8081` - Port the worker service listens on.
*   `GOMAXPROCS=1` - Limits the number of OS threads that can execute Go code simultaneously.
*   `REDIS_ADDR=redis:6379` - Address of the Redis service.

## 📊 Resource Allocation

| Service | CPU | Memory | Purpose |
|---------|-----|--------|---------|
| Nginx | 0.1 | 40MB | Load balancing |
| API Instance (x5) | 0.2 each (1.0 total) | 40MB each (200MB total) | Payment acceptance |
| Worker | 0.3 | 80MB | Payment processing & summary |
| Redis | 0.1 | 30MB | Data persistence & caching |
| **Total** | **1.5** | **350MB** | **Within challenge limits** |

## 💡 Design Principles

### 1. Reliability First
- Utilizes standard Go HTTP for robust communication.
- Employs Redis for data persistence and idempotency, ensuring data integrity even with service restarts.

### 2. Separation of Concerns
- API instances are optimized for fast request acceptance.
- The Worker service is dedicated to the heavier task of payment processing and summary aggregation.
- Clear boundaries enhance maintainability and scalability.

### 3. Asynchronous Processing & Graceful Degradation
- Payment requests are queued and processed asynchronously by the worker, allowing the API to respond quickly.
- This design handles external processor instability gracefully, preventing backpressure on the API.

### 4. Performance Considerations
- Asynchronous processing aims to maximize API throughput.
- Redis provides fast read/write operations for summary data and processed IDs.
- Connection pooling and keepalives are configured for efficient HTTP communication.
- Further optimizations may be explored to reduce `payments_inconsistency`.

## 🔍 Troubleshooting

### General Issues
1.  Check service logs: `podman-compose logs <service_name>` (e.g., `podman-compose logs worker`)
2.  Verify container status: `podman-compose ps`

### If Payments Summary is Incorrect
1.  Ensure Redis is running and healthy: `podman-compose logs redis`
2.  Check worker logs for Redis connection errors or processing failures.

### If Performance Is Poor
1.  Monitor resource usage: `podman stats`
2.  Check service logs for any errors or bottlenecks.
3.  Verify external payment processor response times.

## 🛠️ Development

### Local Development

To run the API and Worker locally without `podman-compose`:

```bash
cd api

# Run as API gateway (in one terminal)
MODE=api PORT=8080 WORKER_HOST=localhost WORKER_PORT=8081 REDIS_ADDR=localhost:6379 go run main.go

# Run as worker (in another terminal)
MODE=worker PORT=8081 DEFAULT_PROCESSOR_URL=http://localhost:8001 FALLBACK_PROCESSOR_URL=http://localhost:8002 REDIS_ADDR=localhost:6379 go run main.go
```

## 📈 Latest Test Results

Here are the results from the most recent `k6` test run:

```
     data_received..................: 2.0 MB    33 kB/s
     data_sent......................: 3.4 MB    55 kB/s
     default_total_amount...........: 201368.1  3296.698092/s
     default_total_fee..............: 10068.405 164.834905/s
     default_total_requests.........: 10119     165.663221/s
     fallback_total_amount..........: 46864.5   767.242218/s
     fallback_total_fee.............: 7029.675  115.086333/s
     fallback_total_requests........: 2355      38.554885/s
     http_req_blocked...............: p(99)=263.55µs count=16803
     http_req_connecting............: p(99)=202.33µs count=16803
     http_req_duration..............: p(99)=1.19ms   count=16803
       { expected_response:true }...: p(99)=1.19ms   count=16803
     http_req_failed................: 0.00%     ✓ 0           ✗ 16803
     http_req_receiving.............: p(99)=76.72µs  count=16803
     http_req_sending...............: p(99)=60.01µs  count=16803
     http_req_tls_handshaking.......: p(99)=0s       count=16803
     http_req_waiting...............: p(99)=1.09ms   count=16803
     http_reqs......................: 16803     275.090335/s
     iteration_duration.............: p(99)=1s       count=16765
     iterations.....................: 16765     274.468218/s
     payments_inconsistency.........: 18343     300.302447/s
     total_transactions_amount......: 248232.6  4063.94031/s
     transactions_failure...........: 0         0/s
     transactions_success...........: 16753     274.27176/s
     vus............................: 87        min=9         max=549

running (1m01.1s), 000/554 VUs, 16765 complete and 0 interrupted iterations
payments             ✓ [======================================] 000/550 VUs  1m0s
payments_consistency ✓ [======================================] 1 VUs        1m0s
stage_00             ✓ [======================================] 1 VUs        1s
stage_01             ✓ [======================================] 1 VUs        1s
stage_02             ✓ [======================================] 1 VUs        1s
stage_03             ✓ [======================================] 1 VUs        1s
stage_04             ✓ [======================================] 1 VUs        1s
stage_05             ✓ [======================================] 1 VUs        1s
```

```
curl http://localhost:9999/payments-summary
{"default":{"totalRequests":10119,"totalAmount":201368.1},"fallback":{"totalRequests":2355,"totalAmount":46864.5}}
```

## ⚖️ License

This project is open-source and available under the MIT License.
