package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/redis/go-redis/v9"
)

// Dispatcher continuously polls the Redis delivery queue and sends jobs
// to the worker pool via channels.
type Dispatcher struct {
	redisClient  *redis.Client
	pool         *Pool
	logger       *slog.Logger
	pollInterval time.Duration
	batchSize    int64
}

// NewDispatcher creates a dispatcher that pulls from the Redis sorted set.
func NewDispatcher(redisClient *redis.Client, pool *Pool, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		redisClient:  redisClient,
		pool:         pool,
		logger:       logger,
		pollInterval: 100 * time.Millisecond,
		batchSize:    10,
	}
}

// Start begins the polling loop. It runs until the context is cancelled.
func (d *Dispatcher) Start(ctx context.Context) {
	d.logger.Info("dispatcher started")

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("dispatcher stopping")
			return
		case <-ticker.C:
			d.poll(ctx)
		}
	}
}

// poll fetches a batch of ready jobs from Redis and sends them to workers.
func (d *Dispatcher) poll(ctx context.Context) {
	now := float64(time.Now().UnixMicro())

	// Fetch jobs with score <= now (ready for delivery)
	results, err := d.redisClient.ZRangeByScoreWithScores(ctx, engine.DeliveryQueueKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   formatFloat(now),
		Count: d.batchSize,
	}).Result()
	if err != nil {
		d.logger.Error("failed to poll delivery queue", "error", err)
		return
	}

	if len(results) == 0 {
		return
	}

	// Remove fetched jobs atomically and dispatch them
	for _, z := range results {
		member := z.Member.(string)

		// Remove from queue — if another worker already took it, ZRem returns 0
		removed, err := d.redisClient.ZRem(ctx, engine.DeliveryQueueKey, member).Result()
		if err != nil {
			d.logger.Error("failed to remove job from queue", "error", err)
			continue
		}
		if removed == 0 {
			// Another dispatcher instance already claimed this job
			continue
		}

		var job engine.DeliveryJob
		if err := json.Unmarshal([]byte(member), &job); err != nil {
			d.logger.Error("failed to unmarshal job", "error", err)
			continue
		}

		d.pool.Submit(job)
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
