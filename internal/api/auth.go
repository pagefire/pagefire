package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(userContextKey).(*store.User)
	return u
}

// APITokenAuth middleware validates Bearer tokens against stored API tokens.
// For v0.1: uses a single admin token from config. Will be replaced with
// per-user API tokens stored in the database.
func APITokenAuth(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reject all requests if no admin token is configured
			if adminToken == "" {
				writeError(w, http.StatusUnauthorized, "authentication not configured")
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if token == auth {
				writeError(w, http.StatusUnauthorized, "invalid authorization format, use Bearer token")
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			// For v0.1 with single admin token, inject a synthetic admin user
			adminUser := &store.User{
				ID:   "admin",
				Name: "Admin",
				Role: "admin",
			}
			ctx := context.WithValue(r.Context(), userContextKey, adminUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityHeaders adds standard security response headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// RateLimiter provides simple per-key rate limiting using a sliding window.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Prune old entries
	entries := rl.requests[key]
	valid := entries[:0]
	for _, t := range entries {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}

	rl.requests[key] = append(valid, now)
	return true
}

// RateLimitMiddleware rate-limits by remote IP.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Real-IP"); forwarded != "" {
				ip = forwarded
			}
			if !limiter.Allow(ip) {
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
