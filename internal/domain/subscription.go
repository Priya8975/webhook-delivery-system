package domain

import "time"

type Subscription struct {
	ID           string    `json:"id"`
	SubscriberID string    `json:"subscriber_id"`
	EventType    string    `json:"event_type"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}
