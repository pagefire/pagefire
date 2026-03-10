package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/sqlite"
)

func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEscalationProcessor_TickNoPendingAlerts(t *testing.T) {
	s := newTestStore(t)
	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(context.Background()); err != nil {
		t.Fatalf("Tick with no pending alerts should not error, got: %v", err)
	}
}

func TestEscalationProcessor_TickProcessesPendingAlert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create user
	user := &store.User{Name: "alice", Email: "alice@example.com", Role: "user", Timezone: "UTC"}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	// Create contact method
	cm := &store.ContactMethod{UserID: user.ID, Type: "email", Value: "alice@example.com", Verified: true}
	if err := s.Users().CreateContactMethod(ctx, cm); err != nil {
		t.Fatal(err)
	}

	// Create notification rule
	nr := &store.NotificationRule{UserID: user.ID, ContactMethodID: cm.ID, DelayMinutes: 0}
	if err := s.Users().CreateNotificationRule(ctx, nr); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	// Build escalation snapshot with one step targeting the user
	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 5,
				Targets: []store.TargetSnapshot{
					{TargetType: store.TargetTypeUser, TargetID: user.ID, TargetName: "alice"},
				},
			},
		},
	}

	// Create alert at step 0 with next_escalation_at in the past
	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "test alert",
		Details:                  "details",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	// Run escalation tick
	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	// Verify escalation step advanced
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1, got %d", updated.EscalationStep)
	}

	// Verify notification enqueued
	pending, err := s.Notifications().FindPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending notification, got %d", len(pending))
	}
	if pending[0].AlertID != alert.ID {
		t.Errorf("notification alert_id=%s, want %s", pending[0].AlertID, alert.ID)
	}
	if pending[0].Destination != "alice@example.com" {
		t.Errorf("notification destination=%s, want alice@example.com", pending[0].Destination)
	}
}

func TestEscalationProcessor_EscalationExhausted(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	// Snapshot with one step, repeat=0
	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{StepNumber: 0, DelayMinutes: 5, Targets: nil},
		},
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "exhausted alert",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	// Advance to step 1 (past the only step), loop_count=0 which equals repeat=0
	past := time.Now().Add(-time.Minute)
	if err := s.Alerts().UpdateEscalationStep(ctx, alert.ID, 1, 0, past); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	// Verify step did NOT advance further
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step to remain 1, got %d", updated.EscalationStep)
	}
	if updated.LoopCount != 0 {
		t.Errorf("expected loop_count to remain 0, got %d", updated.LoopCount)
	}
	if updated.NextEscalationAt != nil {
		t.Errorf("expected next_escalation_at to be nil (cleared) after exhaustion, got %v", updated.NextEscalationAt)
	}
}

func TestEscalationProcessor_EscalationLoops(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	// Snapshot with one step, repeat=1 (allows one loop)
	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     1,
		Steps: []store.EscalationStepSnapshot{
			{StepNumber: 0, DelayMinutes: 5, Targets: nil},
		},
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "looping alert",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	// Set alert at step 1 (past last step), loop_count=0 (hasn't looped yet)
	past := time.Now().Add(-time.Minute)
	if err := s.Alerts().UpdateEscalationStep(ctx, alert.ID, 1, 0, past); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	// Verify it looped back to step 0 and incremented loop_count
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	// After processing step 0 again, escalation_step should be 1 (advanced from 0)
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1 after looping, got %d", updated.EscalationStep)
	}
	if updated.LoopCount != 1 {
		t.Errorf("expected loop_count=1, got %d", updated.LoopCount)
	}
}
