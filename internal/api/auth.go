package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(userContextKey).(*store.User)
	return u
}

// SessionOrTokenAuth middleware authenticates requests via:
//  1. Session cookie (for browser UI)
//  2. Bearer token — per-user API tokens (pf_ prefix)
//
// This supports both interactive and programmatic access.
func SessionOrTokenAuth(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Try session cookie
			if user := authSvc.CurrentUser(r.Context()); user != nil {
				ctx := context.WithValue(r.Context(), userContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 2. Try Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == authHeader {
					writeError(w, http.StatusUnauthorized, "invalid authorization format, use Bearer token")
					return
				}

				// 2a. Try per-user API token (pf_ prefix)
				user, _, err := authSvc.ValidateAPIToken(r.Context(), token)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "invalid token")
					return
				}
				ctx := context.WithValue(r.Context(), userContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			writeError(w, http.StatusUnauthorized, "authentication required")
		})
	}
}

// RequireRole middleware ensures the authenticated user has one of the given roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

// RequireAdminForWrites allows GET/HEAD for any authenticated user but
// requires admin role for all other HTTP methods (POST, PUT, DELETE, etc.).
func RequireAdminForWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			user := UserFromContext(r.Context())
			if user == nil || user.Role != store.RoleAdmin {
				writeError(w, http.StatusForbidden, "admin access required")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// Periodic cleanup of stale keys to prevent unbounded memory growth.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for key, entries := range rl.requests {
				hasRecent := false
				for _, t := range entries {
					if t.After(cutoff) {
						hasRecent = true
						break
					}
				}
				if !hasRecent {
					delete(rl.requests, key)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

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
// Uses r.RemoteAddr only. Do not trust X-Forwarded-For or X-Real-IP headers
// as they can be spoofed by clients. In production behind a reverse proxy,
// configure the proxy to set RemoteAddr correctly (e.g. PROXY protocol) or
// use a trusted-proxy-aware middleware upstream.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if !limiter.Allow(ip) {
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
