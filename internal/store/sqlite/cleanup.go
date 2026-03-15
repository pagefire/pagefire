package sqlite

import (
	"context"
	"fmt"
	"time"
)

// PurgeResolvedAlerts deletes resolved alerts older than the given time,
// along with their associated alert_logs and notification_queue records.
func (s *SQLiteStore) PurgeResolvedAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	cutoff := olderThan.UTC()

	// Delete notification_queue records referencing alerts that will be purged.
	_, err = tx.ExecContext(ctx,
		`DELETE FROM notification_queue WHERE alert_id IN (
			SELECT id FROM alerts WHERE status = 'resolved' AND resolved_at < ?
		)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge notifications for old alerts: %w", err)
	}

	// Delete alert_logs for alerts that will be purged.
	_, err = tx.ExecContext(ctx,
		`DELETE FROM alert_logs WHERE alert_id IN (
			SELECT id FROM alerts WHERE status = 'resolved' AND resolved_at < ?
		)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge alert_logs for old alerts: %w", err)
	}

	// Delete the alerts themselves.
	res, err := tx.ExecContext(ctx,
		`DELETE FROM alerts WHERE status = 'resolved' AND resolved_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge resolved alerts: %w", err)
	}

	n, _ := res.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return n, nil
}

// PurgeOldNotifications deletes sent or failed notifications older than the given time.
func (s *SQLiteStore) PurgeOldNotifications(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM notification_queue WHERE status IN ('sent', 'failed') AND created_at < ?`,
		olderThan.UTC())
	if err != nil {
		return 0, fmt.Errorf("purge old notifications: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PurgeExpiredOverrides deletes schedule overrides whose end_time is in the past.
func (s *SQLiteStore) PurgeExpiredOverrides(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM schedule_overrides WHERE end_time < ?`,
		time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("purge expired overrides: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
