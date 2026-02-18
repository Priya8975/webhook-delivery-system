package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Circuit breaker states
const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half-open"
)

// CircuitBreaker implements a per-subscriber circuit breaker using Redis.
// State transitions: closed → open → half-open → closed
//
// - Closed: Normal operation. Failures are counted.
// - Open: All deliveries are rejected. Transitions to half-open after cooldown.
// - Half-Open: One test delivery is allowed. Success → closed, failure → open.
type CircuitBreaker struct {
	redisClient      *redis.Client
	logger           *slog.Logger
	failureThreshold int
	cooldownPeriod   time.Duration
}

// CircuitBreakerState represents the current state of a subscriber's circuit.
type CircuitBreakerState struct {
	State        string `json:"state"`
	Failures     int    `json:"failures"`
	LastFailedAt string `json:"last_failed_at,omitempty"`
}

func NewCircuitBreaker(redisClient *redis.Client, logger *slog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		redisClient:      redisClient,
		logger:           logger,
		failureThreshold: 5,
		cooldownPeriod:   30 * time.Second,
	}
}

func cbKey(subscriberID string) string {
	return fmt.Sprintf("cb:%s", subscriberID)
}

// AllowRequest checks if a delivery to this subscriber is allowed.
// Returns the current state and whether the request should proceed.
func (cb *CircuitBreaker) AllowRequest(ctx context.Context, subscriberID string) (string, bool) {
	key := cbKey(subscriberID)

	data, err := cb.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		// No state yet — circuit is closed (default)
		return StateClosed, true
	}

	state := data["state"]
	failures, _ := strconv.Atoi(data["failures"])
	lastFailedAt, _ := strconv.ParseInt(data["last_failed_at"], 10, 64)

	switch state {
	case StateOpen:
		// Check if cooldown period has elapsed
		if time.Now().Unix()-lastFailedAt >= int64(cb.cooldownPeriod.Seconds()) {
			// Transition to half-open: allow one test request
			cb.redisClient.HSet(ctx, key, "state", StateHalfOpen)
			cb.logger.Info("circuit breaker half-open",
				"subscriber_id", subscriberID,
			)
			return StateHalfOpen, true
		}
		return StateOpen, false

	case StateHalfOpen:
		// Only one request at a time in half-open
		return StateHalfOpen, true

	default: // StateClosed
		_ = failures
		return StateClosed, true
	}
}

// RecordSuccess records a successful delivery. Resets the circuit to closed.
func (cb *CircuitBreaker) RecordSuccess(ctx context.Context, subscriberID string) {
	key := cbKey(subscriberID)

	state, _ := cb.redisClient.HGet(ctx, key, "state").Result()

	cb.redisClient.HSet(ctx, key,
		"state", StateClosed,
		"failures", 0,
	)

	if state == StateHalfOpen {
		cb.logger.Info("circuit breaker closed (recovered)",
			"subscriber_id", subscriberID,
		)
	}
}

// RecordFailure records a failed delivery. Opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure(ctx context.Context, subscriberID string) {
	key := cbKey(subscriberID)

	// Increment failure count atomically
	failures, err := cb.redisClient.HIncrBy(ctx, key, "failures", 1).Result()
	if err != nil {
		cb.logger.Error("failed to record circuit breaker failure", "error", err)
		return
	}

	cb.redisClient.HSet(ctx, key, "last_failed_at", time.Now().Unix())

	state, _ := cb.redisClient.HGet(ctx, key, "state").Result()

	if state == StateHalfOpen {
		// Half-open test failed → back to open
		cb.redisClient.HSet(ctx, key, "state", StateOpen)
		cb.logger.Warn("circuit breaker re-opened (half-open test failed)",
			"subscriber_id", subscriberID,
		)
	} else if failures >= int64(cb.failureThreshold) {
		// Threshold reached → open the circuit
		cb.redisClient.HSet(ctx, key, "state", StateOpen)
		cb.logger.Warn("circuit breaker opened",
			"subscriber_id", subscriberID,
			"failures", failures,
			"threshold", cb.failureThreshold,
		)
	} else {
		// Ensure state is set to closed if not already set
		if state == "" {
			cb.redisClient.HSet(ctx, key, "state", StateClosed)
		}
	}
}

// GetState returns the current circuit breaker state for a subscriber.
func (cb *CircuitBreaker) GetState(ctx context.Context, subscriberID string) CircuitBreakerState {
	key := cbKey(subscriberID)

	data, err := cb.redisClient.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		return CircuitBreakerState{State: StateClosed, Failures: 0}
	}

	failures, _ := strconv.Atoi(data["failures"])
	state := data["state"]
	if state == "" {
		state = StateClosed
	}

	// Check if open circuit should transition to half-open
	if state == StateOpen {
		lastFailedAt, _ := strconv.ParseInt(data["last_failed_at"], 10, 64)
		if time.Now().Unix()-lastFailedAt >= int64(cb.cooldownPeriod.Seconds()) {
			state = StateHalfOpen
		}
	}

	result := CircuitBreakerState{
		State:    state,
		Failures: failures,
	}

	if ts, ok := data["last_failed_at"]; ok && ts != "" {
		lastFailed, _ := strconv.ParseInt(ts, 10, 64)
		if lastFailed > 0 {
			result.LastFailedAt = time.Unix(lastFailed, 0).Format(time.RFC3339)
		}
	}

	return result
}
