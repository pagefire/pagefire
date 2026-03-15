package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
)

// fakeProvider implements notification.Provider, records every message it
// receives, and always returns success.
type fakeProvider struct {
	mu       sync.Mutex
	messages []notification.Message
}

func (f *fakeProvider) Type() string { return "webhook" }

func (f *fakeProvider) Send(_ context.Context, msg notification.Message) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msg)
	return "fake-provider-id", nil
}

func (f *fakeProvider) ValidateTarget(_ string) error { return nil }

func (f *fakeProvider) Messages() []notification.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]notification.Message, len(f.messages))
	copy(out, f.messages)
	return out
}

// TestIntegration_AlertEscalationNotification exercises the full
// alert -> escalation -> notification flow end-to-end against a real
// in-memory SQLite store.
func TestIntegration_AlertEscalationNotification(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// ---------------------------------------------------------------
	// 1. Create a user with a webhook contact method and a notification rule.
	// ---------------------------------------------------------------
	user := &store.User{Name: "oncall-alice", Email: "alice@example.com", Role: "user", Timezone: "UTC"}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	cm := &store.ContactMethod{UserID: user.ID, Type: "webhook", Value: "https://example.com/hook", Verified: true}
	if err := s.Users().CreateContactMethod(ctx, cm); err != nil {
		t.Fatalf("create contact method: %v", err)
	}

	nr := &store.NotificationRule{UserID: user.ID, ContactMethodID: cm.ID, DelayMinutes: 0}
	if err := s.Users().CreateNotificationRule(ctx, nr); err != nil {
		t.Fatalf("create notification rule: %v", err)
	}

	// ---------------------------------------------------------------
	// 2. Create a team and add the user as a member.
	// ---------------------------------------------------------------
	team := &store.Team{Name: "platform"}
	if err := s.Teams().Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := s.Teams().AddMember(ctx, team.ID, user.ID, "member"); err != nil {
		t.Fatalf("add team member: %v", err)
	}

	// ---------------------------------------------------------------
	// 3. Create a service with an escalation policy snapshot that
	//    targets the user directly. (The escalation processor works
	//    off the snapshot JSON embedded in the alert, not live DB
	//    escalation policy rows, so we build the snapshot inline.)
	// ---------------------------------------------------------------
	svc := &store.Service{Name: "api-gateway", EscalationPolicyID: "ep-test"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatalf("create service: %v", err)
	}

	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-test",
		PolicyName: "default-policy",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 0,
				Targets: []store.TargetSnapshot{
					{TargetType: store.TargetTypeUser, TargetID: user.ID, TargetName: user.Name},
				},
			},
		},
	}

	// ---------------------------------------------------------------
	// 4. Create an alert on the service. Create() automatically sets
	//    next_escalation_at = now, so it will be picked up immediately.
	// ---------------------------------------------------------------
	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "CPU > 90% on api-gateway",
		Details:                  "CPU utilization has exceeded 90% for 5 minutes.",
		Source:                   "monitor",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatalf("create alert: %v", err)
	}

	// Sanity-check: alert should be in triggered status.
	got, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get alert: %v", err)
	}
	if got.Status != store.AlertStatusTriggered {
		t.Fatalf("expected alert status=%s, got %s", store.AlertStatusTriggered, got.Status)
	}

	// ---------------------------------------------------------------
	// 5. Run the EscalationProcessor — it should find the alert,
	//    resolve the user target, and enqueue a notification.
	// ---------------------------------------------------------------
	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	escProc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := escProc.Tick(ctx); err != nil {
		t.Fatalf("escalation Tick: %v", err)
	}

	// ---------------------------------------------------------------
	// 6. Verify a notification record was created in the DB.
	// ---------------------------------------------------------------
	pending, err := s.Notifications().FindPending(ctx, 50)
	if err != nil {
		t.Fatalf("find pending notifications: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending notification, got %d", len(pending))
	}

	n := pending[0]
	if n.AlertID != alert.ID {
		t.Errorf("notification alert_id: got %s, want %s", n.AlertID, alert.ID)
	}
	if n.UserID != user.ID {
		t.Errorf("notification user_id: got %s, want %s", n.UserID, user.ID)
	}
	if n.DestinationType != "webhook" {
		t.Errorf("notification destination_type: got %s, want webhook", n.DestinationType)
	}
	if n.Destination != "https://example.com/hook" {
		t.Errorf("notification destination: got %s, want https://example.com/hook", n.Destination)
	}
	if n.Subject != alert.Summary {
		t.Errorf("notification subject: got %q, want %q", n.Subject, alert.Summary)
	}
	if n.Body != alert.Details {
		t.Errorf("notification body: got %q, want %q", n.Body, alert.Details)
	}
	if n.Status != store.NotificationStatusPending {
		t.Errorf("notification status: got %s, want %s", n.Status, store.NotificationStatusPending)
	}

	// ---------------------------------------------------------------
	// 7. Run the NotificationProcessor with the fake webhook provider.
	// ---------------------------------------------------------------
	fake := &fakeProvider{}
	dispatcher := notification.NewDispatcher()
	dispatcher.Register(fake)

	notifProc := NewNotificationProcessor(s.Notifications(), s.Users(), dispatcher)

	if err := notifProc.Tick(ctx); err != nil {
		t.Fatalf("notification Tick: %v", err)
	}

	// ---------------------------------------------------------------
	// 8. Verify no more pending notifications (should be marked sent).
	// ---------------------------------------------------------------
	remaining, err := s.Notifications().FindPending(ctx, 50)
	if err != nil {
		t.Fatalf("find pending after dispatch: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 pending notifications after dispatch, got %d", len(remaining))
	}

	// ---------------------------------------------------------------
	// 9. Verify the fake provider received exactly one message with
	//    the correct To, Subject, and Body.
	// ---------------------------------------------------------------
	msgs := fake.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected fake provider to receive 1 message, got %d", len(msgs))
	}

	msg := msgs[0]
	if msg.To != "https://example.com/hook" {
		t.Errorf("provider message To: got %q, want %q", msg.To, "https://example.com/hook")
	}
	if msg.Subject != "CPU > 90% on api-gateway" {
		t.Errorf("provider message Subject: got %q, want %q", msg.Subject, "CPU > 90% on api-gateway")
	}
	if msg.Body != "CPU utilization has exceeded 90% for 5 minutes." {
		t.Errorf("provider message Body: got %q, want %q", msg.Body, "CPU utilization has exceeded 90% for 5 minutes.")
	}
	if msg.AlertID != alert.ID {
		t.Errorf("provider message AlertID: got %q, want %q", msg.AlertID, alert.ID)
	}
	if msg.UserID != user.ID {
		t.Errorf("provider message UserID: got %q, want %q", msg.UserID, user.ID)
	}
	if msg.UserName != "oncall-alice" {
		t.Errorf("provider message UserName: got %q, want %q", msg.UserName, "oncall-alice")
	}

	// ---------------------------------------------------------------
	// 10. Verify escalation step advanced on the alert.
	// ---------------------------------------------------------------
	finalAlert, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get alert after escalation: %v", err)
	}
	if finalAlert.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1, got %d", finalAlert.EscalationStep)
	}
}
