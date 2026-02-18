package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	ws "github.com/Priya8975/webhook-delivery-system/internal/websocket"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupDeliveryTest creates a deliverer with miniredis (no Postgres — just tests HTTP delivery logic).
// Returns a deliverer without pgStore since we're testing HTTP mechanics, not DB recording.
func setupDeliveryTest(t *testing.T) (*redis.Client, *engine.CircuitBreaker, *engine.RateLimiter, *ws.Hub, *slog.Logger) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cb := engine.NewCircuitBreaker(client, logger)
	rl := engine.NewRateLimiter(client, logger)
	hub := ws.NewHub(logger)
	go hub.Run()

	return client, cb, rl, hub, logger
}

func TestDelivery_SuccessfulEndpoint(t *testing.T) {
	var receivedCount atomic.Int32
	var receivedHeaders http.Header

	// Create a test HTTP server that always succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCount.Add(1)
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	_, cb, rl, hub, logger := setupDeliveryTest(t)

	deliverer := &Deliverer{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		redisClient:    redis.NewClient(&redis.Options{Addr: "localhost:0"}), // not used for this test
		circuitBreaker: cb,
		rateLimiter:    rl,
		hub:            hub,
		logger:         logger,
	}

	job := engine.DeliveryJob{
		EventID:            "evt-test-1",
		SubscriberID:       "sub-test-1",
		EndpointURL:        server.URL,
		Payload:            json.RawMessage(`{"test":true}`),
		SecretKey:          "test-secret",
		EventType:          "test.event",
		Attempt:            1,
		MaxRetries:         5,
		RateLimitPerSecond: 0,
	}

	// Deliver (will fail on DB recording but HTTP delivery should succeed)
	deliverer.Deliver(context.Background(), job)

	if receivedCount.Load() != 1 {
		t.Errorf("expected 1 request to endpoint, got %d", receivedCount.Load())
	}

	// Check webhook headers were set correctly
	if receivedHeaders.Get("X-Webhook-Event") != "test.event" {
		t.Errorf("X-Webhook-Event = %q, want %q", receivedHeaders.Get("X-Webhook-Event"), "test.event")
	}
	if receivedHeaders.Get("X-Webhook-ID") != "evt-test-1" {
		t.Errorf("X-Webhook-ID = %q, want %q", receivedHeaders.Get("X-Webhook-ID"), "evt-test-1")
	}
	if receivedHeaders.Get("X-Webhook-Attempt") != "1" {
		t.Errorf("X-Webhook-Attempt = %q, want %q", receivedHeaders.Get("X-Webhook-Attempt"), "1")
	}
	if receivedHeaders.Get("X-Webhook-Signature") == "" {
		t.Error("X-Webhook-Signature should be set")
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", receivedHeaders.Get("Content-Type"), "application/json")
	}
}

func TestDelivery_SignatureIsValid(t *testing.T) {
	var receivedSig string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Webhook-Signature")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = buf[:n]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, cb, rl, hub, logger := setupDeliveryTest(t)

	payload := json.RawMessage(`{"order_id":"abc-123"}`)
	secret := "my-webhook-secret"

	deliverer := &Deliverer{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		redisClient:    redis.NewClient(&redis.Options{Addr: "localhost:0"}),
		circuitBreaker: cb,
		rateLimiter:    rl,
		hub:            hub,
		logger:         logger,
	}

	deliverer.Deliver(context.Background(), engine.DeliveryJob{
		EventID:      "evt-sig",
		SubscriberID: "sub-sig",
		EndpointURL:  server.URL,
		Payload:      payload,
		SecretKey:    secret,
		EventType:    "test.event",
		Attempt:      1,
		MaxRetries:   5,
	})

	// Verify signature matches what we'd compute
	expectedSig := computeHMAC(receivedBody, secret)
	if receivedSig != expectedSig {
		t.Errorf("signature mismatch:\n  received: %s\n  expected: %s", receivedSig, expectedSig)
	}
}

func TestDelivery_CircuitBreakerBlocks(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, cb, rl, hub, logger := setupDeliveryTest(t)

	// Open the circuit breaker for this subscriber
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		cb.RecordFailure(ctx, "sub-blocked")
	}

	deliverer := &Deliverer{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		redisClient:    client,
		circuitBreaker: cb,
		rateLimiter:    rl,
		hub:            hub,
		logger:         logger,
	}

	deliverer.Deliver(ctx, engine.DeliveryJob{
		EventID:      "evt-blocked",
		SubscriberID: "sub-blocked",
		EndpointURL:  server.URL,
		Payload:      json.RawMessage(`{}`),
		SecretKey:    "secret",
		EventType:    "test.event",
		Attempt:      1,
		MaxRetries:   5,
	})

	// The endpoint should NOT have been called
	if requestCount.Load() != 0 {
		t.Errorf("circuit breaker should block delivery, but %d requests reached the endpoint", requestCount.Load())
	}
}

func TestWorkerPool_ProcessesJobs(t *testing.T) {
	var processed atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		processed.Add(1)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	_, cb, rl, hub, logger := setupDeliveryTest(t)

	deliverer := &Deliverer{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		redisClient:    redis.NewClient(&redis.Options{Addr: "localhost:0"}),
		circuitBreaker: cb,
		rateLimiter:    rl,
		hub:            hub,
		logger:         logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := NewPool(3, deliverer, logger)
	pool.Start(ctx)

	// Submit 5 jobs
	for i := 0; i < 5; i++ {
		pool.Submit(engine.DeliveryJob{
			EventID:      "evt-pool-" + string(rune('a'+i)),
			SubscriberID: "sub-pool",
			EndpointURL:  server.URL,
			Payload:      json.RawMessage(`{"test":true}`),
			SecretKey:    "secret",
			EventType:    "test.event",
			Attempt:      1,
			MaxRetries:   5,
		})
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	cancel()
	pool.Stop()

	if processed.Load() != 5 {
		t.Errorf("expected 5 jobs processed, got %d", processed.Load())
	}
}
