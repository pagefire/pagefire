package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

func makeTestNotification(alertID string) *store.Notification {
	return &store.Notification{
		AlertID:         alertID,
		UserID:          "user-1",
		ContactMethodID: "cm-1",
		Type:            "alert",
		DestinationType: "email",
		Destination:     "alice@example.com",
		Subject:         "Alert fired",
		Body:            "Something broke",
	}
}

func TestNotification_EnqueueAndFindPending(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	n := makeTestNotification("alert-1")
	if err := ns.Enqueue(ctx, n); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if n.ID == "" {
		t.Fatal("expected ID to be set after Enqueue")
	}
	if n.Status != store.NotificationStatusPending {
		t.Errorf("Status = %q, want %q", n.Status, store.NotificationStatusPending)
	}
	if n.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", n.MaxAttempts)
	}

	pending, err := ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("FindPending: got %d, want 1", len(pending))
	}
	if pending[0].ID != n.ID {
		t.Errorf("pending ID = %q, want %q", pending[0].ID, n.ID)
	}
	if pending[0].Subject != "Alert fired" {
		t.Errorf("Subject = %q, want %q", pending[0].Subject, "Alert fired")
	}
}

func TestNotification_MarkSending(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	n := makeTestNotification("alert-1")
	if err := ns.Enqueue(ctx, n); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := ns.MarkSending(ctx, n.ID); err != nil {
		t.Fatalf("MarkSending: %v", err)
	}

	// Should no longer appear in FindPending.
	pending, err := ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("FindPending after MarkSending: got %d, want 0", len(pending))
	}
}

func TestNotification_MarkSent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	n := makeTestNotification("alert-1")
	if err := ns.Enqueue(ctx, n); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := ns.MarkSent(ctx, n.ID, "twilio-msg-123"); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	// Should no longer appear in FindPending.
	pending, err := ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("FindPending after MarkSent: got %d, want 0", len(pending))
	}
}

func TestNotification_MarkFailed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	n := makeTestNotification("alert-1")
	if err := ns.Enqueue(ctx, n); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := ns.MarkFailed(ctx, n.ID); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	// Should no longer appear in FindPending (status is failed, not pending).
	pending, err := ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("FindPending after MarkFailed: got %d, want 0", len(pending))
	}
}

func TestNotification_IncrementAttempts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	n := makeTestNotification("alert-1")
	if err := ns.Enqueue(ctx, n); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Mark as sending first, then increment attempts to reset to pending.
	if err := ns.MarkSending(ctx, n.ID); err != nil {
		t.Fatalf("MarkSending: %v", err)
	}

	nextAt := time.Now().Add(5 * time.Minute)
	if err := ns.IncrementAttempts(ctx, n.ID, nextAt); err != nil {
		t.Fatalf("IncrementAttempts: %v", err)
	}

	// The notification should be pending again but with next_attempt_at in the future,
	// so FindPending (which checks next_attempt_at <= now) should not return it yet.
	pending, err := ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("FindPending after IncrementAttempts (future next_attempt_at): got %d, want 0", len(pending))
	}

	// Use a past next_attempt_at and verify it shows up.
	pastAt := time.Now().Add(-1 * time.Minute)
	if err := ns.IncrementAttempts(ctx, n.ID, pastAt); err != nil {
		t.Fatalf("IncrementAttempts (past): %v", err)
	}

	pending, err = ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("FindPending after IncrementAttempts (past): got %d, want 1", len(pending))
	}
	// Attempts should be 2 (incremented twice).
	if pending[0].Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", pending[0].Attempts)
	}
	if pending[0].Status != store.NotificationStatusPending {
		t.Errorf("Status = %q, want %q", pending[0].Status, store.NotificationStatusPending)
	}
}

func TestNotification_FindPendingRespectsLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	for i := 0; i < 5; i++ {
		n := makeTestNotification("alert-1")
		if err := ns.Enqueue(ctx, n); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	pending, err := ns.FindPending(ctx, 3)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("FindPending with limit 3: got %d, want 3", len(pending))
	}
}

func TestNotification_FindPendingRespectsNextAttemptAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ns := s.Notifications()

	// Notification with next_attempt_at in the future should not appear.
	future := time.Now().Add(1 * time.Hour)
	n := makeTestNotification("alert-1")
	n.NextAttemptAt = &future
	if err := ns.Enqueue(ctx, n); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	pending, err := ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("FindPending with future next_attempt_at: got %d, want 0", len(pending))
	}

	// Notification with next_attempt_at in the past should appear.
	past := time.Now().Add(-1 * time.Minute)
	n2 := makeTestNotification("alert-2")
	n2.NextAttemptAt = &past
	if err := ns.Enqueue(ctx, n2); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	pending, err = ns.FindPending(ctx, 10)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("FindPending with past next_attempt_at: got %d, want 1", len(pending))
	}
	if pending[0].ID != n2.ID {
		t.Errorf("pending ID = %q, want %q", pending[0].ID, n2.ID)
	}
}
