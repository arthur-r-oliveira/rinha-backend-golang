# Rinha de Backend 2025 - Go Edition

This is a backend solution for the Rinha de Backend 2025 challenge, implemented in Go with a focus on **reliability**, **consistency**, and **exploring performance optimization strategies**.

## 🚀 Architecture Overview

The solution employs a microservices-like architecture to handle payment processing efficiently and robustly, especially given the instability of external payment processors.

### Components

*   **Nginx Load Balancer:** Distributes incoming requests across multiple API instances.
*   **API Gateway Instances (5x):** Stateless Go applications responsible for quickly accepting payment requests and queuing them for asynchronous processing.
*   **Worker Service (1x)::** A dedicated Go service that processes queued payment requests, interacts with external payment processors, and maintains a consistent payment summary.
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

## Architectural Rationale: Single Worker Strategy

While a single worker might seem unconventional for a high-concurrent distributed system, it's a deliberate choice for this challenge due to specific constraints and goals:

*   **Simplified Consistency:** The challenge heavily penalizes inconsistencies in payment summaries. A single worker centralizes all payment processing and summary updates, significantly simplifying consistency management by avoiding distributed transaction complexities.
*   **Resource Efficiency:** Given tight CPU and memory limits, a single, highly optimized Go worker can efficiently utilize allocated resources for I/O-bound tasks (HTTP calls to external processors and Redis). Go's concurrency model excels at managing numerous concurrent I/O operations within a single process.
*   **External Bottleneck:** The external payment processors are inherently unstable and slow. The single worker can effectively saturate their capacity, meaning adding more workers might not yield significant throughput gains. The API Gateway acts as a buffer, preventing backpressure from affecting API responsiveness.
*   **Reduced Complexity:** A single worker simplifies deployment, monitoring, and debugging, reducing operational overhead in a time-constrained challenge.

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

# Run the k6 test (replace $MAX_REQUESTS, $participant, $TOKEN, $directory with your values)
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
     default_total_amount...........: 209706.2 3432.974433/s
     default_total_fee..............: 10485.31 171.648722/s
     default_total_requests.........: 10538    172.511278/s
     fallback_total_amount..........: 46864.5  767.190623/s
     fallback_total_fee.............: 7029.675 115.078594/s
     fallback_total_requests........: 2355     38.552293/s
     http_req_blocked...............: p(99)=270.35µs count=16801
     http_req_connecting............: p(99)=205.81µs count=16801
     http_req_duration..............: p(99)=1.18ms   count=16801
       { expected_response:true }...: p(99)=1.18ms   count=16801
     http_req_failed................: 0.00%    ✓ 0           ✗ 16801
     http_req_receiving.............: p(99)=70.72µs  count=16801
     http_req_sending...............: p(99)=56.18µs  count=16801
     http_req_tls_handshaking.......: p(99)=0s       count=16801
     http_req_waiting...............: p(99)=1.09ms   count=16801
     http_reqs......................: 16801    275.039095/s
     iteration_duration.............: p(99)=1s       count=16763
     iterations.....................: 16763    274.41702/s
     payments_inconsistency.........: 18763    307.15782/s
     total_transactions_amount......: 256570.7 4200.165056/s
     transactions_failure...........: 0        0/s
     transactions_success...........: 16751    274.220575/s
     vus............................: 84       min=9         max=549

running (1m01.1s), 000/554 VUs, 16763 complete and 0 interrupted iterations
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
{"default":{"totalRequests":10538,"totalAmount":209706.2},"fallback":{"totalRequests":2355,"totalAmount":46864.5}}
```

## 🚧 Backlog and Future Improvements

While the current architecture provides a solid foundation for reliability and consistency, the `payments_inconsistency` metric indicates areas for further optimization. Here are some potential improvements to explore:

*   **Reduce `payments_inconsistency`:**
    *   **Retry Mechanism:** Implement a more robust retry mechanism for failed calls to external payment processors within the worker. This could involve exponential backoff and a limited number of retries.
    *   **Dead Letter Queue (DLQ):** For payments that consistently fail after retries, consider sending them to a DLQ for later inspection and manual intervention. This prevents them from being lost entirely.
    *   **Idempotency Key Management:** Ensure that the `correlationId` is effectively used across all retries and external calls to prevent duplicate processing by the payment processors themselves.
    *   **Health Check Strategy:** Refine the health check logic to be more adaptive to the external processors' behavior. For instance, instead of just a boolean `failing` status, consider using a more granular metric like response time or error rate to make more informed decisions about which processor to use.

*   **Performance Enhancements:**
    *   **Worker Concurrency:** Experiment with the number of goroutines (workers) processing the `paymentQueue` to find the optimal balance between resource utilization and throughput.
    *   **Batching:** If possible, batch multiple payment requests when sending them to the external processors to reduce network overhead (though this might complicate idempotency and error handling).
    *   **Connection Pooling Tuning:** Further fine-tune HTTP client connection pool parameters (`MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout`) for both the API Gateway and Worker.

*   **Codebase Refinements:**
    *   **Error Handling:** Implement more granular and informative error handling throughout the application, especially for external service interactions.
    *   **Logging:** Enhance logging to provide more insights into the payment processing flow, including successful and failed external calls, and Redis interactions.

These are some initial thoughts, and further investigation and profiling would be necessary to pinpoint the most impactful optimizations.

## ⚖️ License

This project is open-source and available under the MIT License.
