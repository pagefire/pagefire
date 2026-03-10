package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
)

// EscalationProcessor advances alerts through escalation steps.
type EscalationProcessor struct {
	alerts        store.AlertStore
	notifications store.NotificationStore
	users         store.UserStore
	resolver      *oncall.Resolver
}

func NewEscalationProcessor(alerts store.AlertStore, notifications store.NotificationStore, users store.UserStore, resolver *oncall.Resolver) *EscalationProcessor {
	return &EscalationProcessor{alerts: alerts, notifications: notifications, users: users, resolver: resolver}
}

func (p *EscalationProcessor) Name() string { return "escalation" }

func (p *EscalationProcessor) Tick(ctx context.Context) error {
	alerts, err := p.alerts.FindPendingEscalations(ctx, time.Now())
	if err != nil {
		return err
	}

	for _, alert := range alerts {
		if err := p.processAlert(ctx, alert); err != nil {
			slog.Error("escalation failed", "alert_id", alert.ID, "error", err)
		}
	}
	return nil
}

func (p *EscalationProcessor) processAlert(ctx context.Context, alert store.Alert) error {
	var snapshot store.EscalationSnapshot
	if err := json.Unmarshal([]byte(alert.EscalationPolicySnapshot), &snapshot); err != nil {
		return err
	}

	if len(snapshot.Steps) == 0 {
		return nil
	}

	step := alert.EscalationStep
	loopCount := alert.LoopCount

	// Check if we've exhausted all steps
	if step >= len(snapshot.Steps) {
		if loopCount < snapshot.Repeat {
			step = 0
			loopCount++
		} else {
			slog.Info("escalation exhausted", "alert_id", alert.ID)
			// Clear next_escalation_at so this alert stops being picked up
			return p.alerts.UpdateEscalationStep(ctx, alert.ID, step, loopCount, time.Time{})
		}
	}

	currentStep := snapshot.Steps[step]

	// Resolve targets to users
	var targetUsers []store.User
	for _, target := range currentStep.Targets {
		switch target.TargetType {
		case store.TargetTypeSchedule:
			users, err := p.resolver.Resolve(ctx, target.TargetID, time.Now())
			if err != nil {
				slog.Error("resolve schedule failed", "schedule_id", target.TargetID, "error", err)
				continue
			}
			targetUsers = append(targetUsers, users...)
		case store.TargetTypeUser:
			user, err := p.users.Get(ctx, target.TargetID)
			if err != nil {
				slog.Error("get user failed", "user_id", target.TargetID, "error", err)
				continue
			}
			targetUsers = append(targetUsers, *user)
		}
	}

	slog.Info("escalation processing", "alert_id", alert.ID, "step", step, "target_users", len(targetUsers))

	// Queue notifications for each user based on their notification rules
	for _, user := range targetUsers {
		rules, err := p.users.ListNotificationRules(ctx, user.ID)
		if err != nil {
			slog.Error("list notification rules failed", "user_id", user.ID, "error", err)
			continue
		}

		for _, rule := range rules {
			methods, err := p.users.ListContactMethods(ctx, user.ID)
			if err != nil {
				continue
			}

			// Find the contact method for this rule
			for _, cm := range methods {
				if cm.ID == rule.ContactMethodID {
					nextAt := time.Now().Add(time.Duration(rule.DelayMinutes) * time.Minute)
					n := &store.Notification{
						AlertID:         alert.ID,
						UserID:          user.ID,
						UserName:        user.Name,
						ContactMethodID: cm.ID,
						Type:            "alert",
						DestinationType: cm.Type,
						Destination:     cm.Value,
						Subject:         alert.Summary,
						Body:            alert.Details,
						NextAttemptAt:   &nextAt,
					}
					if err := p.notifications.Enqueue(ctx, n); err != nil {
						slog.Error("enqueue notification failed", "user_id", user.ID, "error", err)
					}
					break
				}
			}
		}
	}

	// Advance to next step
	nextStep := step + 1
	nextAt := time.Now().Add(time.Duration(currentStep.DelayMinutes) * time.Minute)
	return p.alerts.UpdateEscalationStep(ctx, alert.ID, nextStep, loopCount, nextAt)
}
