package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

func TestCreateAndGetAlert(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{
		ServiceID:                svc.ID,
		Summary:                  "disk full",
		Details:                  "root partition at 99%",
		Source:                   "monitor",
		DeduplicationKey:         "disk-full-1",
		EscalationPolicySnapshot: `{"policy_id":"ep-1"}`,
	}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected alert ID to be set after Create")
	}

	got, err := alerts.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != a.ID {
		t.Errorf("ID: got %q, want %q", got.ID, a.ID)
	}
	if got.ServiceID != svc.ID {
		t.Errorf("ServiceID: got %q, want %q", got.ServiceID, svc.ID)
	}
	if got.Status != store.AlertStatusTriggered {
		t.Errorf("Status: got %q, want %q", got.Status, store.AlertStatusTriggered)
	}
	if got.Summary != "disk full" {
		t.Errorf("Summary: got %q, want %q", got.Summary, "disk full")
	}
	if got.Details != "root partition at 99%" {
		t.Errorf("Details: got %q, want %q", got.Details, "root partition at 99%")
	}
	if got.Source != "monitor" {
		t.Errorf("Source: got %q, want %q", got.Source, "monitor")
	}
	if got.DeduplicationKey != "disk-full-1" {
		t.Errorf("DeduplicationKey: got %q, want %q", got.DeduplicationKey, "disk-full-1")
	}
	if got.EscalationPolicySnapshot != `{"policy_id":"ep-1"}` {
		t.Errorf("EscalationPolicySnapshot: got %q", got.EscalationPolicySnapshot)
	}
	if got.EscalationStep != 0 {
		t.Errorf("EscalationStep: got %d, want 0", got.EscalationStep)
	}
	if got.LoopCount != 0 {
		t.Errorf("LoopCount: got %d, want 0", got.LoopCount)
	}
	if got.NextEscalationAt == nil {
		t.Error("NextEscalationAt: expected non-nil after Create")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt: expected non-zero")
	}
}

func TestCreateDuplicateKey(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a1 := &store.Alert{
		ServiceID:        svc.ID,
		Summary:          "alert-1",
		Source:           "api",
		DeduplicationKey: "dedup-1",
	}
	if err := alerts.Create(ctx, a1); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	originalID := a1.ID

	a2 := &store.Alert{
		ServiceID:        svc.ID,
		Summary:          "alert-2",
		Source:           "api",
		DeduplicationKey: "dedup-1",
	}
	err := alerts.Create(ctx, a2)
	if !errors.Is(err, store.ErrDuplicateKey) {
		t.Fatalf("expected ErrDuplicateKey, got %v", err)
	}
	if a2.ID != originalID {
		t.Errorf("expected duplicate to return existing ID %q, got %q", originalID, a2.ID)
	}
}

func TestDedupOnlyAppliesToNonResolved(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a1 := &store.Alert{
		ServiceID:        svc.ID,
		Summary:          "alert-1",
		Source:           "api",
		DeduplicationKey: "dedup-resolve",
	}
	if err := alerts.Create(ctx, a1); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	firstID := a1.ID

	if err := alerts.Resolve(ctx, a1.ID); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	a2 := &store.Alert{
		ServiceID:        svc.ID,
		Summary:          "alert-2",
		Source:           "api",
		DeduplicationKey: "dedup-resolve",
	}
	if err := alerts.Create(ctx, a2); err != nil {
		t.Fatalf("expected Create to succeed after resolving first alert, got %v", err)
	}
	if a2.ID == firstID {
		t.Error("expected new alert to have a different ID from the resolved one")
	}
}

func TestListNoFilter(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		a := &store.Alert{ServiceID: svc.ID, Summary: "alert", Source: "api"}
		if err := alerts.Create(ctx, a); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	list, err := alerts.List(ctx, store.AlertFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(list))
	}
}

func TestListWithStatusFilter(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a1 := &store.Alert{ServiceID: svc.ID, Summary: "triggered-1", Source: "api"}
	a2 := &store.Alert{ServiceID: svc.ID, Summary: "triggered-2", Source: "api"}
	if err := alerts.Create(ctx, a1); err != nil {
		t.Fatalf("Create a1: %v", err)
	}
	if err := alerts.Create(ctx, a2); err != nil {
		t.Fatalf("Create a2: %v", err)
	}
	if err := alerts.Resolve(ctx, a2.ID); err != nil {
		t.Fatalf("Resolve a2: %v", err)
	}

	triggered, err := alerts.List(ctx, store.AlertFilter{Status: store.AlertStatusTriggered})
	if err != nil {
		t.Fatalf("List triggered: %v", err)
	}
	if len(triggered) != 1 {
		t.Errorf("expected 1 triggered alert, got %d", len(triggered))
	}

	resolved, err := alerts.List(ctx, store.AlertFilter{Status: store.AlertStatusResolved})
	if err != nil {
		t.Fatalf("List resolved: %v", err)
	}
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved alert, got %d", len(resolved))
	}
}

func TestListWithServiceFilter(t *testing.T) {
	s := newTestStore(t)
	svc1 := createTestService(t, s)
	svc2 := &store.Service{Name: "test-svc-2", EscalationPolicyID: "ep-2"}
	if err := s.Services().Create(context.Background(), svc2); err != nil {
		t.Fatalf("creating second service: %v", err)
	}
	alerts := s.Alerts()
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		a := &store.Alert{ServiceID: svc1.ID, Summary: "svc1-alert", Source: "api"}
		if err := alerts.Create(ctx, a); err != nil {
			t.Fatalf("Create svc1 alert %d: %v", i, err)
		}
	}
	a := &store.Alert{ServiceID: svc2.ID, Summary: "svc2-alert", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create svc2 alert: %v", err)
	}

	list, err := alerts.List(ctx, store.AlertFilter{ServiceID: svc1.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 alerts for svc1, got %d", len(list))
	}
	for _, al := range list {
		if al.ServiceID != svc1.ID {
			t.Errorf("expected ServiceID %q, got %q", svc1.ID, al.ServiceID)
		}
	}
}

func TestListWithLimitOffset(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		a := &store.Alert{ServiceID: svc.ID, Summary: "alert", Source: "api"}
		if err := alerts.Create(ctx, a); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	// Limit only
	list, err := alerts.List(ctx, store.AlertFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List with limit: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 alerts with limit=2, got %d", len(list))
	}

	// Limit + offset
	list2, err := alerts.List(ctx, store.AlertFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List with limit+offset: %v", err)
	}
	if len(list2) != 2 {
		t.Errorf("expected 2 alerts with limit=2 offset=2, got %d", len(list2))
	}

	// Offset past end
	list3, err := alerts.List(ctx, store.AlertFilter{Limit: 10, Offset: 5})
	if err != nil {
		t.Fatalf("List with offset past end: %v", err)
	}
	if len(list3) != 0 {
		t.Errorf("expected 0 alerts with offset=5, got %d", len(list3))
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	u := createTestUser(t, s, "acker", "acker@test.com")
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{ServiceID: svc.ID, Summary: "ack-me", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := alerts.Acknowledge(ctx, a.ID, u.ID); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}

	got, err := alerts.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("Get after ack: %v", err)
	}
	if got.Status != store.AlertStatusAcknowledged {
		t.Errorf("Status: got %q, want %q", got.Status, store.AlertStatusAcknowledged)
	}
	if got.AcknowledgedBy == nil || *got.AcknowledgedBy != u.ID {
		t.Errorf("AcknowledgedBy: got %v, want %q", got.AcknowledgedBy, u.ID)
	}
	if got.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt: expected non-nil after Acknowledge")
	}
	if got.NextEscalationAt != nil {
		t.Errorf("NextEscalationAt: expected nil after Acknowledge, got %v", got.NextEscalationAt)
	}
}

func TestAcknowledgeNonExistent(t *testing.T) {
	s := newTestStore(t)
	alerts := s.Alerts()

	err := alerts.Acknowledge(context.Background(), "nonexistent-id", "user-1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAcknowledgeAlreadyAcknowledged(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	u1 := createTestUser(t, s, "user1", "u1@test.com")
	u2 := createTestUser(t, s, "user2", "u2@test.com")
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{ServiceID: svc.ID, Summary: "ack-twice", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := alerts.Acknowledge(ctx, a.ID, u1.ID); err != nil {
		t.Fatalf("first Acknowledge: %v", err)
	}

	err := alerts.Acknowledge(ctx, a.ID, u2.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second Acknowledge, got %v", err)
	}
}

func TestResolveAlert(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{ServiceID: svc.ID, Summary: "resolve-me", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := alerts.Resolve(ctx, a.ID); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	got, err := alerts.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("Get after resolve: %v", err)
	}
	if got.Status != store.AlertStatusResolved {
		t.Errorf("Status: got %q, want %q", got.Status, store.AlertStatusResolved)
	}
	if got.ResolvedAt == nil {
		t.Error("ResolvedAt: expected non-nil after Resolve")
	}
	if got.NextEscalationAt != nil {
		t.Errorf("NextEscalationAt: expected nil after Resolve, got %v", got.NextEscalationAt)
	}
}

func TestResolveAlreadyResolved(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{ServiceID: svc.ID, Summary: "resolve-twice", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := alerts.Resolve(ctx, a.ID); err != nil {
		t.Fatalf("first Resolve: %v", err)
	}

	err := alerts.Resolve(ctx, a.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second Resolve, got %v", err)
	}
}

func TestFindPendingEscalations(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	// Create three alerts: triggered, acknowledged, resolved
	a1 := &store.Alert{ServiceID: svc.ID, Summary: "triggered", Source: "api"}
	a2 := &store.Alert{ServiceID: svc.ID, Summary: "acked", Source: "api"}
	a3 := &store.Alert{ServiceID: svc.ID, Summary: "resolved", Source: "api"}
	for _, a := range []*store.Alert{a1, a2, a3} {
		if err := alerts.Create(ctx, a); err != nil {
			t.Fatalf("Create %q: %v", a.Summary, err)
		}
	}
	u := createTestUser(t, s, "acker", "acker-esc@test.com")
	if err := alerts.Acknowledge(ctx, a2.ID, u.ID); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if err := alerts.Resolve(ctx, a3.ID); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Query with a time in the future so NextEscalationAt <= before is satisfied
	future := time.Now().Add(1 * time.Minute)
	pending, err := alerts.FindPendingEscalations(ctx, future)
	if err != nil {
		t.Fatalf("FindPendingEscalations: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending escalation, got %d", len(pending))
	}
	if pending[0].ID != a1.ID {
		t.Errorf("expected pending alert ID %q, got %q", a1.ID, pending[0].ID)
	}

	// Query with a time in the past — nothing should match
	past := time.Now().Add(-1 * time.Hour)
	empty, err := alerts.FindPendingEscalations(ctx, past)
	if err != nil {
		t.Fatalf("FindPendingEscalations past: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 pending escalations with past time, got %d", len(empty))
	}
}

func TestUpdateEscalationStep(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{ServiceID: svc.ID, Summary: "escalate-me", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	nextAt := time.Now().Add(5 * time.Minute)
	if err := alerts.UpdateEscalationStep(ctx, a.ID, 2, 1, nextAt); err != nil {
		t.Fatalf("UpdateEscalationStep: %v", err)
	}

	got, err := alerts.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.EscalationStep != 2 {
		t.Errorf("EscalationStep: got %d, want 2", got.EscalationStep)
	}
	if got.LoopCount != 1 {
		t.Errorf("LoopCount: got %d, want 1", got.LoopCount)
	}
	if got.NextEscalationAt == nil {
		t.Fatal("NextEscalationAt: expected non-nil after UpdateEscalationStep")
	}
	// Allow 1 second tolerance for time comparison
	diff := got.NextEscalationAt.Sub(nextAt)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("NextEscalationAt: got %v, want ~%v", got.NextEscalationAt, nextAt)
	}
}

func TestCreateLogAndListLogs(t *testing.T) {
	s := newTestStore(t)
	svc := createTestService(t, s)
	alerts := s.Alerts()
	ctx := context.Background()

	a := &store.Alert{ServiceID: svc.ID, Summary: "log-test", Source: "api"}
	if err := alerts.Create(ctx, a); err != nil {
		t.Fatalf("Create alert: %v", err)
	}

	logs := []store.AlertLog{
		{AlertID: a.ID, Event: "created", Message: "Alert created"},
		{AlertID: a.ID, Event: "escalated", Message: "Escalated to step 1"},
		{AlertID: a.ID, Event: "acknowledged", Message: "Acknowledged"},
	}
	for i := range logs {
		if err := alerts.CreateLog(ctx, &logs[i]); err != nil {
			t.Fatalf("CreateLog %d: %v", i, err)
		}
		if logs[i].ID == "" {
			t.Fatalf("expected log ID to be set for log %d", i)
		}
	}

	got, err := alerts.ListLogs(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(got))
	}

	// Logs are ordered by created_at ASC
	for i, l := range got {
		if l.AlertID != a.ID {
			t.Errorf("log %d: AlertID got %q, want %q", i, l.AlertID, a.ID)
		}
		if l.Event != logs[i].Event {
			t.Errorf("log %d: Event got %q, want %q", i, l.Event, logs[i].Event)
		}
		if l.Message != logs[i].Message {
			t.Errorf("log %d: Message got %q, want %q", i, l.Message, logs[i].Message)
		}
		if l.CreatedAt.IsZero() {
			t.Errorf("log %d: CreatedAt is zero", i)
		}
	}
}

func TestGetNonExistentAlert(t *testing.T) {
	s := newTestStore(t)
	alerts := s.Alerts()

	_, err := alerts.Get(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
