package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"io/fs"

	"github.com/pagefire/pagefire/internal/api"
	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/engine"
	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/notification/providers"
	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/sqlite"
	"github.com/pagefire/pagefire/web"
)

// App holds all application dependencies and manages lifecycle.
type App struct {
	Config     *Config
	Store      store.Store
	Engine     *engine.Engine
	Dispatcher *notification.Dispatcher
	Server     *http.Server
}

// New creates a new App from the given config, wiring all dependencies.
func New(cfg *Config) (*App, error) {
	// Configure logging
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Open store
	var s store.Store
	var sqliteStore *sqlite.SQLiteStore
	var err error
	switch cfg.DatabaseDriver {
	case "sqlite":
		sqliteStore, err = sqlite.New(cfg.DatabaseURL)
		s = sqliteStore
	case "postgres":
		return nil, fmt.Errorf("postgres support not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DatabaseDriver)
	}
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Run migrations
	if err := s.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	// Auth service
	authSvc := auth.NewService(s.Users(), sqliteStore.DB())

	// Notification dispatcher
	dispatcher := notification.NewDispatcher()
	dispatcher.Register(providers.NewWebhook(cfg.AllowPrivateWebhooks))
	if cfg.SMTP.Host != "" {
		dispatcher.Register(providers.NewEmail(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From, cfg.SMTP.Username, cfg.SMTP.Password))
	}
	if cfg.Slack.BotToken != "" {
		dispatcher.Register(providers.NewSlack(cfg.Slack.BotToken))
	}

	// On-call resolver
	resolver := oncall.NewResolver(s.Schedules(), s.Users())

	// Engine processors
	interval := time.Duration(cfg.Engine.IntervalSeconds) * time.Second
	eng := engine.New(interval,
		engine.NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver),
		engine.NewNotificationProcessor(s.Notifications(), s.Users(), dispatcher),
		engine.NewCleanupProcessor(s),
	)

	// Embedded frontend assets
	frontendAssets, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		return nil, fmt.Errorf("loading frontend assets: %w", err)
	}

	// HTTP server
	router := api.NewRouter(s, resolver, dispatcher, authSvc, cfg.AdminToken, frontendAssets)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &App{
		Config:     cfg,
		Store:      s,
		Engine:     eng,
		Dispatcher: dispatcher,
		Server:     srv,
	}, nil
}

// Run starts the engine and HTTP server, blocking until shutdown signal.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start engine
	a.Engine.Start(ctx)
	slog.Info("engine started", "interval", a.Config.Engine.IntervalSeconds)

	// Start HTTP server
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", a.Server.Addr)
		if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		slog.Info("shutting down...")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := a.Server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	if err := a.Store.Close(); err != nil {
		slog.Error("store close error", "error", err)
	}

	slog.Info("shutdown complete")
	return nil
}
