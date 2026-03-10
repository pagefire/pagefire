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

func (s *incidentStore) Create(ctx context.Context, inc *store.Incident) error {
	if inc.ID == "" {
		inc.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO incidents (id, title, status, severity, summary, source, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		inc.ID, inc.Title, inc.Status, inc.Severity, inc.Summary, inc.Source, inc.CreatedBy,
	)
	return err
}

func (s *incidentStore) Get(ctx context.Context, id string) (*store.Incident, error) {
	inc := &store.Incident{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, status, severity, summary, source, created_by, created_at, resolved_at FROM incidents WHERE id = ?`, id,
	).Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &inc.Summary, &inc.Source, &inc.CreatedBy, &inc.CreatedAt, &inc.ResolvedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return inc, err
}

func (s *incidentStore) List(ctx context.Context, filter store.IncidentFilter) ([]store.Incident, error) {
	query := `SELECT id, title, status, severity, summary, source, created_by, created_at, resolved_at FROM incidents WHERE 1=1`
	var args []any

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
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
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &inc.Summary, &inc.Source, &inc.CreatedBy, &inc.CreatedAt, &inc.ResolvedAt); err != nil {
			return nil, err
		}
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

func (s *incidentStore) CreateUpdate(ctx context.Context, u *store.IncidentUpdate) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO incident_updates (id, incident_id, status, message, created_by) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.IncidentID, u.Status, u.Message, u.CreatedBy,
	)
	return err
}

func (s *incidentStore) ListUpdates(ctx context.Context, incidentID string) ([]store.IncidentUpdate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, incident_id, status, message, created_by, created_at FROM incident_updates WHERE incident_id = ? ORDER BY created_at`, incidentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []store.IncidentUpdate
	for rows.Next() {
		var u store.IncidentUpdate
		if err := rows.Scan(&u.ID, &u.IncidentID, &u.Status, &u.Message, &u.CreatedBy, &u.CreatedAt); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	return updates, rows.Err()
}
