package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a per-subscriber sliding window rate limiter using Redis.
// Uses a sorted set where each member is a unique request ID with a timestamp score.
// A Lua script atomically cleans expired entries, checks the count, and adds new entries.
type RateLimiter struct {
	redisClient *redis.Client
	logger      *slog.Logger
	script      *redis.Script
}

// Lua script for atomic sliding window rate limiting.
// 1. Remove entries older than the window
// 2. Count remaining entries
// 3. If under the limit, add a new entry and return 1 (allowed)
// 4. If at/over the limit, return 0 (denied)
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

-- Remove entries outside the sliding window
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

-- Count current entries in the window
local count = redis.call('ZCARD', key)

if count < limit then
    -- Under the limit: add this request and allow
    redis.call('ZADD', key, now, member)
    -- Set TTL so the key auto-expires after the window
    redis.call('EXPIRE', key, window / 1000 + 1)
    return 1
else
    -- At the limit: deny
    return 0
end
`)

func NewRateLimiter(redisClient *redis.Client, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		logger:      logger,
		script:      slidingWindowScript,
	}
}

func rlKey(subscriberID string) string {
	return fmt.Sprintf("rl:%s", subscriberID)
}

// Allow checks if a delivery to this subscriber is within the rate limit.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(ctx context.Context, subscriberID string, limit int) bool {
	if limit <= 0 {
		return true // No rate limit configured
	}

	key := rlKey(subscriberID)
	now := time.Now().UnixMilli()
	window := int64(1000) // 1 second window in milliseconds
	member := fmt.Sprintf("%d:%d", now, time.Now().UnixNano()%10000) // unique member

	result, err := rl.script.Run(ctx, rl.redisClient, []string{key},
		now, window, limit, member,
	).Int64()
	if err != nil {
		rl.logger.Error("rate limiter script failed", "error", err, "subscriber_id", subscriberID)
		return true // Fail open — allow the request if Redis fails
	}

	if result == 0 {
		rl.logger.Debug("rate limited",
			"subscriber_id", subscriberID,
			"limit", limit,
		)
		return false
	}

	return true
}
