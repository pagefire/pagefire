// Package auth provides password hashing, session management, and authentication
// helpers for PageFire. It uses argon2id for password hashing and SCS for
// server-side session management.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/pagefire/pagefire/internal/store"
)

// SessionKeyUserID is the session data key for the authenticated user's ID.
const SessionKeyUserID = "user_id"

// Service provides authentication operations.
type Service struct {
	users          store.UserStore
	sessionManager *scs.SessionManager
}

// NewService creates a new auth service with session management backed by SQLite.
func NewService(users store.UserStore, db *sql.DB) *Service {
	sm := scs.New()
	sm.Store = sqlite3store.New(db)
	sm.Lifetime = 24 * time.Hour
	sm.IdleTimeout = 2 * time.Hour
	sm.Cookie.Name = "pagefire_session"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = 3 // Lax
	sm.Cookie.Secure = true // localhost is treated as a secure context by browsers

	return &Service{
		users:          users,
		sessionManager: sm,
	}
}

// SessionManager returns the underlying SCS session manager for use in middleware.
func (s *Service) SessionManager() *scs.SessionManager {
	return s.sessionManager
}

// HashPassword creates an argon2id hash of the given password.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

// CheckPassword compares a plaintext password against an argon2id hash.
func CheckPassword(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

// Login authenticates a user by email and password, creates a session, and
// returns the authenticated user. Returns store.ErrNotFound if the email doesn't
// exist or the password is wrong (same error to prevent enumeration).
func (s *Service) Login(ctx context.Context, email, password string) (*store.User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, store.ErrNotFound
	}

	if !user.IsActive {
		return nil, store.ErrNotFound
	}

	if user.PasswordHash == "" {
		return nil, store.ErrNotFound
	}

	match, err := CheckPassword(password, user.PasswordHash)
	if err != nil || !match {
		return nil, store.ErrNotFound
	}

	// Renew session token to prevent session fixation
	if err := s.sessionManager.RenewToken(ctx); err != nil {
		return nil, fmt.Errorf("renewing session: %w", err)
	}
	s.sessionManager.Put(ctx, SessionKeyUserID, user.ID)

	// Update last login (best-effort)
	_ = s.users.SetLastLogin(ctx, user.ID)

	return user, nil
}

// Logout destroys the current session.
func (s *Service) Logout(ctx context.Context) error {
	return s.sessionManager.Destroy(ctx)
}

// CurrentUser returns the authenticated user from the session, or nil.
// The SCS LoadAndSave middleware must be applied before calling this.
func (s *Service) CurrentUser(ctx context.Context) (user *store.User) {
	// SCS panics if no session data is in context. Recover gracefully so that
	// code paths where LoadAndSave wasn't applied (e.g. pure Bearer-token
	// requests in tests) simply return nil instead of crashing.
	defer func() { recover() }()

	userID := s.sessionManager.GetString(ctx, SessionKeyUserID)
	if userID == "" {
		return nil
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil
	}
	if !user.IsActive {
		return nil
	}
	return user
}

// GenerateAPIToken creates a new API token, returning the raw token string
// (shown once to the user) and persisting the SHA-256 hash.
func (s *Service) GenerateAPIToken(ctx context.Context, userID, name string) (string, *store.APIToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generating token: %w", err)
	}

	prefix := hex.EncodeToString(raw[:4]) // 8 char prefix for identification
	token := "pf_" + hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	t := &store.APIToken{
		UserID: userID,
		Name:   name,
		Prefix: prefix,
	}
	if err := s.users.CreateAPIToken(ctx, t, tokenHash); err != nil {
		return "", nil, err
	}
	return token, t, nil
}

// ValidateAPIToken looks up a raw token by its SHA-256 hash and returns the
// owning user. Returns store.ErrNotFound if invalid or revoked.
func (s *Service) ValidateAPIToken(ctx context.Context, rawToken string) (*store.User, *store.APIToken, error) {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	token, err := s.users.GetAPITokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.users.Get(ctx, token.UserID)
	if err != nil {
		return nil, nil, err
	}
	if !user.IsActive {
		return nil, nil, store.ErrNotFound
	}

	// Update last used (best-effort, async-safe)
	_ = s.users.TouchAPIToken(ctx, token.ID)

	return user, token, nil
}
