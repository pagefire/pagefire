package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type incidentStore struct {
	db *sql.DB
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func (s *incidentStore) Create(ctx context.Context, inc *store.Incident) error {
	if inc.ID == "" {
		inc.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO incidents (id, title, status, severity, summary, source, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		inc.ID, inc.Title, inc.Status, inc.Severity, inc.Summary, inc.Source, nullString(inc.CreatedBy),
	)
	return err
}

func (s *incidentStore) Get(ctx context.Context, id string) (*store.Incident, error) {
	inc := &store.Incident{}
	var createdBy sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, status, severity, summary, source, created_by, created_at, resolved_at FROM incidents WHERE id = ?`, id,
	).Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &inc.Summary, &inc.Source, &createdBy, &inc.CreatedAt, &inc.ResolvedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	inc.CreatedBy = fromNullString(createdBy)
	return inc, err
}

func (s *incidentStore) List(ctx context.Context, filter store.IncidentFilter) ([]store.Incident, error) {
	query := `SELECT id, title, status, severity, summary, source, created_by, created_at, resolved_at FROM incidents WHERE 1=1`
	var args []any

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.Search != "" {
		query += ` AND (title LIKE ? OR summary LIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like)
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

	var incidents []store.Incident
	for rows.Next() {
		var inc store.Incident
		var createdBy sql.NullString
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &inc.Summary, &inc.Source, &createdBy, &inc.CreatedAt, &inc.ResolvedAt); err != nil {
			return nil, err
		}
		inc.CreatedBy = fromNullString(createdBy)
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}

func (s *incidentStore) Update(ctx context.Context, inc *store.Incident) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE incidents SET title = ?, status = ?, severity = ?, summary = ?, resolved_at = ? WHERE id = ?`,
		inc.Title, inc.Status, inc.Severity, inc.Summary, inc.ResolvedAt, inc.ID,
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

func (s *incidentStore) AddService(ctx context.Context, incidentID, serviceID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO incident_services (incident_id, service_id) VALUES (?, ?)`,
		incidentID, serviceID,
	)
	return err
}

func (s *incidentStore) ListServices(ctx context.Context, incidentID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT service_id FROM incident_services WHERE incident_id = ?`, incidentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var serviceIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		serviceIDs = append(serviceIDs, id)
	}
	return serviceIDs, rows.Err()
}

func (s *incidentStore) LinkAlert(ctx context.Context, incidentID, alertID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO incident_alerts (incident_id, alert_id) VALUES (?, ?)`,
		incidentID, alertID,
	)
	return err
}

func (s *incidentStore) UnlinkAlert(ctx context.Context, incidentID, alertID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM incident_alerts WHERE incident_id = ? AND alert_id = ?`,
		incidentID, alertID,
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

func (s *incidentStore) ListAlerts(ctx context.Context, incidentID string) ([]*store.Alert, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.service_id, a.status, a.summary, a.details, a.source,
		        a.dedup_key, a.group_key, a.escalation_policy_snapshot,
		        a.escalation_step, a.loop_count, a.next_escalation_at,
		        a.acknowledged_by, a.acknowledged_at, a.resolved_at, a.created_at
		 FROM alerts a
		 INNER JOIN incident_alerts ia ON ia.alert_id = a.id
		 WHERE ia.incident_id = ?
		 ORDER BY a.created_at DESC`, incidentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*store.Alert
	for rows.Next() {
		a := &store.Alert{}
		if err := rows.Scan(
			&a.ID, &a.ServiceID, &a.Status, &a.Summary, &a.Details, &a.Source,
			&a.DeduplicationKey, &a.GroupKey, &a.EscalationPolicySnapshot,
			&a.EscalationStep, &a.LoopCount, &a.NextEscalationAt,
			&a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (s *incidentStore) GetIncidentForAlert(ctx context.Context, alertID string) (*store.Incident, error) {
	inc := &store.Incident{}
	var createdBy sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT i.id, i.title, i.status, i.severity, i.summary, i.source, i.created_by, i.created_at, i.resolved_at
		 FROM incidents i
		 INNER JOIN incident_alerts ia ON ia.incident_id = i.id
		 WHERE ia.alert_id = ?
		 LIMIT 1`, alertID,
	).Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &inc.Summary, &inc.Source, &createdBy, &inc.CreatedAt, &inc.ResolvedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	inc.CreatedBy = fromNullString(createdBy)
	return inc, nil
}

func (s *incidentStore) CreateUpdate(ctx context.Context, u *store.IncidentUpdate) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO incident_updates (id, incident_id, status, message, created_by) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.IncidentID, u.Status, u.Message, nullString(u.CreatedBy),
	)
	return err
}

func (s *incidentStore) ListUpdates(ctx context.Context, incidentID string) ([]store.IncidentUpdate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT iu.id, iu.incident_id, iu.status, iu.message, iu.created_by, u.name, iu.created_at
		 FROM incident_updates iu
		 LEFT JOIN users u ON iu.created_by = u.id
		 WHERE iu.incident_id = ?
		 ORDER BY iu.created_at`, incidentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []store.IncidentUpdate
	for rows.Next() {
		var u store.IncidentUpdate
		var createdBy, createdByName sql.NullString
		if err := rows.Scan(&u.ID, &u.IncidentID, &u.Status, &u.Message, &createdBy, &createdByName, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.CreatedBy = fromNullString(createdBy)
		u.CreatedByName = fromNullString(createdByName)
		updates = append(updates, u)
	}
	return updates, rows.Err()
}
