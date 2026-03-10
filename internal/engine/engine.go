package engine

import (
	"context"
	"log/slog"
	"time"
)

// Processor is the interface for engine tick modules.
type Processor interface {
	Name() string
	Tick(ctx context.Context) error
}

// Engine runs processors on a fixed interval tick loop.
type Engine struct {
	processors []Processor
	interval   time.Duration
}

func New(interval time.Duration, processors ...Processor) *Engine {
	return &Engine{processors: processors, interval: interval}
}

// Start begins the tick loop in a goroutine. It stops when ctx is cancelled.
func (e *Engine) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(e.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("engine stopped")
				return
			case <-ticker.C:
				e.tick(ctx)
			}
		}
	}()
}

func (e *Engine) tick(ctx context.Context) {
	for _, p := range e.processors {
		start := time.Now()
		if err := p.Tick(ctx); err != nil {
			slog.Error("processor tick failed", "processor", p.Name(), "error", err, "duration", time.Since(start))
		} else {
			slog.Debug("processor tick", "processor", p.Name(), "duration", time.Since(start))
		}
	}
}
