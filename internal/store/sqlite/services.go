package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type serviceStore struct {
	db *sql.DB
}

func (s *serviceStore) Create(ctx context.Context, svc *store.Service) error {
	if svc.ID == "" {
		svc.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO services (id, name, description, escalation_policy_id) VALUES (?, ?, ?, ?)`,
		svc.ID, svc.Name, svc.Description, svc.EscalationPolicyID,
	)
	return err
}

func (s *serviceStore) Get(ctx context.Context, id string) (*store.Service, error) {
	svc := &store.Service{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, escalation_policy_id, created_at FROM services WHERE id = ?`, id,
	).Scan(&svc.ID, &svc.Name, &svc.Description, &svc.EscalationPolicyID, &svc.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return svc, err
}

func (s *serviceStore) List(ctx context.Context) ([]store.Service, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, escalation_policy_id, created_at FROM services ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []store.Service
	for rows.Next() {
		var svc store.Service
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.Description, &svc.EscalationPolicyID, &svc.CreatedAt); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

func (s *serviceStore) Update(ctx context.Context, svc *store.Service) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE services SET name = ?, description = ?, escalation_policy_id = ? WHERE id = ?`,
		svc.Name, svc.Description, svc.EscalationPolicyID, svc.ID,
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

func (s *serviceStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *serviceStore) CreateIntegrationKey(ctx context.Context, ik *store.IntegrationKey) error {
	if ik.ID == "" {
		ik.ID = uuid.NewString()
	}
	if ik.Secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		ik.Secret = hex.EncodeToString(b)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO integration_keys (id, service_id, name, type, secret) VALUES (?, ?, ?, ?, ?)`,
		ik.ID, ik.ServiceID, ik.Name, ik.Type, ik.Secret,
	)
	return err
}

func (s *serviceStore) ListIntegrationKeys(ctx context.Context, serviceID string) ([]store.IntegrationKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, service_id, name, type, secret, created_at FROM integration_keys WHERE service_id = ?`, serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []store.IntegrationKey
	for rows.Next() {
		var ik store.IntegrationKey
		if err := rows.Scan(&ik.ID, &ik.ServiceID, &ik.Name, &ik.Type, &ik.Secret, &ik.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, ik)
	}
	return keys, rows.Err()
}

func (s *serviceStore) GetIntegrationKeyBySecret(ctx context.Context, secret string) (*store.IntegrationKey, error) {
	ik := &store.IntegrationKey{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, service_id, name, type, secret, created_at FROM integration_keys WHERE secret = ?`, secret,
	).Scan(&ik.ID, &ik.ServiceID, &ik.Name, &ik.Type, &ik.Secret, &ik.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return ik, err
}

func (s *serviceStore) DeleteIntegrationKey(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM integration_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}
