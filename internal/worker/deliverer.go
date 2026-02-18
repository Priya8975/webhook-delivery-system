package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/redis/go-redis/v9"
)

// Deliverer handles the HTTP delivery of webhook payloads to subscriber endpoints.
type Deliverer struct {
	httpClient  *http.Client
	pgStore     *store.PostgresStore
	redisClient *redis.Client
	logger      *slog.Logger
}

// NewDeliverer creates a deliverer with a configured HTTP client.
func NewDeliverer(pgStore *store.PostgresStore, redisClient *redis.Client, logger *slog.Logger) *Deliverer {
	return &Deliverer{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		pgStore:     pgStore,
		redisClient: redisClient,
		logger:      logger,
	}
}

// Deliver sends the webhook payload to the subscriber endpoint via HTTP POST.
// On failure, it either re-queues with exponential backoff or moves to the dead letter queue.
func (d *Deliverer) Deliver(ctx context.Context, job engine.DeliveryJob) {
	start := time.Now()

	// Compute HMAC-SHA256 signature
	signature := computeHMAC(job.Payload, job.SecretKey)

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.EndpointURL, bytes.NewReader(job.Payload))
	if err != nil {
		d.handleFailure(ctx, job, start, nil, "", fmt.Sprintf("failed to create request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", job.EventType)
	req.Header.Set("X-Webhook-ID", job.EventID)
	req.Header.Set("X-Webhook-Attempt", fmt.Sprintf("%d", job.Attempt))

	// Execute the request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.handleFailure(ctx, job, start, nil, "", fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	// Read response body (limit to 1KB to prevent memory issues)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	responseBody := string(body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		d.recordAttempt(ctx, job, start, &resp.StatusCode, responseBody, "", nil)
		d.logger.Info("delivery successful",
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
			"attempt", job.Attempt,
			"status_code", resp.StatusCode,
			"response_time_ms", time.Since(start).Milliseconds(),
		)
	} else {
		d.handleFailure(ctx, job, start, &resp.StatusCode, responseBody, "")
	}
}

// handleFailure processes a failed delivery — either retries or sends to DLQ.
func (d *Deliverer) handleFailure(ctx context.Context, job engine.DeliveryJob, start time.Time, statusCode *int, responseBody string, errMsg string) {
	if job.Attempt < job.MaxRetries {
		// Schedule retry with exponential backoff + jitter
		nextRetry := d.scheduleRetry(ctx, job)
		d.recordAttempt(ctx, job, start, statusCode, responseBody, errMsg, nextRetry)

		d.logger.Warn("delivery failed, scheduling retry",
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
			"attempt", job.Attempt,
			"next_attempt", job.Attempt+1,
			"next_retry_at", nextRetry.Format(time.RFC3339),
			"error", errMsg,
			"status_code", statusCode,
		)
	} else {
		// Max retries exhausted — move to dead letter queue
		d.recordAttempt(ctx, job, start, statusCode, responseBody, errMsg, nil)
		d.moveToDLQ(ctx, job, statusCode, errMsg)

		d.logger.Error("delivery permanently failed, moved to dead letter queue",
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
			"total_attempts", job.Attempt,
			"error", errMsg,
			"status_code", statusCode,
		)
	}
}

// scheduleRetry re-queues the job to Redis with a future timestamp.
// Uses exponential backoff: 2^(attempt-1) seconds + random jitter.
// Attempt 1 fail → retry in ~2s, attempt 2 → ~4s, attempt 3 → ~8s, attempt 4 → ~16s
func (d *Deliverer) scheduleRetry(ctx context.Context, job engine.DeliveryJob) *time.Time {
	baseDelay := time.Duration(math.Pow(2, float64(job.Attempt))) * time.Second
	jitter := time.Duration(rand.IntN(1000)) * time.Millisecond
	delay := baseDelay + jitter

	nextRetry := time.Now().Add(delay)

	retryJob := engine.DeliveryJob{
		EventID:      job.EventID,
		SubscriberID: job.SubscriberID,
		EndpointURL:  job.EndpointURL,
		Payload:      job.Payload,
		SecretKey:    job.SecretKey,
		EventType:    job.EventType,
		Attempt:      job.Attempt + 1,
		MaxRetries:   job.MaxRetries,
	}

	jobBytes, err := json.Marshal(retryJob)
	if err != nil {
		d.logger.Error("failed to marshal retry job", "error", err)
		return &nextRetry
	}

	err = d.redisClient.ZAdd(ctx, engine.DeliveryQueueKey, redis.Z{
		Score:  float64(nextRetry.UnixMicro()),
		Member: string(jobBytes),
	}).Err()
	if err != nil {
		d.logger.Error("failed to queue retry", "error", err)
	}

	return &nextRetry
}

// moveToDLQ inserts the failed delivery into the dead letter queue.
func (d *Deliverer) moveToDLQ(ctx context.Context, job engine.DeliveryJob, statusCode *int, errMsg string) {
	err := d.pgStore.InsertDeadLetter(ctx, store.DeadLetterRecord{
		EventID:        job.EventID,
		SubscriberID:   job.SubscriberID,
		TotalAttempts:  job.Attempt,
		LastHTTPStatus: statusCode,
		LastError:      errMsg,
	})
	if err != nil {
		d.logger.Error("failed to insert into dead letter queue",
			"error", err,
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
		)
	}
}

// recordAttempt logs the delivery result to PostgreSQL.
func (d *Deliverer) recordAttempt(ctx context.Context, job engine.DeliveryJob, start time.Time, statusCode *int, responseBody string, errMsg string, nextRetryAt *time.Time) {
	elapsed := time.Since(start).Milliseconds()

	status := "success"
	if errMsg != "" || (statusCode != nil && *statusCode >= 400) {
		status = "failed"
	}

	err := d.pgStore.RecordDeliveryAttempt(ctx, store.DeliveryAttemptRecord{
		EventID:        job.EventID,
		SubscriberID:   job.SubscriberID,
		AttemptNumber:  job.Attempt,
		Status:         status,
		HTTPStatusCode: statusCode,
		ResponseBody:   responseBody,
		ResponseTimeMs: int(elapsed),
		ErrorMessage:   errMsg,
		NextRetryAt:    nextRetryAt,
	})
	if err != nil {
		d.logger.Error("failed to record delivery attempt",
			"error", err,
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
		)
	}
}

// computeHMAC generates an HMAC-SHA256 signature for the payload.
func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
