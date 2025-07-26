# Rinha de Backend 2025 - Go Edition

This is a high-performance backend solution for the Rinha de Backend 2025 challenge, implemented in Go.

## Architecture

The solution utilizes the following components:

*   **Go Application:** Two instances of a custom Go application handle payment processing and summary generation.
*   **Redis:** Used as a fast, in-memory data store for maintaining consistent payment summaries and processor health status.
*   **Nginx:** Acts as a load balancer to distribute incoming traffic across the two Go application instances.

## Features

*   **`POST /payments`:** Processes payment requests, intelligently routing them to either a default or fallback payment processor based on their health status. Ensures idempotency using Redis.
*   **`GET /payments-summary`:** Provides an aggregated summary of processed payments from both default and fallback processors.
*   **Health Checks:** A background goroutine periodically checks the health of external payment processors and updates their status in Redis.
*   **High Performance:** Built with Go for its concurrency model and low resource consumption.

## Setup and Running

To run this project, ensure you have Docker and Docker Compose installed.

1.  **Build and Start:**

    ```bash
    docker-compose up --build
    ```

    This command will:
    *   Build the Go application Docker image.
    *   Start the Redis container.
    *   Start two instances of the Go application.
    *   Start the Nginx load balancer.

2.  **Access the API:**

    The API will be accessible via Nginx on port `9999`.

    *   **Process Payment:** `POST http://localhost:9999/payments`
    *   **Get Payment Summary:** `GET http://localhost:9999/payments-summary`

## Configuration

Environment variables for the Go application (set in `docker-compose.yml`):

*   `REDIS_ADDR`: Address of the Redis server (e.g., `redis:6379`).
*   `DEFAULT_PROCESSOR_URL`: URL of the default external payment processor.
*   `FALLBACK_PROCESSOR_URL`: URL of the fallback external payment processor.

## Development

To develop the Go application:

1.  Navigate to the `api` directory:

    ```bash
    cd api
    ```

2.  Run the application locally (ensure Redis is running and environment variables are set):

    ```bash
    go run main.go
    ```

## License

This project is open-source and available under the MIT License.
