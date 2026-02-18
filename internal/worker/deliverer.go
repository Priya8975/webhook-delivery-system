package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
)

// Deliverer handles the HTTP delivery of webhook payloads to subscriber endpoints.
type Deliverer struct {
	httpClient *http.Client
	pgStore    *store.PostgresStore
	logger     *slog.Logger
}

// NewDeliverer creates a deliverer with a configured HTTP client.
func NewDeliverer(pgStore *store.PostgresStore, logger *slog.Logger) *Deliverer {
	return &Deliverer{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		pgStore: pgStore,
		logger:  logger,
	}
}

// Deliver sends the webhook payload to the subscriber endpoint via HTTP POST.
// It signs the payload with HMAC-SHA256 and logs the delivery attempt.
func (d *Deliverer) Deliver(ctx context.Context, job engine.DeliveryJob) {
	start := time.Now()

	// Compute HMAC-SHA256 signature
	signature := computeHMAC(job.Payload, job.SecretKey)

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.EndpointURL, bytes.NewReader(job.Payload))
	if err != nil {
		d.recordAttempt(ctx, job, start, nil, "", fmt.Sprintf("failed to create request: %v", err))
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
		d.recordAttempt(ctx, job, start, nil, "", fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	// Read response body (limit to 1KB to prevent memory issues)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	responseBody := string(body)

	d.recordAttempt(ctx, job, start, &resp.StatusCode, responseBody, "")
}

// recordAttempt logs the delivery result to PostgreSQL.
func (d *Deliverer) recordAttempt(ctx context.Context, job engine.DeliveryJob, start time.Time, statusCode *int, responseBody string, errMsg string) {
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
	})
	if err != nil {
		d.logger.Error("failed to record delivery attempt",
			"error", err,
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
		)
	}

	if status == "success" {
		d.logger.Info("delivery successful",
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
			"attempt", job.Attempt,
			"status_code", statusCode,
			"response_time_ms", elapsed,
		)
	} else {
		d.logger.Warn("delivery failed",
			"event_id", job.EventID,
			"subscriber_id", job.SubscriberID,
			"attempt", job.Attempt,
			"error", errMsg,
			"status_code", statusCode,
			"response_time_ms", elapsed,
		)
	}
}

// computeHMAC generates an HMAC-SHA256 signature for the payload.
func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
