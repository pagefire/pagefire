package engine

import (
	"context"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store"
)

// TestEscalationProcessor_DeletedUserTarget verifies that escalation does not
// panic when a target user has been deleted from the database. The processor
// should log an error and continue to the next target / advance the step.
func TestEscalationProcessor_DeletedUserTarget(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create a user, then delete them
	user := &store.User{Name: "ghost", Email: "ghost@example.com", Role: "user", Timezone: "UTC", IsActive: true}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	deletedUserID := user.ID
	if err := s.Users().Delete(ctx, deletedUserID); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	// Snapshot targeting the deleted user
	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 5,
				Targets: []store.TargetSnapshot{
					{TargetType: store.TargetTypeUser, TargetID: deletedUserID, TargetName: "ghost"},
				},
			},
		},
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "alert targeting deleted user",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	// This must not panic
	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick should not error when target user is deleted, got: %v", err)
	}

	// Verify step still advanced (the error is logged, not fatal)
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1, got %d", updated.EscalationStep)
	}

	// No notifications should have been enqueued (user doesn't exist)
	pending, err := s.Notifications().FindPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 notifications for deleted user, got %d", len(pending))
	}
}

// TestEscalationProcessor_UserNoContactMethods verifies that escalation
// handles a user with no contact methods gracefully — no panic, no error,
// just skips notification enqueue.
func TestEscalationProcessor_UserNoContactMethods(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create user with a contact method and notification rule, then delete
	// the contact method. This simulates a user whose contact methods were
	// removed after the notification rule was created.
	user := &store.User{Name: "nocontact", Email: "nocontact@example.com", Role: "user", Timezone: "UTC", IsActive: true}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	cm := &store.ContactMethod{UserID: user.ID, Type: "email", Value: "nocontact@example.com", Verified: true}
	if err := s.Users().CreateContactMethod(ctx, cm); err != nil {
		t.Fatal(err)
	}

	nr := &store.NotificationRule{UserID: user.ID, ContactMethodID: cm.ID, DelayMinutes: 0}
	if err := s.Users().CreateNotificationRule(ctx, nr); err != nil {
		t.Fatal(err)
	}

	// Delete the contact method so the rule now points to nothing
	if err := s.Users().DeleteContactMethod(ctx, cm.ID); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 5,
				Targets: []store.TargetSnapshot{
					{TargetType: store.TargetTypeUser, TargetID: user.ID, TargetName: "nocontact"},
				},
			},
		},
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "alert for user without contact methods",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	// Must not panic or error
	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick should not error when user has no contact methods, got: %v", err)
	}

	// Step should still advance
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1, got %d", updated.EscalationStep)
	}

	// No notifications enqueued (contact method not found)
	pending, err := s.Notifications().FindPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(pending))
	}
}

// TestEscalationProcessor_UserNoNotificationRules verifies that escalation
// handles a user with no notification rules gracefully — no notifications
// are enqueued but the escalation step still advances.
func TestEscalationProcessor_UserNoNotificationRules(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create user with no notification rules at all
	user := &store.User{Name: "norules", Email: "norules@example.com", Role: "user", Timezone: "UTC", IsActive: true}
	if err := s.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	// Create a contact method (but no rules linking to it)
	cm := &store.ContactMethod{UserID: user.ID, Type: "email", Value: "norules@example.com", Verified: true}
	if err := s.Users().CreateContactMethod(ctx, cm); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 5,
				Targets: []store.TargetSnapshot{
					{TargetType: store.TargetTypeUser, TargetID: user.ID, TargetName: "norules"},
				},
			},
		},
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "alert for user without notification rules",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	// Must not panic or error
	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick should not error when user has no notification rules, got: %v", err)
	}

	// Step should still advance
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1, got %d", updated.EscalationStep)
	}

	// No notifications enqueued (no rules)
	pending, err := s.Notifications().FindPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(pending))
	}
}

// TestEscalationProcessor_MultipleTargetsOneFails verifies that when one
// target in a step fails (deleted user), other targets still get notified.
func TestEscalationProcessor_MultipleTargetsOneFails(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create a user that will be deleted
	ghost := &store.User{Name: "ghost", Email: "ghost@example.com", Role: "user", Timezone: "UTC", IsActive: true}
	if err := s.Users().Create(ctx, ghost); err != nil {
		t.Fatal(err)
	}
	ghostID := ghost.ID
	if err := s.Users().Delete(ctx, ghostID); err != nil {
		t.Fatal(err)
	}

	// Create a valid user with contact method and notification rule
	alive := &store.User{Name: "alive", Email: "alive@example.com", Role: "user", Timezone: "UTC", IsActive: true}
	if err := s.Users().Create(ctx, alive); err != nil {
		t.Fatal(err)
	}
	cm := &store.ContactMethod{UserID: alive.ID, Type: "email", Value: "alive@example.com", Verified: true}
	if err := s.Users().CreateContactMethod(ctx, cm); err != nil {
		t.Fatal(err)
	}
	nr := &store.NotificationRule{UserID: alive.ID, ContactMethodID: cm.ID, DelayMinutes: 0}
	if err := s.Users().CreateNotificationRule(ctx, nr); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "default",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 5,
				Targets: []store.TargetSnapshot{
					{TargetType: store.TargetTypeUser, TargetID: ghostID, TargetName: "ghost"},
					{TargetType: store.TargetTypeUser, TargetID: alive.ID, TargetName: "alive"},
				},
			},
		},
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "alert with mixed targets",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	// The alive user should still have gotten a notification
	pending, err := s.Notifications().FindPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 notification (for alive user), got %d", len(pending))
	}
	if pending[0].Destination != "alive@example.com" {
		t.Errorf("notification destination = %s, want alive@example.com", pending[0].Destination)
	}
}

// TestEscalationProcessor_EmptySteps verifies escalation with zero steps
// does not panic and simply returns.
func TestEscalationProcessor_EmptySteps(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "empty",
		Repeat:     0,
		Steps:      []store.EscalationStepSnapshot{}, // no steps
	}

	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "alert with empty steps",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick should handle empty steps gracefully, got: %v", err)
	}

	// Alert should remain at step 0 (nothing to process)
	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 0 {
		t.Errorf("expected escalation_step=0, got %d", updated.EscalationStep)
	}
}

// TestEscalationProcessor_StepWithNoTargets verifies escalation advances
// even when a step has zero targets.
func TestEscalationProcessor_StepWithNoTargets(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	svc := &store.Service{Name: "web", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	snapshot := store.EscalationSnapshot{
		PolicyID:   "ep-1",
		PolicyName: "notargets",
		Repeat:     0,
		Steps: []store.EscalationStepSnapshot{
			{
				StepNumber:   0,
				DelayMinutes: 1,
				Targets:      nil, // no targets
			},
		},
	}

	past := time.Now().Add(-time.Minute)
	alert := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "alert with no targets",
		Source:                   "api",
		EscalationPolicySnapshot: mustJSON(t, snapshot),
		NextEscalationAt:         &past,
	}
	if err := s.Alerts().Create(ctx, alert); err != nil {
		t.Fatal(err)
	}

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	proc := NewEscalationProcessor(s.Alerts(), s.Notifications(), s.Users(), resolver)

	if err := proc.Tick(ctx); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	updated, err := s.Alerts().Get(ctx, alert.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EscalationStep != 1 {
		t.Errorf("expected escalation_step=1, got %d", updated.EscalationStep)
	}

	// No notifications enqueued
	pending, err := s.Notifications().FindPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(pending))
	}
}
