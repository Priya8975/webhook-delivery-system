package domain

import (
	"time"
)

type DeliveryAttempt struct {
	ID             string     `json:"id"`
	EventID        string     `json:"event_id"`
	SubscriberID   string     `json:"subscriber_id"`
	AttemptNumber  int        `json:"attempt_number"`
	Status         string     `json:"status"`
	HTTPStatusCode *int       `json:"http_status_code,omitempty"`
	ResponseBody   *string    `json:"response_body,omitempty"`
	ResponseTimeMs *int       `json:"response_time_ms,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DeadLetter struct {
	ID             string     `json:"id"`
	EventID        string     `json:"event_id"`
	SubscriberID   string     `json:"subscriber_id"`
	TotalAttempts  int        `json:"total_attempts"`
	LastError      *string    `json:"last_error,omitempty"`
	LastHTTPStatus *int       `json:"last_http_status,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy     *string    `json:"resolved_by,omitempty"`
}
