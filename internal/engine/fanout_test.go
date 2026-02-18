package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestDeliveryJob_MarshalUnmarshal(t *testing.T) {
	original := DeliveryJob{
		EventID:            "evt-123",
		SubscriberID:       "sub-456",
		EndpointURL:        "http://example.com/webhook",
		Payload:            json.RawMessage(`{"order_id":"abc","amount":42.5}`),
		SecretKey:          "secret-key-xyz",
		EventType:          "order.created",
		Attempt:            1,
		MaxRetries:         5,
		RateLimitPerSecond: 10,
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var decoded DeliveryJob
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify all fields
	if decoded.EventID != original.EventID {
		t.Errorf("EventID: got %q, want %q", decoded.EventID, original.EventID)
	}
	if decoded.SubscriberID != original.SubscriberID {
		t.Errorf("SubscriberID: got %q, want %q", decoded.SubscriberID, original.SubscriberID)
	}
	if decoded.EndpointURL != original.EndpointURL {
		t.Errorf("EndpointURL: got %q, want %q", decoded.EndpointURL, original.EndpointURL)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Errorf("Payload: got %q, want %q", string(decoded.Payload), string(original.Payload))
	}
	if decoded.SecretKey != original.SecretKey {
		t.Errorf("SecretKey: got %q, want %q", decoded.SecretKey, original.SecretKey)
	}
	if decoded.Attempt != original.Attempt {
		t.Errorf("Attempt: got %d, want %d", decoded.Attempt, original.Attempt)
	}
	if decoded.MaxRetries != original.MaxRetries {
		t.Errorf("MaxRetries: got %d, want %d", decoded.MaxRetries, original.MaxRetries)
	}
	if decoded.RateLimitPerSecond != original.RateLimitPerSecond {
		t.Errorf("RateLimitPerSecond: got %d, want %d", decoded.RateLimitPerSecond, original.RateLimitPerSecond)
	}
}

func TestDeliveryJob_RetryIncrement(t *testing.T) {
	job := DeliveryJob{
		EventID:    "evt-1",
		Attempt:    1,
		MaxRetries: 5,
	}

	// Simulate retry: increment attempt
	retryJob := DeliveryJob{
		EventID:    job.EventID,
		Attempt:    job.Attempt + 1,
		MaxRetries: job.MaxRetries,
	}

	if retryJob.Attempt != 2 {
		t.Errorf("retry attempt should be 2, got %d", retryJob.Attempt)
	}

	if retryJob.Attempt >= retryJob.MaxRetries {
		t.Error("attempt 2 should be below max retries 5")
	}
}

func TestQueueDepth_Empty(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()
	depth, err := client.ZCard(ctx, DeliveryQueueKey).Result()
	if err != nil {
		t.Fatalf("failed to get queue depth: %v", err)
	}

	if depth != 0 {
		t.Errorf("expected empty queue, got depth %d", depth)
	}
}

func TestQueueDepth_AfterAddingJobs(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()

	// Add 3 jobs to the queue
	for i := 0; i < 3; i++ {
		job := DeliveryJob{EventID: "evt-" + string(rune('a'+i))}
		data, _ := json.Marshal(job)
		client.ZAdd(ctx, DeliveryQueueKey, redis.Z{
			Score:  float64(i),
			Member: string(data),
		})
	}

	depth, err := client.ZCard(ctx, DeliveryQueueKey).Result()
	if err != nil {
		t.Fatalf("failed to get queue depth: %v", err)
	}

	if depth != 3 {
		t.Errorf("expected queue depth 3, got %d", depth)
	}
}

func TestDeliveryQueueKey_Constant(t *testing.T) {
	if DeliveryQueueKey != "delivery_queue" {
		t.Errorf("expected DeliveryQueueKey = %q, got %q", "delivery_queue", DeliveryQueueKey)
	}
}
