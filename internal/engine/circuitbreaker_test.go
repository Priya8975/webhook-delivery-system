package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestCB(t *testing.T) (*CircuitBreaker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cb := NewCircuitBreaker(client, logger)
	return cb, mr
}

// openCircuitAndExpireCooldown opens the circuit for a subscriber, then
// sets last_failed_at to 31 seconds ago so the cooldown has elapsed.
func openCircuitAndExpireCooldown(t *testing.T, cb *CircuitBreaker, mr *miniredis.Miniredis, subID string) {
	t.Helper()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		cb.RecordFailure(ctx, subID)
	}

	// Set last_failed_at to 31 seconds ago (past the 30s cooldown)
	pastTime := time.Now().Unix() - 31
	mr.HSet(cbKey(subID), "last_failed_at", fmt.Sprintf("%d", pastTime))
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb, _ := setupTestCB(t)
	ctx := context.Background()

	state, allowed := cb.AllowRequest(ctx, "sub-1")

	if state != StateClosed {
		t.Errorf("expected state %q, got %q", StateClosed, state)
	}
	if !allowed {
		t.Error("new subscriber should be allowed (circuit closed)")
	}
}

func TestCircuitBreaker_GetState_Default(t *testing.T) {
	cb, _ := setupTestCB(t)
	ctx := context.Background()

	state := cb.GetState(ctx, "unknown-sub")

	if state.State != StateClosed {
		t.Errorf("expected state %q, got %q", StateClosed, state.State)
	}
	if state.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", state.Failures)
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb, _ := setupTestCB(t)
	ctx := context.Background()

	// Record 5 failures (threshold)
	for i := 0; i < 5; i++ {
		cb.RecordFailure(ctx, "sub-1")
	}

	state, allowed := cb.AllowRequest(ctx, "sub-1")

	if state != StateOpen {
		t.Errorf("expected state %q, got %q", StateOpen, state)
	}
	if allowed {
		t.Error("should NOT be allowed when circuit is open")
	}
}

func TestCircuitBreaker_StaysClosedBelowThreshold(t *testing.T) {
	cb, _ := setupTestCB(t)
	ctx := context.Background()

	// Record 4 failures (below threshold of 5)
	for i := 0; i < 4; i++ {
		cb.RecordFailure(ctx, "sub-1")
	}

	state, allowed := cb.AllowRequest(ctx, "sub-1")

	if state != StateClosed {
		t.Errorf("expected state %q, got %q", StateClosed, state)
	}
	if !allowed {
		t.Error("should be allowed when below threshold")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb, _ := setupTestCB(t)
	ctx := context.Background()

	// Record 4 failures then a success
	for i := 0; i < 4; i++ {
		cb.RecordFailure(ctx, "sub-1")
	}
	cb.RecordSuccess(ctx, "sub-1")

	cbState := cb.GetState(ctx, "sub-1")

	if cbState.State != StateClosed {
		t.Errorf("expected state %q after success, got %q", StateClosed, cbState.State)
	}
	if cbState.Failures != 0 {
		t.Errorf("expected 0 failures after success, got %d", cbState.Failures)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb, mr := setupTestCB(t)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 5; i++ {
		cb.RecordFailure(ctx, "sub-1")
	}

	// Verify it's open
	state, allowed := cb.AllowRequest(ctx, "sub-1")
	if state != StateOpen || allowed {
		t.Fatal("circuit should be open and blocking")
	}

	// Set last_failed_at to 31 seconds ago (past the 30s cooldown)
	pastTime := time.Now().Unix() - 31
	mr.HSet(cbKey("sub-1"), "last_failed_at", fmt.Sprintf("%d", pastTime))

	// Now it should transition to half-open and allow one request
	state, allowed = cb.AllowRequest(ctx, "sub-1")
	if state != StateHalfOpen {
		t.Errorf("expected state %q, got %q", StateHalfOpen, state)
	}
	if !allowed {
		t.Error("should allow one request in half-open state")
	}
}

func TestCircuitBreaker_HalfOpenSuccess_ClosesCircuit(t *testing.T) {
	cb, mr := setupTestCB(t)
	ctx := context.Background()

	// Open circuit and expire cooldown
	openCircuitAndExpireCooldown(t, cb, mr, "sub-1")
	cb.AllowRequest(ctx, "sub-1") // triggers half-open transition

	// Success in half-open → closed
	cb.RecordSuccess(ctx, "sub-1")

	state := cb.GetState(ctx, "sub-1")
	if state.State != StateClosed {
		t.Errorf("expected %q after half-open success, got %q", StateClosed, state.State)
	}
}

func TestCircuitBreaker_HalfOpenFailure_ReopensCircuit(t *testing.T) {
	cb, mr := setupTestCB(t)
	ctx := context.Background()

	// Open circuit and expire cooldown
	openCircuitAndExpireCooldown(t, cb, mr, "sub-1")
	cb.AllowRequest(ctx, "sub-1") // triggers half-open transition

	// Failure in half-open → back to open
	cb.RecordFailure(ctx, "sub-1")

	state, allowed := cb.AllowRequest(ctx, "sub-1")
	if state != StateOpen {
		t.Errorf("expected %q after half-open failure, got %q", StateOpen, state)
	}
	if allowed {
		t.Error("should NOT be allowed after half-open failure")
	}
}

func TestCircuitBreaker_IsolationBetweenSubscribers(t *testing.T) {
	cb, _ := setupTestCB(t)
	ctx := context.Background()

	// Open circuit for sub-1
	for i := 0; i < 5; i++ {
		cb.RecordFailure(ctx, "sub-1")
	}

	// sub-2 should still be allowed
	state, allowed := cb.AllowRequest(ctx, "sub-2")
	if state != StateClosed {
		t.Errorf("sub-2 should be closed, got %q", state)
	}
	if !allowed {
		t.Error("sub-2 should be allowed — circuit breakers are per-subscriber")
	}
}
