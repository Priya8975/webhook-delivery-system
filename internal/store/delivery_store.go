package store

import (
	"context"
	"fmt"

	"github.com/Priya8975/webhook-delivery-system/internal/domain"
	"github.com/jackc/pgx/v5"
)

// DeliveryAttemptRecord holds data for inserting a delivery attempt.
type DeliveryAttemptRecord struct {
	EventID        string
	SubscriberID   string
	AttemptNumber  int
	Status         string
	HTTPStatusCode *int
	ResponseBody   string
	ResponseTimeMs int
	ErrorMessage   string
}

// RecordDeliveryAttempt inserts a delivery attempt into the database.
func (s *PostgresStore) RecordDeliveryAttempt(ctx context.Context, rec DeliveryAttemptRecord) error {
	var statusCode *int
	if rec.HTTPStatusCode != nil {
		statusCode = rec.HTTPStatusCode
	}

	var respBody *string
	if rec.ResponseBody != "" {
		respBody = &rec.ResponseBody
	}

	var errMsg *string
	if rec.ErrorMessage != "" {
		errMsg = &rec.ErrorMessage
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO delivery_attempts (event_id, subscriber_id, attempt_number, status, http_status_code, response_body, response_time_ms, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, rec.EventID, rec.SubscriberID, rec.AttemptNumber, rec.Status, statusCode, respBody, rec.ResponseTimeMs, errMsg)
	if err != nil {
		return fmt.Errorf("inserting delivery attempt: %w", err)
	}
	return nil
}

// ListDeliveryAttempts returns delivery attempts with optional filtering.
func (s *PostgresStore) ListDeliveryAttempts(ctx context.Context, eventID, subscriberID, status string, limit int) ([]domain.DeliveryAttempt, error) {
	query := `SELECT id, event_id, subscriber_id, attempt_number, status, http_status_code, response_body, response_time_ms, error_message, created_at FROM delivery_attempts`
	args := []interface{}{}
	argIdx := 1
	conditions := []string{}

	if eventID != "" {
		conditions = append(conditions, fmt.Sprintf("event_id = $%d", argIdx))
		args = append(args, eventID)
		argIdx++
	}
	if subscriberID != "" {
		conditions = append(conditions, fmt.Sprintf("subscriber_id = $%d", argIdx))
		args = append(args, subscriberID)
		argIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, c := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []domain.DeliveryAttempt
	for rows.Next() {
		var a domain.DeliveryAttempt
		err := rows.Scan(
			&a.ID, &a.EventID, &a.SubscriberID, &a.AttemptNumber,
			&a.Status, &a.HTTPStatusCode, &a.ResponseBody,
			&a.ResponseTimeMs, &a.ErrorMessage, &a.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning delivery attempt: %w", err)
		}
		attempts = append(attempts, a)
	}

	if attempts == nil {
		attempts = []domain.DeliveryAttempt{}
	}

	return attempts, nil
}

// GetDeliveryAttempt returns a single delivery attempt by ID.
func (s *PostgresStore) GetDeliveryAttempt(ctx context.Context, id string) (*domain.DeliveryAttempt, error) {
	var a domain.DeliveryAttempt
	err := s.pool.QueryRow(ctx, `
		SELECT id, event_id, subscriber_id, attempt_number, status, http_status_code, response_body, response_time_ms, error_message, created_at
		FROM delivery_attempts WHERE id = $1
	`, id).Scan(
		&a.ID, &a.EventID, &a.SubscriberID, &a.AttemptNumber,
		&a.Status, &a.HTTPStatusCode, &a.ResponseBody,
		&a.ResponseTimeMs, &a.ErrorMessage, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying delivery attempt: %w", err)
	}
	return &a, nil
}
