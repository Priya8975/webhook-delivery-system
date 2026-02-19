package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/Priya8975/webhook-delivery-system/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) CreateSubscriber(ctx context.Context, req domain.CreateSubscriberRequest) (*domain.Subscriber, error) {
	secretKey, err := generateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("generating secret key: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert subscriber
	var sub domain.Subscriber
	err = tx.QueryRow(ctx, `
		INSERT INTO subscribers (name, endpoint_url, secret_key)
		VALUES ($1, $2, $3)
		RETURNING id, name, endpoint_url, secret_key, is_active, rate_limit_per_second, created_at, updated_at
	`, req.Name, req.EndpointURL, secretKey).Scan(
		&sub.ID, &sub.Name, &sub.EndpointURL, &sub.SecretKey,
		&sub.IsActive, &sub.RateLimitPerSecond, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting subscriber: %w", err)
	}

	// Insert subscriptions for each event type
	for _, eventType := range req.EventTypes {
		_, err = tx.Exec(ctx, `
			INSERT INTO subscriptions (subscriber_id, event_type)
			VALUES ($1, $2)
		`, sub.ID, eventType)
		if err != nil {
			return nil, fmt.Errorf("inserting subscription for %s: %w", eventType, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &sub, nil
}

func (s *PostgresStore) GetSubscriber(ctx context.Context, id string) (*domain.Subscriber, error) {
	var sub domain.Subscriber
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, endpoint_url, secret_key, is_active, rate_limit_per_second, created_at, updated_at
		FROM subscribers WHERE id = $1
	`, id).Scan(
		&sub.ID, &sub.Name, &sub.EndpointURL, &sub.SecretKey,
		&sub.IsActive, &sub.RateLimitPerSecond, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying subscriber: %w", err)
	}
	return &sub, nil
}

func (s *PostgresStore) ListSubscribers(ctx context.Context) ([]domain.Subscriber, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, endpoint_url, is_active, rate_limit_per_second, created_at, updated_at
		FROM subscribers
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []domain.Subscriber
	for rows.Next() {
		var sub domain.Subscriber
		err := rows.Scan(
			&sub.ID, &sub.Name, &sub.EndpointURL,
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

func (s *PostgresStore) UpdateSubscriber(ctx context.Context, id string, req domain.UpdateSubscriberRequest) (*domain.Subscriber, error) {
	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.EndpointURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("endpoint_url = $%d", argIdx))
		args = append(args, *req.EndpointURL)
		argIdx++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *req.IsActive)
		argIdx++
	}
	if req.RateLimitPerSecond != nil {
		setClauses = append(setClauses, fmt.Sprintf("rate_limit_per_second = $%d", argIdx))
		args = append(args, *req.RateLimitPerSecond)
		argIdx++
	}

	if len(setClauses) == 0 {
		return s.GetSubscriber(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := fmt.Sprintf(`
		UPDATE subscribers SET %s
		WHERE id = $%d
		RETURNING id, name, endpoint_url, is_active, rate_limit_per_second, created_at, updated_at
	`, joinStrings(setClauses, ", "), argIdx)
	args = append(args, id)

	var sub domain.Subscriber
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&sub.ID, &sub.Name, &sub.EndpointURL,
		&sub.IsActive, &sub.RateLimitPerSecond, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("updating subscriber: %w", err)
	}

	return &sub, nil
}

func (s *PostgresStore) GetSubscriberSubscriptions(ctx context.Context, subscriberID string) ([]domain.Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, subscriber_id, event_type, is_active, created_at
		FROM subscriptions
		WHERE subscriber_id = $1
		ORDER BY created_at
	`, subscriberID)
	if err != nil {
		return nil, fmt.Errorf("querying subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		err := rows.Scan(&sub.ID, &sub.SubscriberID, &sub.EventType, &sub.IsActive, &sub.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning subscription: %w", err)
		}
		subs = append(subs, sub)
	}

	if subs == nil {
		subs = []domain.Subscription{}
	}

	return subs, nil
}

func generateSecretKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "whdlv_" + hex.EncodeToString(bytes), nil
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
