package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/sqlite"
)

// okHandler is a simple handler that returns 200 OK for testing middleware.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

func newTestAuthMiddleware(t *testing.T) (func(http.Handler) http.Handler, *auth.Service) {
	t.Helper()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	authSvc := auth.NewService(s.Users(), s.DB())
	return SessionOrTokenAuth(authSvc), authSvc
}

// ---------- SessionOrTokenAuth ----------

func TestSessionOrTokenAuth_NoAuthorizationHeader(t *testing.T) {
	mw, _ := newTestAuthMiddleware(t)
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "authentication required" {
		t.Errorf("error = %q, want %q", body["error"], "authentication required")
	}
}

func TestSessionOrTokenAuth_InvalidFormat(t *testing.T) {
	mw, _ := newTestAuthMiddleware(t)
	handler := mw(okHandler)

	tests := []struct {
		name  string
		value string
	}{
		{"no prefix", "secret-token"},
		{"wrong prefix", "Token secret-token"},
		{"basic auth", "Basic c2VjcmV0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.value)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestSessionOrTokenAuth_WrongToken(t *testing.T) {
	mw, _ := newTestAuthMiddleware(t)
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ---------- RequireRole ----------

func TestRequireRole_Allowed(t *testing.T) {
	handler := RequireRole(store.RoleAdmin)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userContextKey, &store.User{Role: store.RoleAdmin})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRequireRole_Denied(t *testing.T) {
	handler := RequireRole(store.RoleAdmin)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userContextKey, &store.User{Role: store.RoleUser})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

// ---------- SecurityHeaders ----------

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expected := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "0",
		"Content-Security-Policy": "default-src 'none'",
		"Referrer-Policy":         "no-referrer",
	}

	for header, want := range expected {
		got := rr.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ---------- RateLimiter ----------

func TestRateLimiter_AllowUpToLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.Allow("key1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_RejectAfterLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		rl.Allow("key1")
	}

	if rl.Allow("key1") {
		t.Error("request after limit should be rejected")
	}
}

func TestRateLimiter_AllowAfterWindowPasses(t *testing.T) {
	window := 10 * time.Millisecond
	rl := NewRateLimiter(2, window)

	for i := 0; i < 2; i++ {
		if !rl.Allow("key1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow("key1") {
		t.Fatal("should be rate limited")
	}

	time.Sleep(window + 5*time.Millisecond)

	if !rl.Allow("key1") {
		t.Error("should be allowed after window passes")
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	if !rl.Allow("key1") {
		t.Error("key1 first request should be allowed")
	}
	if rl.Allow("key1") {
		t.Error("key1 second request should be rejected")
	}
	if !rl.Allow("key2") {
		t.Error("key2 should be independently allowed")
	}
}

// ---------- RateLimitMiddleware ----------

func TestRateLimitMiddleware_Returns429WhenLimited(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	handler := RateLimitMiddleware(rl)(okHandler)

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want %d", rr1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "1.2.3.4:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitMiddleware_IgnoresXRealIP(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	handler := RateLimitMiddleware(rl)(okHandler)

	// First request from 127.0.0.1:1234 with X-Real-IP header
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	req1.Header.Set("X-Real-IP", "10.0.0.1")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want %d", rr1.Code, http.StatusOK)
	}

	// Second request from different RemoteAddr but same X-Real-IP — should be allowed
	// because we use RemoteAddr (not X-Real-IP) to prevent spoofing
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.1:5678"
	req2.Header.Set("X-Real-IP", "10.0.0.1")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("second request: status = %d, want %d (different RemoteAddr = different key)", rr2.Code, http.StatusOK)
	}
}

// ---------- UserFromContext ----------

func TestUserFromContext_NoUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := UserFromContext(req.Context())
	if user != nil {
		t.Errorf("expected nil user, got %+v", user)
	}
}
