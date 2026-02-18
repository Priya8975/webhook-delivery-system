package domain

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Source    string          `json:"source,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}
