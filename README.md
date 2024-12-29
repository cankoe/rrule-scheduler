# Event Scheduler

Event Scheduler is a service that allows you to create recurring schedules using RRULEs and trigger callbacks at the specified times. It includes components to manage schedules, pre-queue events, dispatch them to a worker queue, and a worker to execute callbacks from the worker_queue.

## Features

- **Create and manage schedules**: Define recurring events using RRULE strings.
- **Pre-queuing events**: Generate future events within a specific timeframe.
- **Dispatcher service**: Moves due events into a worker queue.
- **Worker service**: Executes the callback URLs for triggered events, with retries.

## Components

- **API**: A Gin-based HTTP API providing endpoints for schedules and events.
- **Prequeuer**: Regularly checks for upcoming schedule occurrences and enqueues them as events.
- **Dispatcher**: Monitors a "ready_queue" and moves events to a "worker_queue" when due.
- **Worker**: Dequeues events and executes their callback URLs.

## Project Structure

- `cmd/api`: The main API server
- `cmd/prequeuer`: Pre-queuer service
- `cmd/dispatcher`: Dispatcher service
- `cmd/worker`: Worker service
- `internal/`: Shared internal packages (config, database, helpers, models)
- `api/`: API route definitions and middleware
- `config/config.yaml`: Default configuration

## Getting Started

### Prerequisites
- Go 1.23.2+
- Docker & Docker Compose
- MongoDB & Redis if running locally without Docker

### Running via Docker Compose

1. Ensure Docker and Docker Compose are installed.
2. Run `docker-compose up --build`.
3. The API will be available at `http://localhost:8080`.

### Configuration
Configuration is loaded from `config/config.yaml` and can be overridden by environment variables. See `internal/config/config.go` for environment variable overrides.

### Testing API Endpoints
Use tools like `curl` or `Postman`:
```bash
curl -H "X-API-KEY: user-api-key-123" -X POST -d '{"name":"Daily Backup","rrule":"FREQ=DAILY;INTERVAL=1","callback_url":"https://example.com/callback"}' http://localhost:8080/api/schedules
```


### OpenAPI Specification
An openapi.yml file is included at the root. You can load this into Swagger UI or another tool to browse and test the API interactively.

### Contributing
	1.	Fork the repository
	2.	Create a feature branch
	3.	Submit a Pull Request

### License
