package store

import (
	"context"
	"fmt"
	"time"

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
	NextRetryAt    *time.Time
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
		INSERT INTO delivery_attempts (event_id, subscriber_id, attempt_number, status, http_status_code, response_body, response_time_ms, error_message, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rec.EventID, rec.SubscriberID, rec.AttemptNumber, rec.Status, statusCode, respBody, rec.ResponseTimeMs, errMsg, rec.NextRetryAt)
	if err != nil {
		return fmt.Errorf("inserting delivery attempt: %w", err)
	}
	return nil
}

// DeadLetterRecord holds data for inserting a dead letter entry.
type DeadLetterRecord struct {
	EventID        string
	SubscriberID   string
	TotalAttempts  int
	LastHTTPStatus *int
	LastError      string
}

// InsertDeadLetter adds a permanently failed delivery to the dead letter queue.
func (s *PostgresStore) InsertDeadLetter(ctx context.Context, rec DeadLetterRecord) error {
	var lastErr *string
	if rec.LastError != "" {
		lastErr = &rec.LastError
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO dead_letter_queue (event_id, subscriber_id, total_attempts, last_http_status, last_error)
		VALUES ($1, $2, $3, $4, $5)
	`, rec.EventID, rec.SubscriberID, rec.TotalAttempts, rec.LastHTTPStatus, lastErr)
	if err != nil {
		return fmt.Errorf("inserting dead letter: %w", err)
	}
	return nil
}

// ListDeadLetters returns dead letter entries with optional filtering.
func (s *PostgresStore) ListDeadLetters(ctx context.Context, subscriberID string, resolved bool, limit int) ([]domain.DeadLetter, error) {
	query := `SELECT id, event_id, subscriber_id, total_attempts, last_error, last_http_status, created_at, resolved_at, resolved_by FROM dead_letter_queue`
	args := []interface{}{}
	argIdx := 1
	conditions := []string{}

	if subscriberID != "" {
		conditions = append(conditions, fmt.Sprintf("subscriber_id = $%d", argIdx))
		args = append(args, subscriberID)
		argIdx++
	}

	if resolved {
		conditions = append(conditions, "resolved_at IS NOT NULL")
	} else {
		conditions = append(conditions, "resolved_at IS NULL")
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
		return nil, fmt.Errorf("querying dead letters: %w", err)
	}
	defer rows.Close()

	var letters []domain.DeadLetter
	for rows.Next() {
		var dl domain.DeadLetter
		err := rows.Scan(
			&dl.ID, &dl.EventID, &dl.SubscriberID, &dl.TotalAttempts,
			&dl.LastError, &dl.LastHTTPStatus, &dl.CreatedAt,
			&dl.ResolvedAt, &dl.ResolvedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning dead letter: %w", err)
		}
		letters = append(letters, dl)
	}

	if letters == nil {
		letters = []domain.DeadLetter{}
	}

	return letters, nil
}

// GetDeadLetter returns a single dead letter by ID.
func (s *PostgresStore) GetDeadLetter(ctx context.Context, id string) (*domain.DeadLetter, error) {
	var dl domain.DeadLetter
	err := s.pool.QueryRow(ctx, `
		SELECT id, event_id, subscriber_id, total_attempts, last_error, last_http_status, created_at, resolved_at, resolved_by
		FROM dead_letter_queue WHERE id = $1
	`, id).Scan(
		&dl.ID, &dl.EventID, &dl.SubscriberID, &dl.TotalAttempts,
		&dl.LastError, &dl.LastHTTPStatus, &dl.CreatedAt,
		&dl.ResolvedAt, &dl.ResolvedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying dead letter: %w", err)
	}
	return &dl, nil
}

// ResolveDeadLetter marks a dead letter as resolved.
func (s *PostgresStore) ResolveDeadLetter(ctx context.Context, id string, resolvedBy string) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE dead_letter_queue SET resolved_at = NOW(), resolved_by = $2
		WHERE id = $1 AND resolved_at IS NULL
	`, id, resolvedBy)
	if err != nil {
		return fmt.Errorf("resolving dead letter: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("dead letter not found or already resolved")
	}
	return nil
}

// ListDeliveryAttempts returns delivery attempts with optional filtering.
func (s *PostgresStore) ListDeliveryAttempts(ctx context.Context, eventID, subscriberID, status string, limit int) ([]domain.DeliveryAttempt, error) {
	query := `SELECT id, event_id, subscriber_id, attempt_number, status, http_status_code, response_body, response_time_ms, error_message, next_retry_at, created_at FROM delivery_attempts`
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
			&a.ResponseTimeMs, &a.ErrorMessage, &a.NextRetryAt, &a.CreatedAt,
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
		SELECT id, event_id, subscriber_id, attempt_number, status, http_status_code, response_body, response_time_ms, error_message, next_retry_at, created_at
		FROM delivery_attempts WHERE id = $1
	`, id).Scan(
		&a.ID, &a.EventID, &a.SubscriberID, &a.AttemptNumber,
		&a.Status, &a.HTTPStatusCode, &a.ResponseBody,
		&a.ResponseTimeMs, &a.ErrorMessage, &a.NextRetryAt, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying delivery attempt: %w", err)
	}
	return &a, nil
}
