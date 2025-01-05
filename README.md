# RRule Scheduler

A **distributed event scheduling** system built with Go, MongoDB, and Redis. This project automates the creation, dispatching, and execution of scheduled events based on RRULE definitions.

## Table of Contents
- [RRule Scheduler](#rrule-scheduler)
	- [Table of Contents](#table-of-contents)
	- [Overview](#overview)
	- [Architecture](#architecture)
	- [Services](#services)
		- [API Service](#api-service)
		- [PreQueuer Service](#prequeuer-service)
		- [Dispatcher Service](#dispatcher-service)
		- [Worker Service](#worker-service)
	- [Quick Start](#quick-start)
		- [Using Docker Compose](#using-docker-compose)
		- [Running From Source](#running-from-source)
	- [Configuration](#configuration)
	- [API Documentation](#api-documentation)
		- [Examples](#examples)
		- [RRULE Examples](#rrule-examples)
	- [Project Structure](#project-structure)
	- [Logging](#logging)
	- [Contributing](#contributing)
	- [License](#license)

## Overview

The **Event Scheduler** project is designed to help developers schedule recurring or one-time tasks that are executed at specified times in the future. It uses:
- **MongoDB** for persistent storage of schedules and events.
- **Redis** for queueing events that are about to be executed.
- **Go (Golang)** for the various microservices and CLI tools to manage schedules, pre-queue events, dispatch them to workers, and finally execute callbacks.

This system is particularly useful when you need robust, distributed scheduling with clear separation of concerns and resilience.



## Architecture

The system follows a **microservices** approach, with each service focusing on a specific responsibility:

1. **API**  
   Handles creating, reading, updating, and deleting schedules. Also provides endpoints to list pending and archived events.

2. **PreQueuer**  
   Periodically scans schedules to pre-generate events for the near future and enqueues them into a Redis **ready_queue**.

3. **Dispatcher**  
   Monitors the **ready_queue** in Redis. When an event is due, the **Dispatcher** removes it from the ready queue, updates its status to `worker_queue`, and enqueues it for processing by the **Worker**.

4. **Worker**  
   Polls the **worker_queue** for events. Executes scheduled callbacks (HTTP requests), and upon success, archives the event; on failure, retries a configurable number of times before marking the event as `error`.



## Services

Below is a brief summary of each service. (For full source code, see [`cmd/`](./cmd) directory.)

### API Service

- **Path**: `cmd/api/main.go`
- **Role**: 
  - Exposes REST endpoints to manage schedules and query events.
  - Uses [Gin](https://github.com/gin-gonic/gin) web framework.
  - Persists schedule data in MongoDB.
  - OpenAPI/Swagger documentation available at [`docs/openapi.yml`](./docs/openapi.yml), served by the API at [`/docs/openapi.yml`](http://localhost:8080/docs/openapi.yml) and via the built-in Swagger UI at [`/swagger-ui`](http://localhost:8080/swagger-ui).

### PreQueuer Service

- **Path**: `cmd/prequeuer/main.go`
- **Role**:
  - Periodically scans all existing schedules in MongoDB.
  - Generates future events (up to a configured time window).
  - Inserts these events into the `events` collection and places them into a Redis **ready_queue** with a timestamp score.
  - Configuration for the scanning interval and how far ahead to generate events is in `config.yaml` (under `prequeuer`).

### Dispatcher Service

- **Path**: `cmd/dispatcher/main.go`
- **Role**:
  - Monitors Redis `ready_queue` for events whose scheduled time has arrived (score <= current timestamp).
  - Moves due events to the `worker_queue`, updating their status in MongoDB to `worker_queue`.
  - If an event fails to be moved or updated, marks it as `error`.

### Worker Service

- **Path**: `cmd/worker/main.go`
- **Role**:
  - Continuously polls the `worker_queue` (Redis list).
  - Performs the HTTP callback for each event.
  - Retries the callback a configured number of times (`max_retries`) on failure.
  - Archives the event into `archived_events` on success or marks it as `error` on unrecoverable failure.


## Quick Start

### Using Docker Compose

1. **Build and run** the stack:

   ```bash
   docker-compose up --build
   ```

2.	**Services**:
	•	**MongoDB** available on port 27017.
	•	**Redis** available on port 6379.
	•	**API** exposed on port 8080.
	•	**PreQueuer**, **Dispatcher**, and **Worker** run in the background.

3.	**Verify** that everything is up and running:
	```bash
	docker-compose ps
	```

	Check logs with:
	```bash
	docker-compose logs -f
	```

4. **Access the API**:
- Visit: http://localhost:8080/swagger-ui for the Swagger UI
- Or see the raw OpenAPI spec at: http://localhost:8080/docs/openapi.yml

### Running From Source
1. **Start MongoDB and Redis** (e.g., via Docker or local installation):
	```bash
	docker run -p 27017:27017 --name mongodb mongo:6.0
	docker run -p 6379:6379 --name redis redis:7.0
	```

2. **Install Go dependencies**:
	```bash
	go mod tidy
	```

3. **Compile and run** each service separately (in different terminals or using a process manager):
	- API
	```bash
	go run cmd/api/main.go
	```
	- PreQueuer
	```bash
	go run cmd/prequeuer/main.go
	```
	- Dispatcher
	```bash
	go run cmd/dispatcher/main.go
	```
	- Worker
	```bash
	go run cmd/worker/main.go
	```

4. **Configuration** can be done via config/config.yaml, environment variables (e.g., MONGO_URI, REDIS_HOST), or command-line flags (e.g., --worker-count=3).

## Configuration
The main configuration is located in [config/config.yaml](./config/config.yaml). It includes:

```yaml
mongo:
  uri: "mongodb://localhost:27017"
  database: "schedulerdb"

redis:
  host: "localhost"
  port: 6379

prequeuer:
  ticker_interval_seconds: 20
  event_timeframe_minutes: 10

worker:
  count: 5
  max_retries: 3

log:
  level: "info"
```

- **mongo**: MongoDB connection parameters.
- **redis**: Redis connection parameters.
- **prequeuer**:
  - **ticker_interval_seconds**: How often the PreQueuer scans for new events.
  - **event_timeframe_minutes**: How far into the future events should be generated.
- **worker**:
  - **count**: How many worker routines should be started.
  - **max_retries**: How often should a worker retry a failed callback.
- **log**: Logging level (e.g., info, debug, warn, error).

You can **override** these values with environment variables or command-line flags:
- Environment variables are automatically bound:
  ```bash
  MONGO_URI=mongodb://localhost:27017
  MONGO_DATABASE=schedulerdb

  REDIS_HOST=localhost
  REDIS_PORT=6379

  PREQUEUER_TICKER_INTERVAL_SECONDS=20
  PREQUEUER_EVENT_TIMEFRAME_MINUTES=10
  
  WORKER_COUNT=5
  WORKER_MAX_RETRIES=3

  LOG_LEVEL=info
  ```

- Supported Command-line flags are `prequeuer-ticker-seconds`, `prequeuer-timeframe-minutes`, `worker-count`, `worker-max-retries`, and `log-level`, e.g.:

	```bash
	./prequeuer --prequeuer-ticker-seconds=20 --prequeuer-timeframe-minutes=10 --log-level=info
	```

	```bash
	./worker --worker-count=5 --worker-max-retries=3 --log-level=info
	```

## API Documentation
OpenAPI documentation is available at [docs/openapi.yml](./docs/openapi.yml)

•**Endpoint**: GET /docs/openapi.yml

•**Swagger UI**: GET /swagger-ui (when the API service is running)

**Note**: The system uses [RRULE strings](https://icalendar.org/iCalendar-RFC-5545/3-8-5-3-recurrence-rule.html) for recurring schedules (powered by [teambition/rrule-go](https://github.com/teambition/rrule-go)).

### Examples
1. Create a New Schedule
	**Endpoint**: `POST /api/schedules`

	**Request**:
	```json
	{
		"name": "Daily Backup",
		"rrule": "FREQ=DAILY;INTERVAL=1",
		"callback_url": "https://example.com/backup",
		"method": "POST",
		"headers": {
			"Authorization": "Bearer abc123",
			"Content-Type": "application/json"
		},
		"body": "{\"task\":\"backup\"}"
	}
	```

	**Response**:
	```json
	{
		"id": "64b76c5986b6c9f24f1c0952"
	}
	```

2. Update an Existing Schedule
	**Endpoint**: `PUT /api/schedules/{scheduleId}`

	Replace `{scheduleId}` with a real schedule ID, e.g., 64b76c5986b6c9f24f1c0952.

	**Request**:
	```json
	{
		"name": "Updated Daily Backup",
		"rrule": "FREQ=DAILY;INTERVAL=2",
		"callback_url": "https://example.com/new-backup",
		"method": "PUT",
		"headers": {
			"Authorization": "Bearer updated_token",
			"Content-Type": "application/json"
		},
		"body": "{\"task\":\"updated-backup\"}"
	}
	```

	**Response**:
	```json
	{
		"message": "Schedule updated successfully."
	}
	```

3. Delete a Schedule
	**Endpoint**: `DELETE /api/schedules/{scheduleId}`

	Replace `{scheduleId}` with a real schedule ID, e.g., 64b76c5986b6c9f24f1c0952.

	**Request**:

	`DELETE /api/schedules/64b76c5986b6c9f24f1c0952`

	**Response**:
	```json
	{
		"message": "Schedule and associated events deleted."
	}
	```

4. List Pending Events for a Schedule
	**Endpoint**: `GET /api/schedules/{scheduleId}/events/pending`

	Replace `{scheduleId}` with a real schedule ID, e.g., 64b76c5986b6c9f24f1c0952.

	**Request**:

	`GET /api/schedules/64b76c5986b6c9f24f1c0952/events/pending?limit=5&page=1`

	**Response**:
	```JSON
	{
		"events": [
			{
			"_id": "64c10d4286b6c9f24f1c0952",
			"schedule_id": "64b76c5986b6c9f24f1c0952",
			"run_time": "2025-01-05T12:00:00Z",
			"status": [
				{
				"time": "2025-01-04T12:00:00Z",
				"status": "ready_queue",
				"message": "Event pre-queued for ready queue"
				}
			],
			"created_at": "2025-01-04T11:00:00Z"
			}
		],
		"page": 1,
		"limit": 5
	}
	```

5. List Archived (Historical) Events for a Schedule
	**Endpoint**: `GET /api/schedules/{scheduleId}/events/history`

	Replace `{scheduleId}` with a real schedule ID, e.g., 64b76c5986b6c9f24f1c0952.

	**Request**:

	`GET /api/schedules/64b76c5986b6c9f24f1c0952/events/history?limit=5&page=1`

	**Response**:
	```JSON
	{
		"events": [
			{
			"_id": "64c10d4286b6c9f24f1c0953",
			"schedule_id": "64b76c5986b6c9f24f1c0952",
			"run_time": "2025-01-03T12:00:00Z",
			"status": [
				{
				"time": "2025-01-03T12:00:00Z",
				"status": "completed",
				"message": "Event successfully processed"
				}
			],
			"created_at": "2025-01-03T11:00:00Z"
			}
		],
		"page": 1,
		"limit": 5
	}
	```

### RRULE Examples
1. **Daily Recurrence at 8:30 AM**
	```RRULE
	DTSTART:20250101T083000Z
	RRULE:FREQ=DAILY;INTERVAL=1
	```
	**Description**: Occurs every day at 8:30 AM (UTC).
	- January 1, 2025, at 8:30 AM
	- January 2, 2025, at 8:30 AM
	- January 3, 2025, at 8:30 AM

2. **Weekly on Specific Days**
	```RRULE
	DTSTART:20250101T151500Z
	RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,WE,FR
	```
	**Description**: Occurs every Monday, Wednesday, and Friday at 3:15 PM (UTC).
	- January 1, 2025 (Wednesday), at 3:15 PM
	- January 3, 2025 (Friday), at 3:15 PM
	- January 6, 2025 (Monday), at 3:15 PM

3. **Monthly on the 1st and 15th at 10:00 AM**
	```RRULE
	DTSTART:20250101T100000Z
	RRULE:FREQ=MONTHLY;INTERVAL=1;BYMONTHDAY=1,15
	```
	**Description**: Occurs on the 1st and 15th of every month at 10:00 AM (UTC).
	- January 1, 2025, at 10:00 AM
	- January 15, 2025, at 10:00 AM
	- February 1, 2025, at 10:00 AM
	- February 15, 2025, at 10:00 AM

4. **Yearly on December 25th at 7:00 AM**
	```RRULE
	DTSTART:20250101T070000Z
	RRULE:FREQ=YEARLY;BYMONTH=12;BYMONTHDAY=25
	```
	**Description**: Occurs every year on December 25th at 7:00 AM (UTC).
	- December 25, 2025, at 7:00 AM
	- December 25, 2026, at 7:00 AM

5. **Event Ends After 5 Occurrences at 6:45 PM**
	```RRULE
	DTSTART:20250101T184500Z
	RRULE:FREQ=DAILY;INTERVAL=1;COUNT=5
	```
	**Description**: Occurs daily at 6:45 PM (UTC) but stops after 5 occurrences.
	- January 1, 2025, at 6:45 PM
	- January 2, 2025, at 6:45 PM
	- January 3, 2025, at 6:45 PM
	- January 4, 2025, at 6:45 PM
	- January 5, 2025, at 6:45 PM

6. **Event Ends on a Specific Date (January 10, 2025, at 5:00 PM)**
	```RRULE
	DTSTART:20250101T170000Z
	RRULE:FREQ=DAILY;UNTIL=20250110T170000Z
	```
	**Description**: Occurs daily at 5:00 PM (UTC) until January 10, 2025.
	- January 1, 2025, at 5:00 PM
	- January 2, 2025, at 5:00 PM
	- January 3, 2025, at 5:00 PM
	- …
	- January 10, 2025, at 5:00 PM


7. **Hourly Recurrence Every 3 Hours Starting at 2:00 AM**
	```RRULE
	DTSTART:20250101T020000Z
	RRULE:FREQ=HOURLY;INTERVAL=3
	```
	**Description**: Occurs every 3 hours starting at 2:00 AM (UTC).
	- January 1, 2025, at 2:00 AM
	- January 1, 2025, at 5:00 AM
	- January 1, 2025, at 8:00 AM
	- January 1, 2025, at 11:00 AM


8. **Specific Days of the Month at 11:30 PM**
	```RRULE
	DTSTART:20250101T233000Z
	RRULE:FREQ=MONTHLY;BYDAY=MO;BYSETPOS=2
	```
	**Description**: Occurs on the second Monday of every month at 11:30 PM (UTC).
	- January 13, 2025, at 11:30 PM
	- February 10, 2025, at 11:30 PM
	- March 10, 2025, at 11:30 PM


## Project Structure
```
.
├── cmd/
│   ├── api/
│   │   └── main.go          # Entry point for the API service
│   ├── dispatcher/
│   │   └── main.go          # Entry point for the Dispatcher service
│   ├── prequeuer/
│   │   └── main.go          # Entry point for the PreQueuer service
│   └── worker/
│       └── main.go          # Entry point for the Worker service
├── config/
│   └── config.yaml          # Main configuration file
├── docs/
│   └── openapi.yml          # API documentation (OpenAPI spec)
├── internal/
│   ├── api/                 # API route registration
│   ├── config/              # Configuration loading logic
│   ├── database/            # Database connection helpers (Mongo, Redis)
│   ├── dispatcher/          # Dispatcher logic
│   ├── events/              # Event status updates, archiving
│   ├── helpers/             # Common initialization and teardown
│   ├── models/              # MongoDB models (schedules, events)
│   ├── prequeuer/           # Logic for generating and scheduling events
│   ├── queue/               # Redis queue logic
│   ├── schedules/           # Schedule CRUD logic
│   └── worker/              # Worker logic (processing event callbacks)
├── docker-compose.yml       # Docker Compose for local development
├── Dockerfile               # Multi-stage Docker build
└── swagger-ui/              # Static Swagger UI files
```

**How It Works (Scheduling Flow)**

1. **User Creates a Schedule**
    - **API** inserts a new document into MongoDB’s schedules collection.

2. **PreQueuer Generates Events**
    - Every `prequeuer.ticker_interval_seconds`, the PreQueuer:
      1. Reads all schedules.
      2. Uses each schedule’s RRULE to find occurrences in `[now, now + event_timeframe_minutes)`.
      3. For each occurrence, creates a new document in MongoDB’s `events` collection and adds the event ID into Redis `ready_queue` (scored by the event’s run time).

3. **Dispatcher Dispatches Due Events**
    - Looks for events in `ready_queue` with a score <= current time (meaning the event is due).

    - Moves them from `ready_queue` to `worker_queue` and updates the event’s status in MongoDB to `worker_queue`.

4. **Worker Executes the Events**
   - Polls the `worker_queue`.
   - Fetches the event and corresponding schedule from MongoDB.
   - Makes an HTTP request (POST, GET, etc.) to the schedule’s `callback_url`.
   - On success:
     - Updates/archives the event (moves it to `archived_events`).
   - On failure:
     - Retries up to `worker.max_retries`.
     - If all retries fail, marks the event as error and moves it to `archived_events`.

5. **Query Schedules and Events**

   - **API** can return schedules, upcoming events, and archived (finished) events.

## Logging

Logging is provided by [Zerolog](https://github.com/rs/zerolog). The default level is info but can be configured via LOG\_LEVEL or in config.yaml.

- **Info logs** provide normal operational messages (like “Event scheduled”, “Worker started”).

- **Debug logs** can be enabled for more verbose output (showing every Redis fetch operation, etc.).

- **Error logs** indicate operational errors (like failing to connect to MongoDB or failing to schedule an event).

## Contributing
1. Fork the repository
2. Create a new feature branch
3. Commit your changes
4. Push to your branch
5. Open a Pull Request

Feel free to open issues for bug reports or feature requests.

## License

This project is licensed under the MIT License.