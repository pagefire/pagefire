package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
)

func NewRouter(s store.Store, resolver *oncall.Resolver, dispatcher *notification.Dispatcher, authSvc *auth.Service, frontendFS ...fs.FS) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(SecurityHeaders)
	r.Use(requestLogger)
	r.Use(chimw.Recoverer)

	// Session middleware (loads/saves session data on every request)
	r.Use(authSvc.SessionManager().LoadAndSave)

	// Health check (no auth)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Auth endpoints (single mount — public + protected routes handled internally)
	authHandler := NewAuthHandler(authSvc, s.Users())
	authMiddleware := SessionOrTokenAuth(authSvc)
	r.Mount("/api/v1/auth", authHandler.Routes(authMiddleware))

	// Integration webhooks (authenticated by integration key secret, rate-limited)
	integrationLimiter := NewRateLimiter(60, time.Minute)
	r.Group(func(r chi.Router) {
		r.Use(RateLimitMiddleware(integrationLimiter))
		r.Mount("/api/v1/integrations", NewIntegrationHandler(s.Services(), s.Alerts(), s.EscalationPolicies()).Routes())
	})

	// Authenticated API routes
	apiLimiter := NewRateLimiter(1000, time.Minute)
	r.Group(func(r chi.Router) {
		r.Use(RateLimitMiddleware(apiLimiter))
		r.Use(authMiddleware)

		r.Route("/api/v1", func(r chi.Router) {
			scheduleHandler := NewScheduleHandler(s.Schedules())

			// Admin-only writes, all users can read
			r.Group(func(r chi.Router) {
				r.Use(RequireAdminForWrites)
				r.Mount("/users", NewUserHandler(s.Users()).Routes())
				r.Mount("/teams", NewTeamHandler(s.Teams()).Routes())
				r.Mount("/services", NewServiceHandler(s.Services()).Routes())
				r.Mount("/escalation-policies", NewEscalationPolicyHandler(s.EscalationPolicies()).Routes())
				r.Mount("/schedules", scheduleHandler.Routes())
			})

			// All authenticated users: alerts, incidents, on-call, schedule overrides
			r.Mount("/alerts", NewAlertHandler(s.Alerts(), s.Services(), s.EscalationPolicies()).Routes())
			r.Mount("/incidents", NewIncidentHandler(s.Incidents()).Routes())
			r.Mount("/oncall", NewOnCallHandler(resolver).Routes())
			r.Mount("/schedule-overrides", scheduleHandler.OverrideRoutes())
		})
	})

	// Serve embedded frontend SPA (catch-all, must come after API routes)
	if len(frontendFS) > 0 && frontendFS[0] != nil {
		frontendHandler := spaHandler(frontendFS[0])
		r.NotFound(frontendHandler)
	}

	return r
}

// spaHandler serves static files from the embedded filesystem. If the requested
// file doesn't exist, it falls back to index.html for client-side routing.
func spaHandler(assets fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(assets))

	// Pre-read index.html for SPA fallback (avoids redirect loop)
	indexHTML, _ := fs.ReadFile(assets, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		// Relax CSP for frontend pages (override the strict API CSP)
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		}

		// Try to serve the exact file (static assets like JS, CSS)
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := assets.Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// File not found or root — serve index.html for SPA client-side routing
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start),
		)
	})
}
