package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Priya8975/webhook-delivery-system/internal/domain"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/redis/go-redis/v9"
)

const DeliveryQueueKey = "delivery_queue"

// DeliveryJob represents a single webhook delivery task queued in Redis.
type DeliveryJob struct {
	EventID      string          `json:"event_id"`
	SubscriberID string          `json:"subscriber_id"`
	EndpointURL  string          `json:"endpoint_url"`
	Payload      json.RawMessage `json:"payload"`
	SecretKey    string          `json:"secret_key"`
	EventType    string          `json:"event_type"`
	Attempt      int             `json:"attempt"`
	MaxRetries   int             `json:"max_retries"`
}

// FanOutEngine distributes events to matching subscribers via Redis queue.
type FanOutEngine struct {
	pgStore    *store.PostgresStore
	redisStore *store.RedisStore
	logger     *slog.Logger
}

func NewFanOutEngine(pg *store.PostgresStore, rs *store.RedisStore, logger *slog.Logger) *FanOutEngine {
	return &FanOutEngine{
		pgStore:    pg,
		redisStore: rs,
		logger:     logger,
	}
}

// FanOut finds all matching subscribers for an event and queues delivery jobs.
// Returns the number of deliveries queued.
func (f *FanOutEngine) FanOut(ctx context.Context, event *domain.Event) (int, error) {
	subscribers, err := f.pgStore.FindMatchingSubscribers(ctx, event.EventType)
	if err != nil {
		return 0, fmt.Errorf("finding matching subscribers: %w", err)
	}

	if len(subscribers) == 0 {
		f.logger.Info("no matching subscribers", "event_id", event.ID, "event_type", event.EventType)
		return 0, nil
	}

	// Use Redis pipeline to batch-insert all delivery jobs
	pipe := f.redisStore.Client().Pipeline()

	for _, sub := range subscribers {
		job := DeliveryJob{
			EventID:      event.ID,
			SubscriberID: sub.ID,
			EndpointURL:  sub.EndpointURL,
			Payload:      event.Payload,
			SecretKey:    sub.SecretKey,
			EventType:    event.EventType,
			Attempt:      1,
			MaxRetries:   5,
		}

		jobBytes, err := json.Marshal(job)
		if err != nil {
			f.logger.Error("failed to marshal job", "error", err, "subscriber_id", sub.ID)
			continue
		}

		pipe.ZAdd(ctx, DeliveryQueueKey, redis.Z{
			Score:  float64(time.Now().UnixMicro()),
			Member: string(jobBytes),
		})
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("queuing deliveries to redis: %w", err)
	}

	f.logger.Info("fan-out complete",
		"event_id", event.ID,
		"event_type", event.EventType,
		"deliveries_queued", len(subscribers),
	)

	return len(subscribers), nil
}

// QueueDepth returns the current number of jobs waiting in the delivery queue.
func (f *FanOutEngine) QueueDepth(ctx context.Context) (int64, error) {
	return f.redisStore.Client().ZCard(ctx, DeliveryQueueKey).Result()
}
