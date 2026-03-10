package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/store"
)

// NotificationProcessor drains the notification queue and dispatches via providers.
type NotificationProcessor struct {
	notifications store.NotificationStore
	dispatcher    *notification.Dispatcher
}

func NewNotificationProcessor(notifications store.NotificationStore, dispatcher *notification.Dispatcher) *NotificationProcessor {
	return &NotificationProcessor{notifications: notifications, dispatcher: dispatcher}
}

func (p *NotificationProcessor) Name() string { return "notification" }

func (p *NotificationProcessor) Tick(ctx context.Context) error {
	pending, err := p.notifications.FindPending(ctx, 50)
	if err != nil {
		return err
	}

	for _, n := range pending {
		if err := p.notifications.MarkSending(ctx, n.ID); err != nil {
			slog.Error("mark sending failed", "notification_id", n.ID, "error", err)
			continue
		}

		providerID, err := p.dispatcher.Dispatch(ctx, n)
		if err != nil {
			slog.Error("dispatch failed", "notification_id", n.ID, "type", n.DestinationType, "error", err)

			if n.Attempts+1 >= n.MaxAttempts {
				if markErr := p.notifications.MarkFailed(ctx, n.ID); markErr != nil {
					slog.Error("mark failed error", "notification_id", n.ID, "error", markErr)
				}
			} else {
				// Exponential backoff: 30s, 2m, 10m
				backoffs := []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute}
				backoff := backoffs[0]
				if n.Attempts < len(backoffs) {
					backoff = backoffs[n.Attempts]
				}
				nextAt := time.Now().Add(backoff)
				if retryErr := p.notifications.IncrementAttempts(ctx, n.ID, nextAt); retryErr != nil {
					slog.Error("increment attempts error", "notification_id", n.ID, "error", retryErr)
				}
			}
			continue
		}

		if err := p.notifications.MarkSent(ctx, n.ID, providerID); err != nil {
			slog.Error("mark sent failed", "notification_id", n.ID, "error", err)
		}
	}

	return nil
}
