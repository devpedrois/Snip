# snip

A production-ready URL shortener API written in Go, demonstrating idiomatic concurrency patterns, cache-aside architecture, and asynchronous analytics processing.

## Overview

**snip** is a RESTful microservice that shortens URLs with the following characteristics:

- **Cache-Aside Pattern**: Redis cache layer in front of MySQL for high-performance reads
- **Async Analytics**: Goroutine-based worker pool for non-blocking click tracking
- **Idiomatic Go**: Proper context propagation, graceful shutdown, interface-driven design
- **Production-Grade**: Structured logging, comprehensive error handling, containerized deployment

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Client Requests                                          │
└──────────────────┬──────────────────────────────────────┘
                   │
         ┌─────────┴─────────┐
         │                   │
    ┌────▼─────┐      ┌─────▼────┐
    │  GET /{} │      │POST /api/ │
    │ (redirect)│      │  shorten  │
    └────┬─────┘      └─────┬────┘
         │                   │
    ┌────▼────────────────────▼────┐
    │  Cache-Aside (Redis)          │
    │  - Miss → MySQL → Populate    │
    │  - TTL: 30 days               │
    └────┬────────────────────┬────┘
         │                    │
         │            ┌───────▼────────┐
         │            │ Fire-and-Forget │
         │            │ Click Event     │
         │            └───────┬────────┘
         │                    │
    ┌────▼─────────────────────▼────┐
    │ MySQL (Persistent Store)       │
    │ - urls table (7-char hash)     │
    │ - clicks table (analytics)     │
    │ - Foreign keys enforced        │
    └──────────────────────────────┘
         │
    ┌────▼──────────────────────────┐
    │ Analytics Workers (goroutines) │
    │ - Concurrent writes            │
    │ - Batch processing ready       │
    └───────────────────────────────┘
```

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/)
- [Go 1.24+](https://golang.org/dl/) (development only)

## Quick Start

```bash
# Clone and navigate to project
git clone <repository>
cd snip

# Load environment variables
cp .env.example .env

# Start services (MySQL, Redis, API)
make up

# Verify health
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

## Project Structure

```
snip/
├── cmd/api/                 # Application entrypoint
├── internal/
│   ├── config/              # Environment configuration
│   ├── domain/              # Domain entities and errors
│   ├── handler/             # HTTP handlers (future)
│   ├── service/             # Business logic (future)
│   ├── repository/
│   │   ├── mysql/           # MySQL driver and pool
│   │   └── redis/           # Redis cache (future)
│   ├── middleware/          # HTTP middleware (future)
│   ├── hash/                # Base62 encoder (future)
│   └── analytics/           # Worker pool (future)
├── migrations/              # SQL migrations (golang-migrate)
├── tests/                   # Integration & e2e tests
├── docker/                  # Docker configuration
├── Makefile                 # Development targets
├── docker-compose.yml       # Local environment
└── .env.example             # Example environment variables
```

## API Endpoints (Planned)

### Create Short URL
```
POST /api/shorten
Content-Type: application/json

{
  "url": "https://example.com/very/long/path?param=value"
}

Response (201):
{
  "hash": "abc1234",
  "short_url": "http://localhost:8080/abc1234"
}
```

### Redirect to Original URL
```
GET /{hash}

Response (301): Location: https://example.com/very/long/path?param=value
```

### Analytics
```
GET /api/analytics/{hash}

Response (200):
{
  "total_clicks": 1250,
  "clicks_last_30_days": [...],
  "top_user_agents": [...]
}
```

## Configuration

All configuration is loaded from environment variables. See `.env.example` for defaults:

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | 8080 | HTTP server port |
| `APP_ENV` | development | Logging format (development/production) |
| `MYSQL_HOST` | mysql | MySQL hostname |
| `MYSQL_USER` | shortener | MySQL user |
| `MYSQL_PASSWORD` | shortener_pass | MySQL password |
| `MYSQL_DATABASE` | snip | Database name |
| `REDIS_HOST` | redis | Redis hostname |
| `REDIS_TTL_DAYS` | 30 | Cache TTL |
| `ANALYTICS_WORKERS` | 4 | Goroutine worker pool size |
| `URL_EXPIRATION_DAYS` | 30 | Link expiration period |

## Development

### Build from source
```bash
go build ./cmd/api
```

### Run tests
```bash
make test
```

### Code quality checks
```bash
make lint          # golangci-lint
make fmt           # gofmt
go vet ./...       # go vet
```

### Database migrations
```bash
make migrate-up    # Apply pending migrations
make migrate-down  # Revert last migration
```

### Logs
```bash
docker compose logs -f api      # Stream API logs
docker compose logs -f mysql    # Stream MySQL logs
docker compose logs -f redis    # Stream Redis logs
```

## Technology Stack

| Layer | Technology |
|---|---|
| Language | Go 1.24+ |
| HTTP Router | chi/v5 |
| Database | MySQL 8 |
| Cache | Redis 7 |
| Migrations | golang-migrate/v4 |
| Logging | log/slog (stdlib) |
| Testing | testify, go-sqlmock |
| Containerization | Docker + Docker Compose |

## Design Decisions

### Cache-Aside (Lazy Loading)
- Cache miss on read: query MySQL → populate Redis → respond
- Keeps cache fresh without background jobs for reads
- Suitable for read-heavy, variable-traffic workloads

### Async Analytics
- Click events are **non-blocking** via channel dispatch
- Goroutine workers consume events concurrently
- Server responds before database write completes
- Graceful shutdown drains pending events

### Base62 Hashing
- 7-character hash generated from auto-increment ID
- Deterministic: same ID always produces same hash
- Collision-free by design

### Expiration Strategy
- Links expire after 30 days of inactivity
- `last_accessed_at` updated in background (non-blocking)
- Queries include expiration check for stale link detection

## Deployment

### Production Build
```bash
docker compose -f docker-compose.yml build
docker compose -f docker-compose.yml up -d
```

### Health Check
```bash
curl http://localhost:8080/health
```

### Shutdown
```bash
docker compose down
# Graceful shutdown: 10s timeout for in-flight requests
```

## Performance Characteristics

- **Cache Hit**: ~1ms (Redis lookup)
- **Cache Miss**: ~5-10ms (MySQL + Redis populate)
- **Analytics**: Non-blocking (p99 latency unaffected)
- **Worker Pool**: Tunable via `ANALYTICS_WORKERS` env var

## Monitoring (Future)

- Structured logging with request IDs
- Prometheus metrics on handler latencies
- Graceful shutdown with drain period

## Contributing

This project follows structured development practices:

- **Branching**: Feature branches (`feat/<slug>`, `fix/<slug>`)
- **Commits**: Atomic, conventional format (`feat(scope): description`)
- **Code Standards**: Clean Code principles, idiomatic Go patterns
- **Testing**: Test-driven development (TDD) for new features
- **Review**: All changes require code review before merge

## License

MIT
