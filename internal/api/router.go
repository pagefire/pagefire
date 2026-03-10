package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
)

func NewRouter(s store.Store, resolver *oncall.Resolver, dispatcher *notification.Dispatcher, adminToken string) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(SecurityHeaders)
	r.Use(requestLogger)
	r.Use(chimw.Recoverer)

	// Health check (no auth)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

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
		r.Use(APITokenAuth(adminToken))

		r.Route("/api/v1", func(r chi.Router) {
			r.Mount("/users", NewUserHandler(s.Users()).Routes())
			r.Mount("/teams", NewTeamHandler(s.Teams()).Routes())
			r.Mount("/services", NewServiceHandler(s.Services()).Routes())
			r.Mount("/escalation-policies", NewEscalationPolicyHandler(s.EscalationPolicies()).Routes())
			r.Mount("/schedules", NewScheduleHandler(s.Schedules()).Routes())
			r.Mount("/alerts", NewAlertHandler(s.Alerts(), s.Services(), s.EscalationPolicies()).Routes())
			r.Mount("/incidents", NewIncidentHandler(s.Incidents()).Routes())
			r.Mount("/oncall", NewOnCallHandler(resolver).Routes())
		})
	})

	return r
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
