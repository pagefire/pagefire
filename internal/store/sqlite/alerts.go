package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type alertStore struct {
	db *sql.DB
}

func (s *alertStore) Create(ctx context.Context, a *store.Alert) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}

	// Deduplication: if an active alert exists with the same service_id + dedup_key, return it.
	// The partial unique index idx_alerts_active_dedup enforces this at the DB level as well,
	// but we check first to return the existing alert ID to the caller.
	if a.DeduplicationKey != "" {
		var existingID string
		err := s.db.QueryRowContext(ctx,
			`SELECT id FROM alerts WHERE service_id = ? AND dedup_key = ? AND status != ?`,
			a.ServiceID, a.DeduplicationKey, store.AlertStatusResolved,
		).Scan(&existingID)
		if err == nil {
			a.ID = existingID
			return store.ErrDuplicateKey
		}
	}

	now := time.Now().UTC()
	a.NextEscalationAt = &now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alerts (id, service_id, status, summary, details, source, dedup_key, escalation_policy_snapshot, next_escalation_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ServiceID, store.AlertStatusTriggered, a.Summary, a.Details, a.Source,
		a.DeduplicationKey, a.EscalationPolicySnapshot, a.NextEscalationAt,
	)
	// If the unique index catches a race condition, treat it as a dedup
	if err != nil && a.DeduplicationKey != "" {
		var existingID string
		rerr := s.db.QueryRowContext(ctx,
			`SELECT id FROM alerts WHERE service_id = ? AND dedup_key = ? AND status != ?`,
			a.ServiceID, a.DeduplicationKey, store.AlertStatusResolved,
		).Scan(&existingID)
		if rerr == nil {
			a.ID = existingID
			return store.ErrDuplicateKey
		}
	}
	return err
}

func (s *alertStore) Get(ctx context.Context, id string) (*store.Alert, error) {
	a := &store.Alert{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, service_id, status, summary, details, source, dedup_key,
		        escalation_policy_snapshot, escalation_step, loop_count,
		        next_escalation_at, acknowledged_by, acknowledged_at, resolved_at, created_at
		 FROM alerts WHERE id = ?`, id,
	).Scan(&a.ID, &a.ServiceID, &a.Status, &a.Summary, &a.Details, &a.Source,
		&a.DeduplicationKey, &a.EscalationPolicySnapshot, &a.EscalationStep, &a.LoopCount,
		&a.NextEscalationAt, &a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return a, err
}

func (s *alertStore) List(ctx context.Context, filter store.AlertFilter) ([]store.Alert, error) {
	query := `SELECT id, service_id, status, summary, details, source, dedup_key,
	                 escalation_policy_snapshot, escalation_step, loop_count,
	                 next_escalation_at, acknowledged_by, acknowledged_at, resolved_at, created_at
	          FROM alerts WHERE 1=1`
	var args []any

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.ServiceID != "" {
		query += ` AND service_id = ?`
		args = append(args, filter.ServiceID)
	}

	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []store.Alert
	for rows.Next() {
		var a store.Alert
		if err := rows.Scan(&a.ID, &a.ServiceID, &a.Status, &a.Summary, &a.Details, &a.Source,
			&a.DeduplicationKey, &a.EscalationPolicySnapshot, &a.EscalationStep, &a.LoopCount,
			&a.NextEscalationAt, &a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (s *alertStore) Acknowledge(ctx context.Context, id string, userID string) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE alerts SET status = ?, acknowledged_by = ?, acknowledged_at = ?, next_escalation_at = NULL
		 WHERE id = ? AND status = ?`,
		store.AlertStatusAcknowledged, userID, now, id, store.AlertStatusTriggered,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *alertStore) Resolve(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE alerts SET status = ?, resolved_at = ?, next_escalation_at = NULL
		 WHERE id = ? AND status != ?`,
		store.AlertStatusResolved, now, id, store.AlertStatusResolved,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *alertStore) FindPendingEscalations(ctx context.Context, before time.Time) ([]store.Alert, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, service_id, status, summary, details, source, dedup_key,
		        escalation_policy_snapshot, escalation_step, loop_count,
		        next_escalation_at, acknowledged_by, acknowledged_at, resolved_at, created_at
		 FROM alerts
		 WHERE status = ? AND next_escalation_at IS NOT NULL AND next_escalation_at <= ?`,
		store.AlertStatusTriggered, before.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []store.Alert
	for rows.Next() {
		var a store.Alert
		if err := rows.Scan(&a.ID, &a.ServiceID, &a.Status, &a.Summary, &a.Details, &a.Source,
			&a.DeduplicationKey, &a.EscalationPolicySnapshot, &a.EscalationStep, &a.LoopCount,
			&a.NextEscalationAt, &a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (s *alertStore) UpdateEscalationStep(ctx context.Context, id string, step int, loopCount int, nextAt time.Time) error {
	var nextAtParam any
	if !nextAt.IsZero() {
		nextAtParam = nextAt.UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE alerts SET escalation_step = ?, loop_count = ?, next_escalation_at = ? WHERE id = ?`,
		step, loopCount, nextAtParam, id,
	)
	return err
}

func (s *alertStore) CreateLog(ctx context.Context, log *store.AlertLog) error {
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alert_logs (id, alert_id, event, message, user_id) VALUES (?, ?, ?, ?, ?)`,
		log.ID, log.AlertID, log.Event, log.Message, log.UserID,
	)
	return err
}

func (s *alertStore) ListLogs(ctx context.Context, alertID string) ([]store.AlertLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, alert_id, event, message, user_id, created_at FROM alert_logs WHERE alert_id = ? ORDER BY created_at`, alertID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []store.AlertLog
	for rows.Next() {
		var l store.AlertLog
		if err := rows.Scan(&l.ID, &l.AlertID, &l.Event, &l.Message, &l.UserID, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
