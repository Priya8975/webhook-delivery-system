package store

import (
	"context"
	"fmt"

	"github.com/Priya8975/webhook-delivery-system/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) CreateEvent(ctx context.Context, eventType string, payload []byte, source string) (*domain.Event, error) {
	var event domain.Event
	err := s.pool.QueryRow(ctx, `
		INSERT INTO events (event_type, payload, source)
		VALUES ($1, $2, $3)
		RETURNING id, event_type, payload, source, created_at
	`, eventType, payload, source).Scan(
		&event.ID, &event.EventType, &event.Payload, &event.Source, &event.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting event: %w", err)
	}
	return &event, nil
}

func (s *PostgresStore) GetEvent(ctx context.Context, id string) (*domain.Event, error) {
	var event domain.Event
	err := s.pool.QueryRow(ctx, `
		SELECT id, event_type, payload, source, created_at
		FROM events WHERE id = $1
	`, id).Scan(
		&event.ID, &event.EventType, &event.Payload, &event.Source, &event.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying event: %w", err)
	}
	return &event, nil
}

func (s *PostgresStore) ListEvents(ctx context.Context, eventType string, limit int) ([]domain.Event, error) {
	query := `SELECT id, event_type, payload, source, created_at FROM events`
	args := []interface{}{}
	argIdx := 1

	if eventType != "" {
		query += fmt.Sprintf(" WHERE event_type = $%d", argIdx)
		args = append(args, eventType)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Source, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, e)
	}

	if events == nil {
		events = []domain.Event{}
	}

	return events, nil
}

// FindMatchingSubscribers finds all active subscribers whose event type
// patterns match the given event type.
func (s *PostgresStore) FindMatchingSubscribers(ctx context.Context, eventType string) ([]domain.Subscriber, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT s.id, s.name, s.endpoint_url, s.secret_key, s.is_active,
			   s.rate_limit_per_second, s.created_at, s.updated_at
		FROM subscribers s
		JOIN subscriptions sub ON s.id = sub.subscriber_id
		WHERE s.is_active = true
		  AND sub.is_active = true
		  AND (
			sub.event_type = $1
			OR sub.event_type = '*'
			OR (
				sub.event_type LIKE '%.*'
				AND $1 LIKE REPLACE(sub.event_type, '.*', '.%')
			)
		  )
	`, eventType)
	if err != nil {
		return nil, fmt.Errorf("finding matching subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []domain.Subscriber
	for rows.Next() {
		var sub domain.Subscriber
		err := rows.Scan(
			&sub.ID, &sub.Name, &sub.EndpointURL, &sub.SecretKey,
			&sub.IsActive, &sub.RateLimitPerSecond, &sub.CreatedAt, &sub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning subscriber: %w", err)
		}
		subscribers = append(subscribers, sub)
	}

	if subscribers == nil {
		subscribers = []domain.Subscriber{}
	}

	return subscribers, nil
}
