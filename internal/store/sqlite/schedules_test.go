package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

func createTestUserWithID(t *testing.T, s *SQLiteStore, id, name, email string) {
	t.Helper()
	u := &store.User{ID: id, Name: name, Email: email, Role: "user", Timezone: "UTC"}
	if err := s.Users().Create(context.Background(), u); err != nil {
		t.Fatalf("create test user %q: %v", id, err)
	}
}

func TestSchedule_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sched := &store.Schedule{
		Name:        "Primary",
		Description: "Primary on-call",
		Timezone:    "America/New_York",
	}
	if err := s.Schedules().Create(ctx, sched); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sched.ID == "" {
		t.Fatal("expected ID to be set after Create")
	}

	got, err := s.Schedules().Get(ctx, sched.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Primary" {
		t.Errorf("Name = %q, want %q", got.Name, "Primary")
	}
	if got.Description != "Primary on-call" {
		t.Errorf("Description = %q, want %q", got.Description, "Primary on-call")
	}
	if got.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", got.Timezone, "America/New_York")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestSchedule_GetNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.Schedules().Get(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("Get non-existent: got %v, want ErrNotFound", err)
	}
}

func TestSchedule_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	got, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List empty: got %d, want 0", len(got))
	}

	if err := ss.Create(ctx, &store.Schedule{Name: "Zulu", Timezone: "UTC"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ss.Create(ctx, &store.Schedule{Name: "Alpha", Timezone: "UTC"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err = ss.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List: got %d, want 2", len(got))
	}
	if got[0].Name != "Alpha" || got[1].Name != "Zulu" {
		t.Errorf("List order: got [%q, %q], want [Alpha, Zulu]", got[0].Name, got[1].Name)
	}
}

func TestSchedule_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	sched := &store.Schedule{Name: "Original", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sched.Name = "Updated"
	sched.Description = "new desc"
	sched.Timezone = "Europe/London"
	if err := ss.Update(ctx, sched); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := ss.Get(ctx, sched.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name = %q, want Updated", got.Name)
	}
	if got.Description != "new desc" {
		t.Errorf("Description = %q, want 'new desc'", got.Description)
	}
	if got.Timezone != "Europe/London" {
		t.Errorf("Timezone = %q, want Europe/London", got.Timezone)
	}
}

func TestSchedule_UpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Schedules().Update(ctx, &store.Schedule{ID: "nonexistent", Timezone: "UTC"})
	if err != store.ErrNotFound {
		t.Fatalf("Update non-existent: got %v, want ErrNotFound", err)
	}
}

func TestSchedule_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	sched := &store.Schedule{Name: "To Delete", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := ss.Delete(ctx, sched.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := ss.Get(ctx, sched.ID)
	if err != store.ErrNotFound {
		t.Fatalf("Get after delete: got %v, want ErrNotFound", err)
	}
}

func TestSchedule_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Schedules().Delete(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("Delete non-existent: got %v, want ErrNotFound", err)
	}
}

func TestRotation_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	r := &store.Rotation{
		ScheduleID:  sched.ID,
		Name:        "Weekly Rotation",
		Type:        store.RotationTypeWeekly,
		ShiftLength: 7,
		StartTime:   now,
		HandoffTime: "09:00",
	}
	if err := ss.CreateRotation(ctx, r); err != nil {
		t.Fatalf("CreateRotation: %v", err)
	}
	if r.ID == "" {
		t.Fatal("expected rotation ID to be set")
	}

	rotations, err := ss.ListRotations(ctx, sched.ID)
	if err != nil {
		t.Fatalf("ListRotations: %v", err)
	}
	if len(rotations) != 1 {
		t.Fatalf("ListRotations: got %d, want 1", len(rotations))
	}
	if rotations[0].Name != "Weekly Rotation" {
		t.Errorf("rotation Name = %q, want %q", rotations[0].Name, "Weekly Rotation")
	}
	if rotations[0].Type != store.RotationTypeWeekly {
		t.Errorf("rotation Type = %q, want %q", rotations[0].Type, store.RotationTypeWeekly)
	}
	if rotations[0].ShiftLength != 7 {
		t.Errorf("rotation ShiftLength = %d, want 7", rotations[0].ShiftLength)
	}
	if rotations[0].HandoffTime != "09:00" {
		t.Errorf("rotation HandoffTime = %q, want %q", rotations[0].HandoffTime, "09:00")
	}
}

func TestRotation_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}
	r := &store.Rotation{
		ScheduleID: sched.ID, Name: "R1", Type: store.RotationTypeDaily,
		ShiftLength: 1, StartTime: time.Now(), HandoffTime: "08:00",
	}
	if err := ss.CreateRotation(ctx, r); err != nil {
		t.Fatalf("CreateRotation: %v", err)
	}

	if err := ss.DeleteRotation(ctx, r.ID); err != nil {
		t.Fatalf("DeleteRotation: %v", err)
	}

	rotations, err := ss.ListRotations(ctx, sched.ID)
	if err != nil {
		t.Fatalf("ListRotations: %v", err)
	}
	if len(rotations) != 0 {
		t.Fatalf("ListRotations after delete: got %d, want 0", len(rotations))
	}
}

func TestRotation_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Schedules().DeleteRotation(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("DeleteRotation non-existent: got %v, want ErrNotFound", err)
	}
}

func TestParticipant_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	createTestUserWithID(t, s, "user-1", "Alice", "alice@example.com")
	createTestUserWithID(t, s, "user-2", "Bob", "bob@example.com")

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}
	r := &store.Rotation{
		ScheduleID: sched.ID, Name: "R1", Type: store.RotationTypeDaily,
		ShiftLength: 1, StartTime: time.Now(), HandoffTime: "08:00",
	}
	if err := ss.CreateRotation(ctx, r); err != nil {
		t.Fatalf("CreateRotation: %v", err)
	}

	// Create participants out of order to verify ordering by position.
	p2 := &store.RotationParticipant{RotationID: r.ID, UserID: "user-2", Position: 2}
	p1 := &store.RotationParticipant{RotationID: r.ID, UserID: "user-1", Position: 1}
	if err := ss.CreateParticipant(ctx, p2); err != nil {
		t.Fatalf("CreateParticipant 2: %v", err)
	}
	if err := ss.CreateParticipant(ctx, p1); err != nil {
		t.Fatalf("CreateParticipant 1: %v", err)
	}

	participants, err := ss.ListParticipants(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("ListParticipants: got %d, want 2", len(participants))
	}
	if participants[0].Position != 1 {
		t.Errorf("first participant position = %d, want 1", participants[0].Position)
	}
	if participants[0].UserID != "user-1" {
		t.Errorf("first participant UserID = %q, want user-1", participants[0].UserID)
	}
	if participants[1].Position != 2 {
		t.Errorf("second participant position = %d, want 2", participants[1].Position)
	}
}

func TestParticipant_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	createTestUserWithID(t, s, "user-1", "Alice", "alice@example.com")

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}
	r := &store.Rotation{
		ScheduleID: sched.ID, Name: "R1", Type: store.RotationTypeDaily,
		ShiftLength: 1, StartTime: time.Now(), HandoffTime: "08:00",
	}
	if err := ss.CreateRotation(ctx, r); err != nil {
		t.Fatalf("CreateRotation: %v", err)
	}
	p := &store.RotationParticipant{RotationID: r.ID, UserID: "user-1", Position: 1}
	if err := ss.CreateParticipant(ctx, p); err != nil {
		t.Fatalf("CreateParticipant: %v", err)
	}

	if err := ss.DeleteParticipant(ctx, p.ID); err != nil {
		t.Fatalf("DeleteParticipant: %v", err)
	}

	participants, err := ss.ListParticipants(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(participants) != 0 {
		t.Fatalf("ListParticipants after delete: got %d, want 0", len(participants))
	}
}

func TestParticipant_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Schedules().DeleteParticipant(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("DeleteParticipant non-existent: got %v, want ErrNotFound", err)
	}
}

func TestOverride_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	createTestUserWithID(t, s, "user-a", "Alice", "alice@example.com")
	createTestUserWithID(t, s, "user-b", "Bob", "bob@example.com")

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	o := &store.ScheduleOverride{
		ScheduleID:   sched.ID,
		StartTime:    now,
		EndTime:      now.Add(24 * time.Hour),
		ReplaceUser:  "user-a",
		OverrideUser: "user-b",
	}
	if err := ss.CreateOverride(ctx, o); err != nil {
		t.Fatalf("CreateOverride: %v", err)
	}
	if o.ID == "" {
		t.Fatal("expected override ID to be set")
	}

	overrides, err := ss.ListOverrides(ctx, sched.ID)
	if err != nil {
		t.Fatalf("ListOverrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("ListOverrides: got %d, want 1", len(overrides))
	}
	if overrides[0].ReplaceUser != "user-a" {
		t.Errorf("ReplaceUser = %q, want user-a", overrides[0].ReplaceUser)
	}
	if overrides[0].OverrideUser != "user-b" {
		t.Errorf("OverrideUser = %q, want user-b", overrides[0].OverrideUser)
	}
}

func TestOverride_ListActiveWithTimeFiltering(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	createTestUserWithID(t, s, "user-a", "Alice", "alice@example.com")
	createTestUserWithID(t, s, "user-b", "Bob", "bob@example.com")

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	// Expired override: ended 1 hour ago.
	expired := &store.ScheduleOverride{
		ScheduleID: sched.ID,
		StartTime:  now.Add(-3 * time.Hour),
		EndTime:    now.Add(-1 * time.Hour),
		ReplaceUser: "user-a", OverrideUser: "user-b",
	}
	// Active override: started 1 hour ago, ends in 1 hour.
	active := &store.ScheduleOverride{
		ScheduleID: sched.ID,
		StartTime:  now.Add(-1 * time.Hour),
		EndTime:    now.Add(1 * time.Hour),
		ReplaceUser: "user-a", OverrideUser: "user-b",
	}
	// Future override: starts in 2 hours.
	future := &store.ScheduleOverride{
		ScheduleID: sched.ID,
		StartTime:  now.Add(2 * time.Hour),
		EndTime:    now.Add(4 * time.Hour),
		ReplaceUser: "user-a", OverrideUser: "user-b",
	}

	for _, o := range []*store.ScheduleOverride{expired, active, future} {
		if err := ss.CreateOverride(ctx, o); err != nil {
			t.Fatalf("CreateOverride: %v", err)
		}
	}

	// Only the active override should be returned.
	result, err := ss.ListActiveOverrides(ctx, sched.ID, now)
	if err != nil {
		t.Fatalf("ListActiveOverrides: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("ListActiveOverrides: got %d, want 1", len(result))
	}
	if result[0].ID != active.ID {
		t.Errorf("active override ID = %q, want %q", result[0].ID, active.ID)
	}
}

func TestOverride_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	createTestUserWithID(t, s, "user-a", "Alice", "alice@example.com")
	createTestUserWithID(t, s, "user-b", "Bob", "bob@example.com")

	sched := &store.Schedule{Name: "Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}

	o := &store.ScheduleOverride{
		ScheduleID: sched.ID,
		StartTime:  time.Now(), EndTime: time.Now().Add(time.Hour),
		ReplaceUser: "user-a", OverrideUser: "user-b",
	}
	if err := ss.CreateOverride(ctx, o); err != nil {
		t.Fatalf("CreateOverride: %v", err)
	}

	if err := ss.DeleteOverride(ctx, o.ID); err != nil {
		t.Fatalf("DeleteOverride: %v", err)
	}

	overrides, err := ss.ListOverrides(ctx, sched.ID)
	if err != nil {
		t.Fatalf("ListOverrides: %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("ListOverrides after delete: got %d, want 0", len(overrides))
	}
}

func TestOverride_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.Schedules().DeleteOverride(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("DeleteOverride non-existent: got %v, want ErrNotFound", err)
	}
}

func TestSchedule_DeleteCascade(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ss := s.Schedules()

	createTestUserWithID(t, s, "user-a", "Alice", "alice@example.com")
	createTestUserWithID(t, s, "user-b", "Bob", "bob@example.com")

	sched := &store.Schedule{Name: "Cascade Sched", Timezone: "UTC"}
	if err := ss.Create(ctx, sched); err != nil {
		t.Fatalf("Create schedule: %v", err)
	}

	r := &store.Rotation{
		ScheduleID: sched.ID, Name: "R1", Type: store.RotationTypeDaily,
		ShiftLength: 1, StartTime: time.Now(), HandoffTime: "08:00",
	}
	if err := ss.CreateRotation(ctx, r); err != nil {
		t.Fatalf("CreateRotation: %v", err)
	}

	p := &store.RotationParticipant{RotationID: r.ID, UserID: "user-a", Position: 1}
	if err := ss.CreateParticipant(ctx, p); err != nil {
		t.Fatalf("CreateParticipant: %v", err)
	}

	o := &store.ScheduleOverride{
		ScheduleID: sched.ID,
		StartTime:  time.Now(), EndTime: time.Now().Add(time.Hour),
		ReplaceUser: "user-a", OverrideUser: "user-b",
	}
	if err := ss.CreateOverride(ctx, o); err != nil {
		t.Fatalf("CreateOverride: %v", err)
	}

	// Delete the schedule; rotations, participants, overrides should cascade.
	if err := ss.Delete(ctx, sched.ID); err != nil {
		t.Fatalf("Delete schedule: %v", err)
	}

	rotations, err := ss.ListRotations(ctx, sched.ID)
	if err != nil {
		t.Fatalf("ListRotations after cascade: %v", err)
	}
	if len(rotations) != 0 {
		t.Errorf("rotations after cascade: got %d, want 0", len(rotations))
	}

	participants, err := ss.ListParticipants(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListParticipants after cascade: %v", err)
	}
	if len(participants) != 0 {
		t.Errorf("participants after cascade: got %d, want 0", len(participants))
	}

	overrides, err := ss.ListOverrides(ctx, sched.ID)
	if err != nil {
		t.Fatalf("ListOverrides after cascade: %v", err)
	}
	if len(overrides) != 0 {
		t.Errorf("overrides after cascade: got %d, want 0", len(overrides))
	}
}
