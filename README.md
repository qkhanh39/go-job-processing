# go-job-processing

A background **job processing** service written in Go. It exposes an HTTP API to
submit asynchronous jobs, persists them in PostgreSQL, and runs them through a
**concurrent worker pool** with automatic retries, backpressure, and graceful
shutdown.

Currently supports a single job type (`SEND_EMAIL`) as a reference
implementation — the architecture is designed so new job types can be added by
registering one handler.

## Features

- **HTTP API** to submit jobs (`POST /api/v1/jobs`)
- **Concurrent worker pool** — one dispatcher claims jobs and a configurable set of
  executors run them in parallel
- **PostgreSQL-backed queue** using `FOR UPDATE SKIP LOCKED` for safe concurrent claiming
- **Automatic retry** with exponential backoff (1s base, 1m cap) up to a max attempt count
- **Bounded in-memory queue** that backpressures the dispatcher
- **Immediate wake-up** on new job submission (channel notifier) with polling as a fallback
- **Graceful shutdown** — stops dispatching, drains in-flight work, then interrupts
- **Clean/hexagonal architecture** with interfaces at every boundary
- **Structured logging** (`log/slog`)

## Architecture

```
Client
  │  POST /api/v1/jobs
  ▼
HTTP Handler ──► JobService.Create ──► PostgreSQL (status = PENDING)
                                            │
                                            ▼
                                   Notifier.Notify()   (wake-up signal)
                                            │
                                            ▼
                              Dispatcher  (ClaimNext via FOR UPDATE SKIP LOCKED)
                                            │
                                            ▼
                              Bounded job queue (chan *Job)
                                            │
                     ┌──────────────────────┼──────────────────────┐
                     ▼                      ▼                      ▼
               Executor 1             Executor 2             Executor N
                     │                      │                      │
                     └──────────────────────┼──────────────────────┘
                                            ▼
                              JobService.Execute ──► job handler
                                            │
                              ┌─────────────┴─────────────┐
                              ▼                           ▼
                        COMPLETED                 retry (backoff) / FAILED
```

### Project structure

```
.
├── cmd/
│   └── http-server/
│       └── main.go              # entrypoint: config, wiring, lifecycle
├── internal/
│   ├── adapter/
│   │   ├── http/                # HTTP adapter
│   │   │   ├── router.go        # chi router, route definitions
│   │   │   ├── handler/         # HTTP handlers
│   │   │   └── dto/             # request/response DTOs
│   │   └── repository/
│   │       └── postgres.go      # PostgreSQL JobRepository (pgxpool)
│   ├── config/
│   │   └── config.go            # Viper-based configuration
│   ├── connector/
│   │   └── db.go                # PostgreSQL connection pool
│   ├── entities/                # domain model
│   │   ├── job.go               # Job entity
│   │   ├── type.go              # JobType
│   │   ├── status.go            # Status
│   │   └── error.go             # sentinel errors
│   ├── notification/            # channel-based wake-up notifier
│   ├── service/
│   │   ├── job/                 # JobService: create / execute / claim / retry
│   │   ├── handler/             # per-job-type handlers (send_email)
│   │   └── dto/job/             # service-layer DTOs
│   └── worker/
│       ├── pool.go              # concurrent worker pool (dispatcher + executors)
│       └── pool_test.go         # unit tests
├── db_migrations/               # golang-migrations SQL files
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── .env.example
```

### Design notes

- **Dependency inversion** — `JobService` depends on the `JobRepository`, `JobHandler`,
  and `JobNotifier` interfaces, not concrete types. Adapters (Postgres, channel
  notifier, HTTP) are wired in `main.go`.
- **Strategy pattern** — job handlers are registered in a map keyed by `JobType`, so
  adding a job type is a one-file change plus a map entry.
- **Worker pool** — a single dispatcher is the sole claimer and sender; executors only
  consume from the bounded channel. This keeps claiming safe (no double-claim) and
  bounds memory. See [Worker pool](#worker-pool).

## Tech stack

| Concern      | Choice                                  |
| ------------ | --------------------------------------- |
| Language     | Go (module targets `go 1.26`)           |
| HTTP routing | [chi](https://github.com/go-chi/chi)    |
| Database     | PostgreSQL 17 via [pgx v5](https://github.com/jackc/pgx) |
| Config       | [Viper](https://github.com/spf13/viper) + `godotenv` |
| Migrations   | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Shared lib   | `github.com/qkhanh39/go-common` (validation, API envelope, app errors) |
| Packaging    | Docker (multi-stage), Docker Compose    |

## Prerequisites

- Go **1.26+**
- Docker + Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) (`migrate` on PATH) for running migrations

## Getting started

```bash
# 1. Configure
cp .env.example .env        # then edit values if needed

# 2. Start Postgres and apply migrations
make reset-db

# 3. Build and run the service (Postgres + HTTP server)
make up
```

The service listens on `:8080` by default.

### Submit a job

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "SEND_EMAIL",
    "payload": {
      "to": "user@example.com",
      "subject": "Hello",
      "body": "World"
    }
  }'
```

A `201 Created` response returns the created job (wrapped in the `go-common` API
envelope):

```json
{
  "id": "…",
  "type": "SEND_EMAIL",
  "payload": { "to": "user@example.com", "subject": "Hello", "body": "World" },
  "status": "PENDING",
  "attempts": 0,
  "max_attempts": 3,
  "run_at": "2026-10-19T00:00:00Z",
  "created_at": "2026-10-19T00:00:00Z"
}
```

> **Note:** the `SEND_EMAIL` handler is a reference stub — it validates the payload
> and returns `{"sent": true}` without sending a real email. Swap in a real mailer
> inside `internal/service/handler/send_email_handler.go`.

## Configuration

All configuration is read from environment variables (or a local `.env` file).

| Variable                  | Default   | Description                              |
| ------------------------- | --------- | ---------------------------------------- |
| `SERVICE_NAME`            | —         | Service name                             |
| `ENV`                     | `local`   | Environment name                         |
| `DB_SOURCE_HOST`          | —         | Postgres host                            |
| `DB_SOURCE_PORT`          | `5432`    | Postgres port                            |
| `DB_SOURCE_DATABASE`      | —         | Database name                            |
| `DB_SOURCE_USERNAME`      | —         | Database user                            |
| `DB_SOURCE_PASSWORD`      | —         | Database password                        |
| `DB_SOURCE_SSL_MODE`      | `disable` | Postgres SSL mode                        |
| `MAX_ATTEMPTS`            | `5`       | Max execution attempts per job           |
| `WORKER_COUNT`            | `4`       | Number of concurrent executors           |
| `WORKER_QUEUE_SIZE`       | `8`       | Bounded in-memory queue capacity         |
| `WORKER_POLL_INTERVAL`    | `5s`      | Polling fallback interval                |
| `WORKER_SHUTDOWN_TIMEOUT` | `10s`     | Grace period before interrupting work    |
| `SERVER_HTTP_PORT`        | —         | HTTP listen port                         |
| `SERVER_HTTP_TIMEOUT`     | —         | HTTP server read/write timeout           |

## Job lifecycle & retry

A job moves through these states:

```
PENDING ──► PROCESSING ──► COMPLETED
                  │
                  └──────► FAILED   (after MAX_ATTEMPTS)
```

- On a failed execution, the job is rescheduled as `PENDING` with
  `run_at = now + backoff(attempts)`, where backoff doubles from `1s` up to a `1m` cap.
- Once `attempts` reaches `max_attempts`, the job is marked `FAILED`.

## Worker pool

The worker pool (`internal/worker/pool.go`) is a **dispatcher + N executors** design:

- **Dispatcher** — the single goroutine that claims jobs. It wakes on a notification
  or a polling tick, then drains available jobs into a **bounded channel**. Because it
  is the only claimer and the only sender, claiming stays safe and the channel provides
  backpressure: when the queue is full the dispatcher stops claiming.
- **Executors** — `WORKER_COUNT` goroutines that read from the channel and call
  `JobService.Execute` concurrently.
- **Claiming** — `ClaimNext` uses `FOR UPDATE SKIP LOCKED`, so concurrent claims never
  double-claim the same row.
- **Shutdown** — on context cancellation the dispatcher stops claiming, the queue is
  closed, and executors drain remaining work. A separate execution context is used so a
  server shutdown does not immediately cancel in-flight jobs; if the drain does not
  finish within `WORKER_SHUTDOWN_TIMEOUT`, the execution context is cancelled.

## Testing

Unit tests for the worker pool live in `internal/worker/pool_test.go` and use a fake
`JobService` (no database required). They cover concurrent execution, queue-capacity
backpressure, graceful shutdown, and recovery from transient claim errors.

```bash
go test ./...
go test -race ./...
```

## Database migrations

Migrations are plain SQL managed by `golang-migrate` in `db_migrations/`.

```bash
make migrate-up          # apply pending migrations
make migrate-down        # roll back the last migration
make migrate-create name=add_foo   # scaffold a new migration pair
make reset-db            # wipe data, restart Postgres, re-apply migrations
```

## Makefile targets

| Target             | Description                                  |
| ------------------ | -------------------------------------------- |
| `make up`          | Build and start all services (detached)      |
| `make down`        | Stop all services                            |
| `make logs`        | Tail service logs                            |
| `make migrate-up`  | Apply pending migrations                     |
| `make migrate-down`| Roll back the last migration                 |
| `make migrate-create` | Scaffold a new migration (set `name=`)   |
| `make reset-db`    | Wipe data, restart Postgres, re-apply        |

## Limitations / future work

- The `SEND_EMAIL` handler is a stub (no real email delivery).
- No job query/cancel endpoints (only job creation).
- No stale-`PROCESSING` recovery: a job interrupted by a crash or forced termination
  can remain in `PROCESSING`. A reaper/lease mechanism would address this.
- No metrics/health endpoints or authentication.
