package engine

import (
	"context"

	"github.com/pagefire/pagefire/internal/store"
)

// CleanupProcessor purges old resolved data. Runs actual cleanup every ~1 hour
// (720 ticks at 5s interval) to avoid expensive queries on every tick.
type CleanupProcessor struct {
	store    store.Store
	tickCount int
}

func NewCleanupProcessor(s store.Store) *CleanupProcessor {
	return &CleanupProcessor{store: s}
}

func (p *CleanupProcessor) Name() string { return "cleanup" }

func (p *CleanupProcessor) Tick(_ context.Context) error {
	p.tickCount++
	if p.tickCount < 720 {
		return nil
	}
	p.tickCount = 0

	// TODO: Implement cleanup of old resolved alerts (90 days),
	// sent/failed notifications (30 days), and expired overrides.
	// Deferred until we have meaningful data volume.

	return nil
}
