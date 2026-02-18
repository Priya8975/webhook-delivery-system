# Webhook Delivery System

A fault-tolerant webhook delivery platform built in Go that reliably delivers event notifications to subscriber endpoints. Implements industry-standard patterns used by Stripe, GitHub, and Shopify for their webhook infrastructure.

## Features

- **Event Fan-out** — Publish events and automatically deliver to all matching subscribers
- **Pattern Matching** — Subscribe to exact events (`payment.completed`), wildcards (`payment.*`), or everything (`*`)
- **Guaranteed Delivery** — At-least-once delivery with exponential backoff retries
- **Circuit Breaker** — Per-endpoint circuit breakers prevent cascading failures
- **Rate Limiting** — Per-subscriber sliding window rate limiting with atomic Redis operations
- **Dead Letter Queue** — Failed deliveries captured for manual review and replay
- **HMAC Signatures** — Every delivery signed with HMAC-SHA256 for verification
- **Worker Pool** — Goroutine-based concurrent delivery engine with configurable pool size
- **Real-time Dashboard** — Live WebSocket feed of delivery events and system health

## Architecture

```
┌──────────────────────────────────────────────────┐
│              Dashboard (React + WebSocket)        │
└──────────────────────┬───────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────┐
│              API Server (Go + Chi)                │
│                                                   │
│  POST /events       → Publish events              │
│  POST /subscribers  → Register endpoints          │
│  GET  /deliveries   → Delivery logs               │
│  GET  /health       → System health               │
└────────┬──────────────────────────┬──────────────┘
         │                          │
┌────────▼────────┐      ┌─────────▼─────────┐
│   PostgreSQL    │      │     Redis          │
│                 │      │                    │
│  Events         │      │  Delivery Queue    │
│  Subscribers    │      │  Circuit Breaker   │
│  Delivery Logs  │      │  Rate Limits       │
│  Dead Letters   │      │                    │
└─────────────────┘      └─────────┬─────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │    Worker Pool (Goroutines)  │
                    │                              │
                    │  Dispatcher → Go Channels →  │
                    │  N Workers → HTTP POST →     │
                    │  Result Handler              │
                    └──────────────────────────────┘
```

## Tech Stack

| Component | Technology | Why |
|-----------|-----------|-----|
| API Server | Go + Chi | Lightweight, idiomatic Go router built on `net/http` |
| Database | PostgreSQL + pgx | ACID guarantees, JSONB payloads, fastest pure-Go driver |
| Queue & Cache | Redis | Sorted sets for priority queue, Lua scripts for atomic ops |
| Workers | Goroutines + Channels | Native concurrency — 1,000 workers in ~2MB memory |
| Dashboard | React + Tailwind | Real-time WebSocket feed |
| Infrastructure | Docker + Docker Compose | One-command setup |
| CI/CD | GitHub Actions | Automated testing, linting, builds |

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 14+
- Redis 7+
- Node.js 20+ (for dashboard)

### Run with Docker

```bash
docker compose up
```

### Run Locally

```bash
# Start dependencies (or use Docker: docker compose up postgres redis -d)
brew services start postgresql
brew services start redis

# Create database
createdb webhook

# Set environment variables
export DATABASE_URL=postgresql://$(whoami)@localhost:5432/webhook?sslmode=disable
export REDIS_URL=redis://localhost:6379/0

# Run the server (migrations run automatically)
go run ./cmd/server
```

## API Reference

### Health Check

```bash
curl http://localhost:8080/api/v1/health
```
```json
{"status": "healthy", "version": "1.0.0"}
```

### Create a Subscriber

```bash
curl -X POST http://localhost:8080/api/v1/subscribers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "endpoint_url": "https://acme.com/webhooks",
    "event_types": ["payment.completed", "order.*"]
  }'
```
```json
{
  "id": "a118483e-a9dd-439e-905e-ac33e23c0cf6",
  "name": "Acme Corp",
  "secret_key": "whsec_2e81b915e052cb9efe6d68117e458872..."
}
```

### List Subscribers

```bash
curl http://localhost:8080/api/v1/subscribers
```

### Get Subscriber Details

```bash
curl http://localhost:8080/api/v1/subscribers/{id}
```
```json
{
  "id": "a118483e-...",
  "name": "Acme Corp",
  "endpoint_url": "https://acme.com/webhooks",
  "is_active": true,
  "rate_limit_per_second": 10,
  "subscriptions": [
    {"event_type": "payment.completed", "is_active": true},
    {"event_type": "order.*", "is_active": true}
  ]
}
```

### Update a Subscriber

```bash
curl -X PATCH http://localhost:8080/api/v1/subscribers/{id} \
  -H "Content-Type: application/json" \
  -d '{"is_active": false, "rate_limit_per_second": 5}'
```

### Publish an Event

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "event_type": "payment.completed",
    "payload": {
      "amount": 99.99,
      "currency": "USD",
      "customer_id": "cust_123"
    }
  }'
```
```json
{
  "event_id": "85e73e85-a6e4-4fb0-ac5c-a5f23823c324",
  "event_type": "payment.completed",
  "deliveries_queued": 2
}
```

### List Events

```bash
curl "http://localhost:8080/api/v1/events?event_type=payment.completed&limit=10"
```

### Event Type Pattern Matching

| Pattern | Matches | Example |
|---------|---------|---------|
| `payment.completed` | Exact match only | `payment.completed` |
| `payment.*` | Any event starting with `payment.` | `payment.completed`, `payment.refunded` |
| `*` | All events | Everything |

## Database Schema

Five core tables with proper indexing:

- **subscribers** — Webhook endpoints with secret keys and rate limits
- **events** — Published events with JSONB payloads
- **subscriptions** — Maps subscribers to event type patterns
- **delivery_attempts** — Every delivery try with status, timing, and response
- **dead_letter_queue** — Failed deliveries after max retries

## Project Structure

```
webhook-delivery-system/
├── cmd/server/             # Application entry point
├── internal/
│   ├── api/                # HTTP handlers and routing
│   │   ├── router.go       # Chi router setup with middleware
│   │   ├── events.go       # Event endpoints
│   │   ├── subscribers.go  # Subscriber CRUD
│   │   ├── health.go       # Health check
│   │   └── response.go     # JSON response helpers
│   ├── config/             # Environment variable configuration
│   ├── domain/             # Domain models (Event, Subscriber, etc.)
│   ├── engine/             # Fan-out engine, delivery logic
│   ├── store/              # PostgreSQL and Redis data access
│   │   ├── postgres.go     # Connection pool + migrations
│   │   ├── redis.go        # Redis client
│   │   ├── subscriber_store.go
│   │   └── event_store.go
│   ├── websocket/          # WebSocket hub for dashboard
│   └── worker/             # Worker pool and dispatcher
├── migrations/             # Versioned SQL migration files
├── mock-endpoints/         # Simulated subscriber endpoints
├── dashboard/              # React frontend
├── docker-compose.yml      # PostgreSQL + Redis + API
├── Dockerfile              # Multi-stage build (~20MB image)
└── .github/workflows/      # CI/CD pipeline
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `REDIS_URL` | — | Redis connection string |
| `NUM_WORKERS` | `50` | Number of delivery worker goroutines |

## Author

**Priya More** — [GitHub](https://github.com/Priya8975)

## License

MIT
