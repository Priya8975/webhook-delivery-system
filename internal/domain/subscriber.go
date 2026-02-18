package domain

import (
	"time"
)

type Subscriber struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	EndpointURL        string    `json:"endpoint_url"`
	SecretKey          string    `json:"secret_key,omitempty"`
	IsActive           bool      `json:"is_active"`
	RateLimitPerSecond int       `json:"rate_limit_per_second"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateSubscriberRequest struct {
	Name        string   `json:"name"`
	EndpointURL string   `json:"endpoint_url"`
	EventTypes  []string `json:"event_types"`
}

type UpdateSubscriberRequest struct {
	Name               *string `json:"name,omitempty"`
	EndpointURL        *string `json:"endpoint_url,omitempty"`
	IsActive           *bool   `json:"is_active,omitempty"`
	RateLimitPerSecond *int    `json:"rate_limit_per_second,omitempty"`
}

type CreateSubscriberResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SecretKey string `json:"secret_key"`
}
