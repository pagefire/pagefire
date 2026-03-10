package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// okHandler is a simple handler that returns 200 OK for testing middleware.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

// ---------- APITokenAuth ----------

func TestAPITokenAuth_NoAuthorizationHeader(t *testing.T) {
	handler := APITokenAuth("secret-token")(okHandler)

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

func TestAPITokenAuth_InvalidFormat(t *testing.T) {
	handler := APITokenAuth("secret-token")(okHandler)

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

			var body map[string]string
			json.NewDecoder(rr.Body).Decode(&body)
			if body["error"] != "invalid authorization format, use Bearer token" {
				t.Errorf("error = %q, want %q", body["error"], "invalid authorization format, use Bearer token")
			}
		})
	}
}

func TestAPITokenAuth_WrongToken(t *testing.T) {
	handler := APITokenAuth("secret-token")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "invalid token" {
		t.Errorf("error = %q, want %q", body["error"], "invalid token")
	}
}

func TestAPITokenAuth_CorrectToken(t *testing.T) {
	handler := APITokenAuth("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			t.Fatal("expected user in context, got nil")
		}
		if user.ID != "admin" {
			t.Errorf("user.ID = %q, want %q", user.ID, "admin")
		}
		if user.Name != "Admin" {
			t.Errorf("user.Name = %q, want %q", user.Name, "Admin")
		}
		if user.Role != "admin" {
			t.Errorf("user.Role = %q, want %q", user.Role, "admin")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestAPITokenAuth_EmptyAdminToken(t *testing.T) {
	handler := APITokenAuth("")(okHandler)

	tests := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"empty bearer", "Bearer "},
		{"some token", "Bearer some-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
			}
		})
	}
}

// ---------- SecurityHeaders ----------

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "0",
		"Content-Security-Policy": "default-src 'none'",
		"Referrer-Policy":        "no-referrer",
	}

	for header, want := range expected {
		got := rr.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	// Verify the next handler was called
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

	// Exhaust the limit
	for i := 0; i < 2; i++ {
		if !rl.Allow("key1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow("key1") {
		t.Fatal("should be rate limited")
	}

	// Wait for window to pass
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

	// First request should pass
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want %d", rr1.Code, http.StatusOK)
	}

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "1.2.3.4:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}

	var body map[string]string
	json.NewDecoder(rr2.Body).Decode(&body)
	if body["error"] != "rate limit exceeded" {
		t.Errorf("error = %q, want %q", body["error"], "rate limit exceeded")
	}
}

func TestRateLimitMiddleware_UsesXRealIP(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	handler := RateLimitMiddleware(rl)(okHandler)

	// First request with X-Real-IP
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	req1.Header.Set("X-Real-IP", "10.0.0.1")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want %d", rr1.Code, http.StatusOK)
	}

	// Second request with same X-Real-IP but different RemoteAddr
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.1:5678"
	req2.Header.Set("X-Real-IP", "10.0.0.1")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want %d (X-Real-IP should be used as key)", rr2.Code, http.StatusTooManyRequests)
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
