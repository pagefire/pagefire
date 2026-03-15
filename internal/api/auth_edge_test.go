package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/sqlite"
)

// ---------- Expired / Invalid Session Cookie ----------

func TestSessionOrTokenAuth_ExpiredSessionCookie(t *testing.T) {
	// An invalid/garbage session cookie should be treated as unauthenticated.
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	authSvc := auth.NewService(s.Users(), s.DB())
	mw := SessionOrTokenAuth(authSvc)
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Set a fake/expired session cookie — SCS will not find a valid session.
	req.AddCookie(&http.Cookie{Name: "pagefire_session", Value: "invalid-garbage-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expired/invalid session cookie: status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "authentication required" {
		t.Errorf("error = %q, want %q", body["error"], "authentication required")
	}
}

// ---------- Revoked API Token ----------

func TestSessionOrTokenAuth_RevokedAPIToken(t *testing.T) {
	ctx := context.Background()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	// Create a user
	user := &store.User{
		Name:     "test-user",
		Email:    "test@example.com",
		Role:     store.RoleAdmin,
		Timezone: "UTC",
		IsActive: true,
	}
	hash, err := auth.HashPassword("TestPass1")
	if err != nil {
		t.Fatal(err)
	}
	user.PasswordHash = hash
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	authSvc := auth.NewService(s.Users(), s.DB())

	// Generate a token, then revoke it
	rawToken, token, err := authSvc.GenerateAPIToken(ctx, user.ID, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Users().RevokeAPIToken(ctx, token.ID); err != nil {
		t.Fatal(err)
	}

	// Try to use the revoked token
	mw := SessionOrTokenAuth(authSvc)
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("revoked token: status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "invalid token" {
		t.Errorf("error = %q, want %q", body["error"], "invalid token")
	}
}

// ---------- Content-Type CSRF Protection ----------

func TestDecodeJSON_RejectsNonJSONContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{"empty content-type", ""},
		{"form urlencoded", "application/x-www-form-urlencoded"},
		{"multipart form", "multipart/form-data"},
		{"text plain", "text/plain"},
		{"text html", "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(`{"email":"a@b.com","password":"Test1234"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			var v struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			err := decodeJSON(rr, req, &v)
			if err == nil {
				t.Errorf("expected error for Content-Type %q, got nil", tt.contentType)
			}
		})
	}
}

func TestDecodeJSON_AcceptsJSONContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{"exact", "application/json"},
		{"with charset", "application/json; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(`{"email":"a@b.com"}`)
			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()

			var v struct {
				Email string `json:"email"`
			}
			err := decodeJSON(rr, req, &v)
			if err != nil {
				t.Errorf("expected no error for Content-Type %q, got %v", tt.contentType, err)
			}
			if v.Email != "a@b.com" {
				t.Errorf("email = %q, want %q", v.Email, "a@b.com")
			}
		})
	}
}

// ---------- Password Complexity ----------

func TestValidatePassword_Complexity(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		// Should reject
		{"all lowercase", "abcdefgh", true},
		{"all uppercase", "ABCDEFGH", true},
		{"no digit", "Abcdefgh", true},
		{"no uppercase", "abcdefg1", true},
		{"no lowercase", "ABCDEFG1", true},
		{"too short", "Ab1", true},
		{"empty", "", true},
		{"7 chars valid mix", "Abcdef1", true},

		// Should accept
		{"valid 8 chars", "Abcdefg1", false},
		{"valid longer", "MyP@ssw0rd123", false},
		{"valid with special", "Test!ng1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			if tt.wantErr && err == nil {
				t.Errorf("validatePassword(%q) = nil, want error", tt.password)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validatePassword(%q) = %v, want nil", tt.password, err)
			}
		})
	}
}

// ---------- Valid API Token Works ----------

func TestSessionOrTokenAuth_ValidAPIToken(t *testing.T) {
	ctx := context.Background()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	user := &store.User{
		Name:     "token-user",
		Email:    "token@example.com",
		Role:     store.RoleAdmin,
		Timezone: "UTC",
		IsActive: true,
	}
	hash, err := auth.HashPassword("TestPass1")
	if err != nil {
		t.Fatal(err)
	}
	user.PasswordHash = hash
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	authSvc := auth.NewService(s.Users(), s.DB())
	rawToken, _, err := authSvc.GenerateAPIToken(ctx, user.ID, "my-token")
	if err != nil {
		t.Fatal(err)
	}

	// Verify the token grants access
	mw := SessionOrTokenAuth(authSvc)
	capturedUser := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u != nil && u.ID == user.ID {
			capturedUser = true
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("valid token: status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !capturedUser {
		t.Error("valid token: expected user to be set in context")
	}
}

// ---------- Inactive User Token Rejected ----------

func TestSessionOrTokenAuth_InactiveUserToken(t *testing.T) {
	ctx := context.Background()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	user := &store.User{
		Name:     "inactive-user",
		Email:    "inactive@example.com",
		Role:     store.RoleUser,
		Timezone: "UTC",
		IsActive: true,
	}
	hash, err := auth.HashPassword("TestPass1")
	if err != nil {
		t.Fatal(err)
	}
	user.PasswordHash = hash
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	authSvc := auth.NewService(s.Users(), s.DB())
	rawToken, _, err := authSvc.GenerateAPIToken(ctx, user.ID, "soon-disabled")
	if err != nil {
		t.Fatal(err)
	}

	// Deactivate the user via direct SQL since Update() does not persist is_active.
	// This is a known gap in the store layer (Update omits is_active from the query).
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET is_active = 0 WHERE id = ?`, user.ID); err != nil {
		t.Fatal(err)
	}

	mw := SessionOrTokenAuth(authSvc)
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("inactive user token: status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
