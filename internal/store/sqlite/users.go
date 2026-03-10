package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type userStore struct {
	db *sql.DB
}

func (s *userStore) Create(ctx context.Context, u *store.User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email, role, timezone, avatar_url) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.Email, u.Role, u.Timezone, u.AvatarURL,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *userStore) Get(ctx context.Context, id string) (*store.User, error) {
	u := &store.User{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, role, timezone, avatar_url, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Timezone, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return u, err
}

func (s *userStore) GetByEmail(ctx context.Context, email string) (*store.User, error) {
	u := &store.User{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, role, timezone, avatar_url, created_at FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Timezone, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return u, err
}

func (s *userStore) List(ctx context.Context) ([]store.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, email, role, timezone, avatar_url, created_at FROM users ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []store.User
	for rows.Next() {
		var u store.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Timezone, &u.AvatarURL, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *userStore) Update(ctx context.Context, u *store.User) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET name = ?, email = ?, role = ?, timezone = ?, avatar_url = ? WHERE id = ?`,
		u.Name, u.Email, u.Role, u.Timezone, u.AvatarURL, u.ID,
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

func (s *userStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *userStore) CreateContactMethod(ctx context.Context, cm *store.ContactMethod) error {
	if cm.ID == "" {
		cm.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contact_methods (id, user_id, type, value) VALUES (?, ?, ?, ?)`,
		cm.ID, cm.UserID, cm.Type, cm.Value,
	)
	return err
}

func (s *userStore) ListContactMethods(ctx context.Context, userID string) ([]store.ContactMethod, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, type, value, verified, created_at FROM contact_methods WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var methods []store.ContactMethod
	for rows.Next() {
		var cm store.ContactMethod
		if err := rows.Scan(&cm.ID, &cm.UserID, &cm.Type, &cm.Value, &cm.Verified, &cm.CreatedAt); err != nil {
			return nil, err
		}
		methods = append(methods, cm)
	}
	return methods, rows.Err()
}

func (s *userStore) DeleteContactMethod(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM contact_methods WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *userStore) CreateNotificationRule(ctx context.Context, nr *store.NotificationRule) error {
	if nr.ID == "" {
		nr.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_rules (id, user_id, contact_method_id, delay_minutes) VALUES (?, ?, ?, ?)`,
		nr.ID, nr.UserID, nr.ContactMethodID, nr.DelayMinutes,
	)
	return err
}

func (s *userStore) ListNotificationRules(ctx context.Context, userID string) ([]store.NotificationRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, contact_method_id, delay_minutes FROM notification_rules WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []store.NotificationRule
	for rows.Next() {
		var nr store.NotificationRule
		if err := rows.Scan(&nr.ID, &nr.UserID, &nr.ContactMethodID, &nr.DelayMinutes); err != nil {
			return nil, err
		}
		rules = append(rules, nr)
	}
	return rules, rows.Err()
}

func (s *userStore) DeleteNotificationRule(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notification_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}
