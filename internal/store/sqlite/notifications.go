package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type notificationStore struct {
	db *sql.DB
}

func (s *notificationStore) Enqueue(ctx context.Context, n *store.Notification) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.Status == "" {
		n.Status = store.NotificationStatusPending
	}
	if n.MaxAttempts == 0 {
		n.MaxAttempts = 3
	}
	var nextAt any
	if n.NextAttemptAt != nil {
		utc := n.NextAttemptAt.UTC()
		nextAt = &utc
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_queue
		 (id, alert_id, user_id, contact_method_id, type, destination_type, destination, subject, body, status, max_attempts, next_attempt_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.AlertID, n.UserID, n.ContactMethodID, n.Type,
		n.DestinationType, n.Destination, n.Subject, n.Body, n.Status,
		n.MaxAttempts, nextAt,
	)
	return err
}

func (s *notificationStore) FindPending(ctx context.Context, limit int) ([]store.Notification, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, alert_id, user_id, contact_method_id, type, destination_type, destination,
		        subject, body, status, attempts, max_attempts, next_attempt_at, sent_at, provider_id, created_at
		 FROM notification_queue
		 WHERE status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
		 ORDER BY created_at
		 LIMIT ?`,
		store.NotificationStatusPending, time.Now().UTC(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []store.Notification
	for rows.Next() {
		var n store.Notification
		if err := rows.Scan(&n.ID, &n.AlertID, &n.UserID, &n.ContactMethodID, &n.Type,
			&n.DestinationType, &n.Destination, &n.Subject, &n.Body, &n.Status,
			&n.Attempts, &n.MaxAttempts, &n.NextAttemptAt, &n.SentAt, &n.ProviderID, &n.CreatedAt,
		); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

func (s *notificationStore) MarkSending(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_queue SET status = ? WHERE id = ?`,
		store.NotificationStatusSending, id,
	)
	return err
}

func (s *notificationStore) MarkSent(ctx context.Context, id string, providerID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_queue SET status = ?, sent_at = ?, provider_id = ? WHERE id = ?`,
		store.NotificationStatusSent, now, providerID, id,
	)
	return err
}

func (s *notificationStore) MarkFailed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_queue SET status = ? WHERE id = ?`,
		store.NotificationStatusFailed, id,
	)
	return err
}

func (s *notificationStore) IncrementAttempts(ctx context.Context, id string, nextAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notification_queue SET attempts = attempts + 1, status = ?, next_attempt_at = ? WHERE id = ?`,
		store.NotificationStatusPending, nextAt.UTC(), id,
	)
	return err
}
