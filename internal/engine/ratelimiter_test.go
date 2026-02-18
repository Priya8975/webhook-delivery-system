package engine

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRL(t *testing.T) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	rl := NewRateLimiter(client, logger)
	return rl, mr
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl, _ := setupTestRL(t)
	ctx := context.Background()

	// Limit of 5 per second — first 5 should all be allowed
	for i := 0; i < 5; i++ {
		if !rl.Allow(ctx, "sub-1", 5) {
			t.Errorf("request %d should be allowed (limit=5)", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl, _ := setupTestRL(t)
	ctx := context.Background()

	// Fill up the limit
	for i := 0; i < 3; i++ {
		rl.Allow(ctx, "sub-1", 3)
	}

	// Next request should be blocked
	if rl.Allow(ctx, "sub-1", 3) {
		t.Error("request should be blocked when over limit")
	}
}

func TestRateLimiter_ZeroLimit_AllowsAll(t *testing.T) {
	rl, _ := setupTestRL(t)
	ctx := context.Background()

	// Zero limit means no rate limiting
	for i := 0; i < 100; i++ {
		if !rl.Allow(ctx, "sub-1", 0) {
			t.Errorf("request %d should be allowed with limit=0 (unlimited)", i+1)
		}
	}
}

func TestRateLimiter_IsolationBetweenSubscribers(t *testing.T) {
	rl, _ := setupTestRL(t)
	ctx := context.Background()

	// Fill up sub-1's limit
	for i := 0; i < 2; i++ {
		rl.Allow(ctx, "sub-1", 2)
	}

	// sub-1 should be blocked
	if rl.Allow(ctx, "sub-1", 2) {
		t.Error("sub-1 should be blocked")
	}

	// sub-2 should still be allowed
	if !rl.Allow(ctx, "sub-2", 2) {
		t.Error("sub-2 should be allowed — rate limits are per-subscriber")
	}
}
