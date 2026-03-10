package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

func TestIncident_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()
	u := createTestUser(t, s, "alice", "alice-inc@test.com")

	inc := &store.Incident{
		Title:     "Database outage",
		Status:    store.IncidentStatusTriggered,
		Severity:  store.SeverityCritical,
		Summary:   "Primary DB is unreachable",
		Source:    "manual",
		CreatedBy: u.ID,
	}
	if err := is.Create(ctx, inc); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inc.ID == "" {
		t.Fatal("expected ID to be set after Create")
	}

	got, err := is.Get(ctx, inc.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Database outage" {
		t.Errorf("Title = %q, want %q", got.Title, "Database outage")
	}
	if got.Status != store.IncidentStatusTriggered {
		t.Errorf("Status = %q, want %q", got.Status, store.IncidentStatusTriggered)
	}
	if got.Severity != store.SeverityCritical {
		t.Errorf("Severity = %q, want %q", got.Severity, store.SeverityCritical)
	}
	if got.Summary != "Primary DB is unreachable" {
		t.Errorf("Summary = %q, want %q", got.Summary, "Primary DB is unreachable")
	}
	if got.Source != "manual" {
		t.Errorf("Source = %q, want %q", got.Source, "manual")
	}
	if got.CreatedBy != u.ID {
		t.Errorf("CreatedBy = %q, want %q", got.CreatedBy, u.ID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if got.ResolvedAt != nil {
		t.Errorf("ResolvedAt = %v, want nil", got.ResolvedAt)
	}
}

func TestIncident_GetNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.Incidents().Get(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("Get non-existent: got %v, want ErrNotFound", err)
	}
}

func TestIncident_ListNoFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	got, err := is.List(ctx, store.IncidentFilter{})
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List empty: got %d, want 0", len(got))
	}

	for _, title := range []string{"Inc 1", "Inc 2"} {
		if err := is.Create(ctx, &store.Incident{
			Title: title, Status: store.IncidentStatusTriggered,
			Severity: store.SeverityMajor, Source: "manual",
		}); err != nil {
			t.Fatalf("Create %q: %v", title, err)
		}
	}

	got, err = is.List(ctx, store.IncidentFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List: got %d, want 2", len(got))
	}
}

func TestIncident_ListWithStatusFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	if err := is.Create(ctx, &store.Incident{
		Title: "Triggered", Status: store.IncidentStatusTriggered,
		Severity: store.SeverityMajor, Source: "manual",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := is.Create(ctx, &store.Incident{
		Title: "Resolved", Status: store.IncidentStatusResolved,
		Severity: store.SeverityMinor, Source: "manual",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := is.List(ctx, store.IncidentFilter{Status: store.IncidentStatusTriggered})
	if err != nil {
		t.Fatalf("List with status filter: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List with status filter: got %d, want 1", len(got))
	}
	if got[0].Title != "Triggered" {
		t.Errorf("Title = %q, want Triggered", got[0].Title)
	}
}

func TestIncident_ListWithLimitOffset(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	for i := 0; i < 5; i++ {
		if err := is.Create(ctx, &store.Incident{
			Title: "Inc", Status: store.IncidentStatusTriggered,
			Severity: store.SeverityMinor, Source: "manual",
		}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	got, err := is.List(ctx, store.IncidentFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List limit: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List limit 2: got %d, want 2", len(got))
	}

	got, err = is.List(ctx, store.IncidentFilter{Limit: 2, Offset: 3})
	if err != nil {
		t.Fatalf("List limit+offset: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List limit 2 offset 3: got %d, want 2", len(got))
	}

	got, err = is.List(ctx, store.IncidentFilter{Limit: 10, Offset: 10})
	if err != nil {
		t.Fatalf("List offset past end: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List offset past end: got %d, want 0", len(got))
	}
}

func TestIncident_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	inc := &store.Incident{
		Title: "Original", Status: store.IncidentStatusTriggered,
		Severity: store.SeverityMajor, Source: "manual",
	}
	if err := is.Create(ctx, inc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	inc.Title = "Updated Title"
	inc.Status = store.IncidentStatusResolved
	inc.Severity = store.SeverityMinor
	inc.Summary = "Fixed it"
	inc.ResolvedAt = &now

	if err := is.Update(ctx, inc); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := is.Get(ctx, inc.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want Updated Title", got.Title)
	}
	if got.Status != store.IncidentStatusResolved {
		t.Errorf("Status = %q, want resolved", got.Status)
	}
	if got.Severity != store.SeverityMinor {
		t.Errorf("Severity = %q, want minor", got.Severity)
	}
	if got.Summary != "Fixed it" {
		t.Errorf("Summary = %q, want 'Fixed it'", got.Summary)
	}
	if got.ResolvedAt == nil {
		t.Fatal("ResolvedAt should not be nil after update")
	}
}

func TestIncident_UpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Incidents().Update(ctx, &store.Incident{ID: "nonexistent", Status: "resolved", Severity: "minor"})
	if err != store.ErrNotFound {
		t.Fatalf("Update non-existent: got %v, want ErrNotFound", err)
	}
}

func TestIncident_AddServiceAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	inc := &store.Incident{
		Title: "Inc", Status: store.IncidentStatusTriggered,
		Severity: store.SeverityMajor, Source: "manual",
	}
	if err := is.Create(ctx, inc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := is.AddService(ctx, inc.ID, "svc-1"); err != nil {
		t.Fatalf("AddService: %v", err)
	}
	if err := is.AddService(ctx, inc.ID, "svc-2"); err != nil {
		t.Fatalf("AddService: %v", err)
	}

	services, err := is.ListServices(ctx, inc.ID)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("ListServices: got %d, want 2", len(services))
	}
}

func TestIncident_AddServiceIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	inc := &store.Incident{
		Title: "Inc", Status: store.IncidentStatusTriggered,
		Severity: store.SeverityMajor, Source: "manual",
	}
	if err := is.Create(ctx, inc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := is.AddService(ctx, inc.ID, "svc-1"); err != nil {
		t.Fatalf("AddService first: %v", err)
	}
	if err := is.AddService(ctx, inc.ID, "svc-1"); err != nil {
		t.Fatalf("AddService second (idempotent): %v", err)
	}

	services, err := is.ListServices(ctx, inc.ID)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("ListServices after idempotent add: got %d, want 1", len(services))
	}
}

func TestIncident_CreateUpdateAndListUpdates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	is := s.Incidents()

	inc := &store.Incident{
		Title: "Inc", Status: store.IncidentStatusTriggered,
		Severity: store.SeverityMajor, Source: "manual",
	}
	if err := is.Create(ctx, inc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	u1 := &store.IncidentUpdate{
		IncidentID: inc.ID,
		Status:     store.IncidentStatusInvestigating,
		Message:    "Looking into it",
	}
	u2 := &store.IncidentUpdate{
		IncidentID: inc.ID,
		Status:     store.IncidentStatusResolved,
		Message:    "Fixed",
	}
	if err := is.CreateUpdate(ctx, u1); err != nil {
		t.Fatalf("CreateUpdate 1: %v", err)
	}
	if u1.ID == "" {
		t.Fatal("expected update ID to be set")
	}
	if err := is.CreateUpdate(ctx, u2); err != nil {
		t.Fatalf("CreateUpdate 2: %v", err)
	}

	updates, err := is.ListUpdates(ctx, inc.ID)
	if err != nil {
		t.Fatalf("ListUpdates: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("ListUpdates: got %d, want 2", len(updates))
	}

	if updates[0].Status != store.IncidentStatusInvestigating {
		t.Errorf("first update status = %q, want investigating", updates[0].Status)
	}
	if updates[0].Message != "Looking into it" {
		t.Errorf("first update message = %q, want 'Looking into it'", updates[0].Message)
	}
	if updates[1].Status != store.IncidentStatusResolved {
		t.Errorf("second update status = %q, want resolved", updates[1].Status)
	}
	if updates[1].CreatedAt.IsZero() {
		t.Error("update CreatedAt should not be zero")
	}
}
