package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type userStore struct {
	db *sql.DB
}

// userCols is the column list for user queries. Keep in sync with scanUser.
const userCols = `id, name, email, role, timezone, avatar_url, password_hash, auth_provider, auth_provider_id, is_active, last_login, created_at`

// scanUser scans a row into a User struct. Column order must match userCols.
func scanUser(sc interface{ Scan(...any) error }) (*store.User, error) {
	u := &store.User{}
	var passwordHash, authProvider, authProviderID, avatarURL, lastLoginStr sql.NullString
	err := sc.Scan(
		&u.ID, &u.Name, &u.Email, &u.Role, &u.Timezone, &avatarURL,
		&passwordHash, &authProvider, &authProviderID,
		&u.IsActive, &lastLoginStr, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.AvatarURL = avatarURL.String
	u.PasswordHash = passwordHash.String
	if authProvider.Valid {
		u.AuthProvider = &authProvider.String
	}
	if authProviderID.Valid {
		u.AuthProviderID = &authProviderID.String
	}
	if lastLoginStr.Valid {
		if t, err := time.Parse(time.RFC3339, lastLoginStr.String); err == nil {
			u.LastLogin = &t
		}
	}
	return u, nil
}

func (s *userStore) Create(ctx context.Context, u *store.User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email, role, timezone, avatar_url, password_hash, auth_provider, auth_provider_id, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.Email, u.Role, u.Timezone, u.AvatarURL,
		nilIfEmpty(u.PasswordHash), u.AuthProvider, u.AuthProviderID, u.IsActive,
	)
	return err
}

func (s *userStore) Get(ctx context.Context, id string) (*store.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return u, err
}

func (s *userStore) GetByEmail(ctx context.Context, email string) (*store.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE email = ?`, email)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return u, err
}

func (s *userStore) List(ctx context.Context) ([]store.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []store.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
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

func (s *userStore) SetPassword(ctx context.Context, id string, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id,
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

func (s *userStore) SetLastLogin(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET last_login = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id,
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

func (s *userStore) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// Contact methods

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

// Notification rules

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

// API tokens

func (s *userStore) CreateAPIToken(ctx context.Context, t *store.APIToken, tokenHash string) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (id, user_id, name, token_hash, prefix) VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.Name, tokenHash, t.Prefix,
	)
	return err
}

func (s *userStore) ListAPITokens(ctx context.Context, userID string) ([]store.APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, prefix, created_at, last_used, revoked_at
		 FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []store.APIToken
	for rows.Next() {
		var t store.APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.CreatedAt, &t.LastUsed, &t.RevokedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *userStore) GetAPITokenByHash(ctx context.Context, tokenHash string) (*store.APIToken, error) {
	t := &store.APIToken{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, prefix, created_at, last_used, revoked_at
		 FROM api_tokens WHERE token_hash = ? AND revoked_at IS NULL`, tokenHash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.CreatedAt, &t.LastUsed, &t.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return t, err
}

func (s *userStore) RevokeAPIToken(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now().UTC(), id,
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

func (s *userStore) TouchAPIToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used = ? WHERE id = ?`, time.Now().UTC(), id,
	)
	return err
}

// --- Invite Tokens ---

func (s *userStore) CreateInviteToken(ctx context.Context, t *store.InviteToken) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	t.CreatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO invite_tokens (id, user_id, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.TokenHash, t.ExpiresAt.UTC().Format(time.RFC3339), t.CreatedAt,
	)
	return err
}

func (s *userStore) GetInviteTokenByHash(ctx context.Context, tokenHash string) (*store.InviteToken, error) {
	t := &store.InviteToken{}
	var expiresStr, usedStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, used_at, created_at
		 FROM invite_tokens WHERE token_hash = ?`, tokenHash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &expiresStr, &usedStr, &t.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	if expiresStr.Valid {
		if parsed, err := time.Parse(time.RFC3339, expiresStr.String); err == nil {
			t.ExpiresAt = parsed
		}
	}
	if usedStr.Valid {
		if parsed, err := time.Parse(time.RFC3339, usedStr.String); err == nil {
			t.UsedAt = &parsed
		}
	}
	return t, nil
}

func (s *userStore) UseInviteToken(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE invite_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), id,
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

// nilIfEmpty returns nil for empty strings, used for nullable text columns.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
