package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

// ---------------------------------------------------------------------------
// Team CRUD
// ---------------------------------------------------------------------------

func TestTeamCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "Platform", Description: "Platform team"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if team.ID == "" {
		t.Fatal("expected ID to be auto-generated")
	}

	got, err := teams.Get(ctx, team.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Platform" {
		t.Errorf("Name = %q, want %q", got.Name, "Platform")
	}
	if got.Description != "Platform team" {
		t.Errorf("Description = %q, want %q", got.Description, "Platform team")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestTeamGetNotFound(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	_, err := teams.Get(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Get nonexistent: got err %v, want ErrNotFound", err)
	}
}

func TestTeamDuplicateName(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	if err := teams.Create(ctx, &store.Team{Name: "Unique"}); err != nil {
		t.Fatal(err)
	}
	err := teams.Create(ctx, &store.Team{Name: "Unique"})
	if err == nil {
		t.Fatal("expected error for duplicate team name, got nil")
	}
}

func TestTeamList(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	list, err := teams.List(ctx)
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 teams, got %d", len(list))
	}

	// Insert out of order to verify ORDER BY name.
	for _, name := range []string{"Charlie", "Alpha", "Bravo"} {
		if err := teams.Create(ctx, &store.Team{Name: name}); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	list, err = teams.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 teams, got %d", len(list))
	}
	wantNames := []string{"Alpha", "Bravo", "Charlie"}
	for i, want := range wantNames {
		if list[i].Name != want {
			t.Errorf("list[%d].Name = %q, want %q", i, list[i].Name, want)
		}
	}
}

func TestTeamUpdate(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "Old Name", Description: "old"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	team.Name = "New Name"
	team.Description = "new"
	if err := teams.Update(ctx, team); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := teams.Get(ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "New Name" {
		t.Errorf("Name = %q, want %q", got.Name, "New Name")
	}
	if got.Description != "new" {
		t.Errorf("Description = %q, want %q", got.Description, "new")
	}
}

func TestTeamUpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	err := teams.Update(ctx, &store.Team{ID: "ghost", Name: "x"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Update nonexistent: got err %v, want ErrNotFound", err)
	}
}

func TestTeamDelete(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "Doomed"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}
	if err := teams.Delete(ctx, team.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := teams.Get(ctx, team.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Get after Delete: got err %v, want ErrNotFound", err)
	}
}

func TestTeamDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	err := teams.Delete(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Delete nonexistent: got err %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Team membership
// ---------------------------------------------------------------------------

func TestTeamAddAndListMembers(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "SRE"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}
	alice := createTestUser(t, s, "Alice", "alice@example.com")
	bob := createTestUser(t, s, "Bob", "bob@example.com")

	// Add alice as admin, bob as member.
	if err := teams.AddMember(ctx, team.ID, alice.ID, "admin"); err != nil {
		t.Fatalf("AddMember alice: %v", err)
	}
	if err := teams.AddMember(ctx, team.ID, bob.ID, "member"); err != nil {
		t.Fatalf("AddMember bob: %v", err)
	}

	members, err := teams.ListMembers(ctx, team.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// Verify at least one admin and one member.
	roles := map[string]bool{}
	for _, m := range members {
		roles[m.Role] = true
		if m.TeamID != team.ID {
			t.Errorf("TeamID = %q, want %q", m.TeamID, team.ID)
		}
	}
	if !roles["admin"] {
		t.Error("expected an admin member")
	}
	if !roles["member"] {
		t.Error("expected a regular member")
	}
}

func TestTeamRemoveMember(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "Backend"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}
	user := createTestUser(t, s, "Charlie", "charlie@example.com")
	if err := teams.AddMember(ctx, team.ID, user.ID, "member"); err != nil {
		t.Fatal(err)
	}

	if err := teams.RemoveMember(ctx, team.ID, user.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	members, err := teams.ListMembers(ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatalf("expected 0 members after remove, got %d", len(members))
	}
}

func TestTeamRemoveMemberNotFound(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "Empty"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	err := teams.RemoveMember(ctx, team.ID, "nonexistent-user")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("RemoveMember nonexistent: got err %v, want ErrNotFound", err)
	}
}

func TestTeamListTeamsForUser(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	user := createTestUser(t, s, "Multi-team", "multi@example.com")

	// Create two teams and add user to both.
	for _, name := range []string{"Zulu", "Alpha"} {
		team := &store.Team{Name: name}
		if err := teams.Create(ctx, team); err != nil {
			t.Fatal(err)
		}
		if err := teams.AddMember(ctx, team.ID, user.ID, "member"); err != nil {
			t.Fatal(err)
		}
	}

	userTeams, err := teams.ListTeamsForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListTeamsForUser: %v", err)
	}
	if len(userTeams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(userTeams))
	}
	// Should be ordered by name.
	if userTeams[0].Name != "Alpha" {
		t.Errorf("first team = %q, want %q", userTeams[0].Name, "Alpha")
	}
	if userTeams[1].Name != "Zulu" {
		t.Errorf("second team = %q, want %q", userTeams[1].Name, "Zulu")
	}
}

func TestTeamDeleteCascadesMembers(t *testing.T) {
	s := newTestStore(t)
	teams := s.Teams()
	ctx := context.Background()

	team := &store.Team{Name: "Cascade"}
	if err := teams.Create(ctx, team); err != nil {
		t.Fatal(err)
	}
	user := createTestUser(t, s, "CascadeUser", "cascade@example.com")
	if err := teams.AddMember(ctx, team.ID, user.ID, "admin"); err != nil {
		t.Fatal(err)
	}

	if err := teams.Delete(ctx, team.ID); err != nil {
		t.Fatal(err)
	}

	// User's team list should be empty after team deletion.
	userTeams, err := teams.ListTeamsForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(userTeams) != 0 {
		t.Fatalf("expected 0 teams after cascade delete, got %d", len(userTeams))
	}
}

// ---------------------------------------------------------------------------
// Team-scoped filtering (ListByTeam on services, policies, schedules)
// ---------------------------------------------------------------------------

func TestServiceListByTeam(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	team := &store.Team{Name: "TeamA"}
	if err := s.Teams().Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	// Create two services: one with team, one without.
	svc1 := &store.Service{Name: "svc-with-team", EscalationPolicyID: "ep-1", TeamID: &team.ID}
	svc2 := &store.Service{Name: "svc-no-team", EscalationPolicyID: "ep-1"}
	if err := s.Services().Create(ctx, svc1); err != nil {
		t.Fatal(err)
	}
	if err := s.Services().Create(ctx, svc2); err != nil {
		t.Fatal(err)
	}

	// ListByTeam should return only svc1.
	list, err := s.Services().ListByTeam(ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByTeam: got %d, want 1", len(list))
	}
	if list[0].Name != "svc-with-team" {
		t.Errorf("Name = %q, want %q", list[0].Name, "svc-with-team")
	}

	// Full List should return both.
	all, err := s.Services().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("List: got %d, want 2", len(all))
	}
}

func TestEscalationPolicyListByTeam(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	team := &store.Team{Name: "TeamB"}
	if err := s.Teams().Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	ep1 := &store.EscalationPolicy{Name: "ep-team", TeamID: &team.ID}
	ep2 := &store.EscalationPolicy{Name: "ep-global"}
	if err := s.EscalationPolicies().Create(ctx, ep1); err != nil {
		t.Fatal(err)
	}
	if err := s.EscalationPolicies().Create(ctx, ep2); err != nil {
		t.Fatal(err)
	}

	list, err := s.EscalationPolicies().ListByTeam(ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByTeam: got %d, want 1", len(list))
	}
	if list[0].Name != "ep-team" {
		t.Errorf("Name = %q, want %q", list[0].Name, "ep-team")
	}
}

func TestScheduleListByTeam(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	team := &store.Team{Name: "TeamC"}
	if err := s.Teams().Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	sched1 := &store.Schedule{Name: "sched-team", Timezone: "UTC", TeamID: &team.ID}
	sched2 := &store.Schedule{Name: "sched-global", Timezone: "UTC"}
	if err := s.Schedules().Create(ctx, sched1); err != nil {
		t.Fatal(err)
	}
	if err := s.Schedules().Create(ctx, sched2); err != nil {
		t.Fatal(err)
	}

	list, err := s.Schedules().ListByTeam(ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByTeam: got %d, want 1", len(list))
	}
	if list[0].Name != "sched-team" {
		t.Errorf("Name = %q, want %q", list[0].Name, "sched-team")
	}
}

func TestTeamDeleteSetsNullOnResources(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	team := &store.Team{Name: "WillBeDeleted"}
	if err := s.Teams().Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	svc := &store.Service{Name: "orphan-svc", EscalationPolicyID: "ep-1", TeamID: &team.ID}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	// Delete team — ON DELETE SET NULL should clear team_id.
	if err := s.Teams().Delete(ctx, team.ID); err != nil {
		t.Fatal(err)
	}

	got, err := s.Services().Get(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeamID != nil {
		t.Errorf("expected TeamID to be nil after team deletion, got %v", *got.TeamID)
	}
}
