package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

// CleanupProcessor purges old resolved data. Runs actual cleanup every ~1 hour
// (720 ticks at 5s interval) to avoid expensive queries on every tick.
type CleanupProcessor struct {
	store     store.Store
	tickCount int
}

func NewCleanupProcessor(s store.Store) *CleanupProcessor {
	return &CleanupProcessor{store: s}
}

func (p *CleanupProcessor) Name() string { return "cleanup" }

func (p *CleanupProcessor) Tick(ctx context.Context) error {
	p.tickCount++
	if p.tickCount < 720 {
		return nil
	}
	p.tickCount = 0

	now := time.Now().UTC()
	alertCutoff := now.Add(-90 * 24 * time.Hour)
	notifCutoff := now.Add(-30 * 24 * time.Hour)

	alertsPurged, err := p.store.PurgeResolvedAlerts(ctx, alertCutoff)
	if err != nil {
		slog.Error("cleanup: failed to purge resolved alerts", "error", err)
	} else if alertsPurged > 0 {
		slog.Info("cleanup: purged resolved alerts", "count", alertsPurged, "older_than", alertCutoff)
	}

	notifsPurged, err := p.store.PurgeOldNotifications(ctx, notifCutoff)
	if err != nil {
		slog.Error("cleanup: failed to purge old notifications", "error", err)
	} else if notifsPurged > 0 {
		slog.Info("cleanup: purged old notifications", "count", notifsPurged, "older_than", notifCutoff)
	}

	overridesPurged, err := p.store.PurgeExpiredOverrides(ctx)
	if err != nil {
		slog.Error("cleanup: failed to purge expired overrides", "error", err)
	} else if overridesPurged > 0 {
		slog.Info("cleanup: purged expired overrides", "count", overridesPurged)
	}

	return nil
}
