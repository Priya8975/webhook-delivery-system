# Webhook Delivery System

A fault-tolerant webhook delivery platform built in Go that reliably delivers event notifications to subscriber endpoints. Implements industry-standard patterns used by Stripe, GitHub, and Shopify for their webhook infrastructure.

## Features

- **Event Fan-out** — Publish events and automatically deliver to all matching subscribers
- **Guaranteed Delivery** — At-least-once delivery with exponential backoff retries
- **Circuit Breaker** — Per-endpoint circuit breakers prevent cascading failures
- **Rate Limiting** — Per-subscriber sliding window rate limiting with Redis
- **Dead Letter Queue** — Failed deliveries are captured for manual review and replay
- **HMAC Signatures** — Every delivery is signed with HMAC-SHA256 for verification
- **Real-time Dashboard** — Live WebSocket feed of delivery events and system health
- **Worker Pool** — Goroutine-based concurrent delivery engine

## Architecture

```
┌──────────────────────────────────────────────────┐
│              Dashboard (React + WebSocket)        │
└──────────────────────┬───────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────┐
│              API Server (Go + Chi)                │
│  POST /events  POST /subscribers  GET /deliveries │
└────────┬──────────────────────────┬──────────────┘
         │                          │
┌────────▼────────┐      ┌─────────▼─────────┐
│   PostgreSQL    │      │     Redis          │
│  Events, Logs,  │      │  Delivery Queue,   │
│  Subscribers    │      │  Circuit Breaker,  │
│                 │      │  Rate Limits       │
└─────────────────┘      └─────────┬─────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │    Worker Pool (Goroutines)  │
                    │  Dispatcher → Channels →     │
                    │  Workers → HTTP POST         │
                    └──────────────────────────────┘
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| API Server | Go, Chi router |
| Database | PostgreSQL (pgx driver) |
| Queue & Cache | Redis |
| Workers | Goroutines + Channels |
| Dashboard | React, Tailwind CSS |
| Infrastructure | Docker, Docker Compose |
| CI/CD | GitHub Actions |

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Node.js 20+ (for dashboard)

### Run with Docker

```bash
docker compose up
```

The API server will be available at `http://localhost:8080`.

### Run Locally (Development)

```bash
# Start PostgreSQL and Redis
docker compose up postgres redis -d

# Set environment variables
export DATABASE_URL=postgresql://webhook:webhook@localhost:5432/webhook?sslmode=disable
export REDIS_URL=redis://localhost:6379/0

# Run the server
go run ./cmd/server
```

### API Examples

**Create a subscriber:**
```bash
curl -X POST http://localhost:8080/api/v1/subscribers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "endpoint_url": "https://acme.com/webhooks",
    "event_types": ["payment.completed", "order.*"]
  }'
```

**Publish an event:**
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

**Check system health:**
```bash
curl http://localhost:8080/api/v1/health
```

## Project Structure

```
webhook-delivery-system/
├── cmd/server/          # Application entry point
├── internal/
│   ├── api/             # HTTP handlers and routing
│   ├── config/          # Configuration management
│   ├── domain/          # Domain models
│   ├── engine/          # Delivery engine (circuit breaker, retry, rate limiter)
│   ├── store/           # Database and Redis access
│   ├── websocket/       # WebSocket hub for dashboard
│   └── worker/          # Worker pool and dispatcher
├── migrations/          # SQL migration files
├── mock-endpoints/      # Simulated subscriber endpoints
├── dashboard/           # React frontend
├── docker-compose.yml
└── Dockerfile
```

## Design Decisions

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed design decisions and tradeoffs.

## Author

**Priya More** — [GitHub](https://github.com/Priya8975)

## License

MIT
