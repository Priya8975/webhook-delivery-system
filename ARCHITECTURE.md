# Architecture & Design Decisions

This document explains the key architectural decisions, tradeoffs, and alternatives considered while building the Webhook Delivery System.

## High-Level Design

The system follows a **producer-consumer pattern** with Redis as the message broker:

1. **API layer** accepts events and subscriber registrations
2. **Fan-out engine** matches events to subscribers and queues delivery jobs
3. **Worker pool** consumes jobs from the queue and delivers via HTTP
4. **Resilience layer** (circuit breaker, rate limiter, retries) handles failures
5. **Dashboard** provides real-time visibility via WebSocket

## Design Decision: Why Redis Sorted Sets for the Queue?

**Chosen:** Redis sorted sets (`ZRANGEBYSCORE` + `ZREM`)

**Alternatives considered:**
- **Redis lists** (`LPUSH`/`BRPOP`) — Simple FIFO queue, but no way to schedule future deliveries for retries
- **RabbitMQ / Kafka** — More powerful but adds operational complexity for a single-service system
- **PostgreSQL-only polling** — Would work but adds load to the database for queue operations

**Why sorted sets win:** Jobs are scored by Unix timestamp in microseconds. When a retry is scheduled for 8 seconds from now, it gets a score 8 seconds in the future. The dispatcher simply asks Redis "give me all jobs with score <= now" — delayed retries naturally appear at the right time without any timer management.

## Design Decision: Worker Pool Architecture

**Chosen:** Fixed-size goroutine pool with a buffered Go channel

```
Dispatcher → Channel (buffered) → N Worker Goroutines → HTTP POST
```

**Why not one goroutine per delivery?** Unbounded goroutine spawning can cause memory issues under high load. A fixed pool of N workers (configurable via `NUM_WORKERS`) provides backpressure — if all workers are busy, the channel blocks and the dispatcher waits. This is a natural flow control mechanism.

**Why a channel, not a mutex-guarded queue?** Channels are Go's idiomatic way to communicate between goroutines. They provide built-in blocking, signaling (close to stop workers), and are safe for concurrent use without explicit locking.

## Design Decision: Circuit Breaker in Redis

**Chosen:** Per-subscriber circuit breaker state stored in Redis hashes

**Why Redis, not in-memory?** If the server restarts, in-memory state is lost. A subscriber whose endpoint is down would get hammered again after restart. Redis-backed state survives restarts and could be shared across multiple server instances in a horizontal scaling scenario.

**State machine:**
- **Closed** (normal): Track failure count. After 5 consecutive failures → Open.
- **Open** (blocking): Reject all deliveries. After 30-second cooldown → Half-Open.
- **Half-Open** (probing): Allow exactly one test request. Success → Closed. Failure → Open.

**Tradeoff:** Redis adds a network round-trip for every circuit breaker check. In practice, this is sub-millisecond on the same network and much cheaper than making an HTTP call to a broken endpoint.

## Design Decision: Rate Limiter with Lua Scripts

**Chosen:** Sliding window rate limiter using a Redis Lua script

**Why Lua, not separate Redis commands?** Rate limiting requires three atomic operations:
1. Remove expired entries from the window
2. Count current entries
3. If under limit, add new entry

Without Lua, a race condition exists between steps 2 and 3 — two workers could both see "under limit" and both add entries, exceeding the limit. The Lua script executes atomically on the Redis server.

**Why sliding window, not fixed window?** Fixed windows have a burst problem at window boundaries — a subscriber could get 2x their limit if they send requests right at the boundary of two windows. Sliding windows distribute the limit evenly over time.

## Design Decision: Retry Strategy

**Chosen:** Exponential backoff with jitter: `delay = 2^attempt + random(0-1s)`

**Why exponential backoff?** If an endpoint is down, retrying every second creates unnecessary load. Exponential backoff gives the target service time to recover while reducing our own resource usage.

**Why jitter?** Without jitter, if 1000 deliveries all fail at the same time, they'd all retry at exactly the same moment (thundering herd). Random jitter spreads retries across time, reducing spike load on both the target endpoint and our own system.

**Why 5 max retries?** Diminishing returns. After 5 retries (spanning ~30 seconds), if the endpoint is still down, it's likely a persistent issue that needs human attention. The dead letter queue captures these for manual review.

## Design Decision: HMAC-SHA256 Signatures

**Chosen:** Sign every delivery payload with HMAC-SHA256 using the subscriber's secret key

**Why HMAC, not JWT or API keys?** HMAC is the industry standard for webhook signatures (used by Stripe, GitHub, Shopify). It's simple: the receiver computes the same HMAC over the received payload using their shared secret, and compares. No token parsing, no key rotation complexity.

**What it protects against:** A malicious actor cannot forge webhook deliveries because they don't have the subscriber's secret key. The receiver can verify that the payload was sent by our system and hasn't been tampered with.

## Design Decision: WebSocket for Real-Time Dashboard

**Chosen:** Server-sent WebSocket events from the delivery engine

**Alternatives considered:**
- **Server-Sent Events (SSE)** — Simpler but one-directional. WebSocket allows bidirectional communication if needed in the future.
- **Polling** — Used for metrics (every 2s) since they're aggregated SQL queries. But for the live delivery feed, polling would miss events between intervals.

**Hub pattern:** A single goroutine manages all WebSocket connections through channels. This avoids mutex complexity — register, unregister, and broadcast all go through channels, so the hub's internal state is only accessed by one goroutine.

## Design Decision: PostgreSQL for Persistent Storage

**Chosen:** PostgreSQL with pgx (pure Go driver)

**Why not MySQL?** PostgreSQL has better JSON support (`JSONB` type) for storing event payloads, and the `FILTER` clause for aggregate queries (used in the metrics endpoint).

**Why pgx, not database/sql?** pgx is the fastest pure-Go PostgreSQL driver. It supports connection pooling natively (`pgxpool`), PostgreSQL-specific features, and avoids the overhead of the `database/sql` abstraction layer.

## Design Decision: Fan-Out at Ingestion Time

**Chosen:** When an event is published, immediately find all matching subscribers and queue individual delivery jobs.

**Alternative:** Store the event and have workers look up subscribers at delivery time.

**Why fan-out at ingestion?** If a subscriber is added after an event was published, they shouldn't receive it (events are point-in-time). Pre-computing the delivery list at ingestion time captures the exact set of subscribers at the moment of the event. It also means workers don't need database access — they get a self-contained job with everything needed for delivery.

## Tradeoffs & Limitations

| Decision | Benefit | Tradeoff |
|----------|---------|----------|
| Redis queue | Fast, supports delayed jobs | Not durable — if Redis crashes, queued jobs are lost |
| Fixed worker pool | Predictable resource usage | May under-utilize resources at low load |
| Per-subscriber circuit breaker | Fine-grained protection | More Redis keys to manage |
| Fan-out at ingestion | Consistent subscriber snapshot | Late subscribers miss past events |
| Single-server design | Simple to deploy and operate | No horizontal scaling without adding a load balancer |

## Future Improvements

- **Redis persistence** (`RDB` or `AOF`) for queue durability
- **Horizontal scaling** with multiple server instances sharing the Redis queue
- **Event replay** endpoint to re-deliver past events to a specific subscriber
- **Webhook verification endpoint** where subscribers can validate their setup
- **Batch delivery** for high-throughput subscribers
