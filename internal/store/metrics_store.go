package store

import (
	"context"
	"fmt"
)

// DeliveryMetrics holds aggregated delivery statistics.
type DeliveryMetrics struct {
	TotalDeliveries  int     `json:"total_deliveries"`
	SuccessCount     int     `json:"success_count"`
	FailedCount      int     `json:"failed_count"`
	SuccessRate      float64 `json:"success_rate"`
	AvgResponseMs    float64 `json:"avg_response_ms"`
	DeadLetterCount  int     `json:"dead_letter_count"`
	ActiveSubscribers int    `json:"active_subscribers"`
	TotalEvents      int     `json:"total_events"`
}

// GetDeliveryMetrics returns aggregated delivery statistics from the database.
func (s *PostgresStore) GetDeliveryMetrics(ctx context.Context) (*DeliveryMetrics, error) {
	var m DeliveryMetrics

	// Delivery counts and average response time
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'success') AS success,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COALESCE(AVG(response_time_ms) FILTER (WHERE response_time_ms > 0), 0) AS avg_response_ms
		FROM delivery_attempts
	`).Scan(&m.TotalDeliveries, &m.SuccessCount, &m.FailedCount, &m.AvgResponseMs)
	if err != nil {
		return nil, fmt.Errorf("querying delivery metrics: %w", err)
	}

	if m.TotalDeliveries > 0 {
		m.SuccessRate = float64(m.SuccessCount) / float64(m.TotalDeliveries) * 100
	}

	// Unresolved dead letters
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM dead_letter_queue WHERE resolved_at IS NULL
	`).Scan(&m.DeadLetterCount)
	if err != nil {
		return nil, fmt.Errorf("querying dead letter count: %w", err)
	}

	// Active subscribers
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM subscribers WHERE is_active = true
	`).Scan(&m.ActiveSubscribers)
	if err != nil {
		return nil, fmt.Errorf("querying active subscribers: %w", err)
	}

	// Total events
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM events
	`).Scan(&m.TotalEvents)
	if err != nil {
		return nil, fmt.Errorf("querying total events: %w", err)
	}

	return &m, nil
}
