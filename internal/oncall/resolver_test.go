package oncall

import (
	"context"
	"testing"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

// ---------------------------------------------------------------------------
// Mock: ScheduleStore
// ---------------------------------------------------------------------------

type mockScheduleStore struct {
	rotations    []store.Rotation
	participants map[string][]store.RotationParticipant // keyed by rotation ID
	overrides    []store.ScheduleOverride
}

func (s *mockScheduleStore) ListRotations(_ context.Context, _ string) ([]store.Rotation, error) {
	return s.rotations, nil
}

func (s *mockScheduleStore) ListParticipants(_ context.Context, rotationID string) ([]store.RotationParticipant, error) {
	return s.participants[rotationID], nil
}

func (s *mockScheduleStore) ListActiveOverrides(_ context.Context, _ string, _ time.Time) ([]store.ScheduleOverride, error) {
	return s.overrides, nil
}

func (s *mockScheduleStore) Create(context.Context, *store.Schedule) error   { panic("not implemented") }
func (s *mockScheduleStore) Get(context.Context, string) (*store.Schedule, error) {
	panic("not implemented")
}
func (s *mockScheduleStore) List(context.Context) ([]store.Schedule, error) { panic("not implemented") }
func (s *mockScheduleStore) Update(context.Context, *store.Schedule) error  { panic("not implemented") }
func (s *mockScheduleStore) Delete(context.Context, string) error           { panic("not implemented") }
func (s *mockScheduleStore) CreateRotation(context.Context, *store.Rotation) error {
	panic("not implemented")
}
func (s *mockScheduleStore) GetRotation(context.Context, string) (*store.Rotation, error) {
	panic("not implemented")
}
func (s *mockScheduleStore) DeleteRotation(context.Context, string) error { panic("not implemented") }
func (s *mockScheduleStore) CreateParticipant(context.Context, *store.RotationParticipant) error {
	panic("not implemented")
}
func (s *mockScheduleStore) DeleteParticipant(context.Context, string) error {
	panic("not implemented")
}
func (s *mockScheduleStore) CreateOverride(context.Context, *store.ScheduleOverride) error {
	panic("not implemented")
}
func (s *mockScheduleStore) ListOverrides(context.Context, string) ([]store.ScheduleOverride, error) {
	panic("not implemented")
}
func (s *mockScheduleStore) DeleteOverride(context.Context, string) error { panic("not implemented") }
func (s *mockScheduleStore) ListByTeam(context.Context, string) ([]store.Schedule, error) {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Mock: UserStore
// ---------------------------------------------------------------------------

type mockUserStore struct {
	users map[string]*store.User
}

func (s *mockUserStore) Get(_ context.Context, id string) (*store.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (s *mockUserStore) Create(context.Context, *store.User) error            { panic("not implemented") }
func (s *mockUserStore) GetByEmail(context.Context, string) (*store.User, error) {
	panic("not implemented")
}
func (s *mockUserStore) List(context.Context) ([]store.User, error) { panic("not implemented") }
func (s *mockUserStore) Update(context.Context, *store.User) error  { panic("not implemented") }
func (s *mockUserStore) Delete(context.Context, string) error       { panic("not implemented") }
func (s *mockUserStore) CreateContactMethod(context.Context, *store.ContactMethod) error {
	panic("not implemented")
}
func (s *mockUserStore) ListContactMethods(context.Context, string) ([]store.ContactMethod, error) {
	panic("not implemented")
}
func (s *mockUserStore) DeleteContactMethod(context.Context, string) error {
	panic("not implemented")
}
func (s *mockUserStore) CreateNotificationRule(context.Context, *store.NotificationRule) error {
	panic("not implemented")
}
func (s *mockUserStore) ListNotificationRules(context.Context, string) ([]store.NotificationRule, error) {
	panic("not implemented")
}
func (s *mockUserStore) DeleteNotificationRule(context.Context, string) error {
	panic("not implemented")
}
func (s *mockUserStore) SetPassword(context.Context, string, string) error { panic("not implemented") }
func (s *mockUserStore) SetLastLogin(context.Context, string) error       { panic("not implemented") }
func (s *mockUserStore) CountUsers(context.Context) (int, error)          { panic("not implemented") }
func (s *mockUserStore) CreateAPIToken(context.Context, *store.APIToken, string) error {
	panic("not implemented")
}
func (s *mockUserStore) ListAPITokens(context.Context, string) ([]store.APIToken, error) {
	panic("not implemented")
}
func (s *mockUserStore) GetAPITokenByHash(context.Context, string) (*store.APIToken, error) {
	panic("not implemented")
}
func (s *mockUserStore) RevokeAPIToken(context.Context, string) error { panic("not implemented") }
func (s *mockUserStore) TouchAPIToken(context.Context, string) error  { panic("not implemented") }
func (s *mockUserStore) CreateInviteToken(context.Context, *store.InviteToken) error {
	panic("not implemented")
}
func (s *mockUserStore) GetInviteTokenByHash(context.Context, string) (*store.InviteToken, error) {
	panic("not implemented")
}
func (s *mockUserStore) UseInviteToken(context.Context, string) error { panic("not implemented") }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var baseTime = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC) // Monday 09:00 UTC

func makeUsers(ids ...string) map[string]*store.User {
	m := make(map[string]*store.User, len(ids))
	for _, id := range ids {
		m[id] = &store.User{ID: id, Name: "User " + id, Email: id + "@example.com"}
	}
	return m
}

func makeParticipants(rotationID string, userIDs ...string) []store.RotationParticipant {
	out := make([]store.RotationParticipant, len(userIDs))
	for i, uid := range userIDs {
		out[i] = store.RotationParticipant{
			ID:         rotationID + "-p" + uid,
			RotationID: rotationID,
			UserID:     uid,
			Position:   i,
		}
	}
	return out
}

func requireUsers(t *testing.T, got []store.User, wantIDs ...string) {
	t.Helper()
	if len(got) != len(wantIDs) {
		t.Fatalf("expected %d on-call users, got %d: %v", len(wantIDs), len(got), userIDs(got))
	}
	gotSet := make(map[string]bool, len(got))
	for _, u := range got {
		gotSet[u.ID] = true
	}
	for _, id := range wantIDs {
		if !gotSet[id] {
			t.Errorf("expected user %q on-call, got %v", id, userIDs(got))
		}
	}
}

func userIDs(users []store.User) []string {
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return ids
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestResolve_EmptyRotations(t *testing.T) {
	r := NewResolver(
		&mockScheduleStore{},
		&mockUserStore{users: makeUsers()},
	)

	got, err := r.Resolve(context.Background(), "sched-1", baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 on-call users, got %d", len(got))
	}
}

func TestResolve_SingleWeeklyRotation_ThreeParticipants(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": makeParticipants("rot-1", "alice", "bob", "charlie"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob", "charlie")}
	r := NewResolver(schedStore, userStore)

	cases := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"at start", 0, "alice"},
		{"1 week later", 7 * 24 * time.Hour, "bob"},
		{"2 weeks later", 2 * 7 * 24 * time.Hour, "charlie"},
		{"3 weeks later wraps", 3 * 7 * 24 * time.Hour, "alice"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Resolve(context.Background(), "sched-1", baseTime.Add(tc.offset))
			if err != nil {
				t.Fatal(err)
			}
			requireUsers(t, got, tc.want)
		})
	}
}

func TestResolve_DailyRotation(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-d",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeDaily,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-d": makeParticipants("rot-d", "alice", "bob"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob")}
	r := NewResolver(schedStore, userStore)

	cases := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"day 0", 0, "alice"},
		{"day 1", 24 * time.Hour, "bob"},
		{"day 2 wraps", 2 * 24 * time.Hour, "alice"},
		{"day 3", 3 * 24 * time.Hour, "bob"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Resolve(context.Background(), "sched-1", baseTime.Add(tc.offset))
			if err != nil {
				t.Fatal(err)
			}
			requireUsers(t, got, tc.want)
		})
	}
}

func TestResolve_CustomHourlyRotation(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-c",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeCustom,
			ShiftLength: 4, // 4-hour shifts
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-c": makeParticipants("rot-c", "alice", "bob", "charlie"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob", "charlie")}
	r := NewResolver(schedStore, userStore)

	cases := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"hour 0", 0, "alice"},
		{"hour 3 (still alice)", 3 * time.Hour, "alice"},
		{"hour 4 (bob)", 4 * time.Hour, "bob"},
		{"hour 8 (charlie)", 8 * time.Hour, "charlie"},
		{"hour 12 (wraps to alice)", 12 * time.Hour, "alice"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Resolve(context.Background(), "sched-1", baseTime.Add(tc.offset))
			if err != nil {
				t.Fatal(err)
			}
			requireUsers(t, got, tc.want)
		})
	}
}

func TestResolve_BeforeRotationStart(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": makeParticipants("rot-1", "alice"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("alice")}
	r := NewResolver(schedStore, userStore)

	got, err := r.Resolve(context.Background(), "sched-1", baseTime.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 on-call users before rotation start, got %d", len(got))
	}
}

func TestResolve_EmptyParticipants(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": {},
		},
	}
	userStore := &mockUserStore{users: makeUsers()}
	r := NewResolver(schedStore, userStore)

	got, err := r.Resolve(context.Background(), "sched-1", baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 on-call users with empty participants, got %d", len(got))
	}
}

func TestResolve_MultipleRotations(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{
			{
				ID:          "rot-a",
				ScheduleID:  "sched-1",
				Type:        store.RotationTypeWeekly,
				ShiftLength: 1,
				StartTime:   baseTime,
			},
			{
				ID:          "rot-b",
				ScheduleID:  "sched-1",
				Type:        store.RotationTypeWeekly,
				ShiftLength: 1,
				StartTime:   baseTime,
			},
		},
		participants: map[string][]store.RotationParticipant{
			"rot-a": makeParticipants("rot-a", "alice"),
			"rot-b": makeParticipants("rot-b", "bob"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob")}
	r := NewResolver(schedStore, userStore)

	got, err := r.Resolve(context.Background(), "sched-1", baseTime)
	if err != nil {
		t.Fatal(err)
	}
	requireUsers(t, got, "alice", "bob")
}

func TestResolve_OverrideReplacesOnCallUser(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": makeParticipants("rot-1", "alice", "charlie"),
		},
		overrides: []store.ScheduleOverride{{
			ID:           "ovr-1",
			ScheduleID:   "sched-1",
			StartTime:    baseTime,
			EndTime:      baseTime.Add(7 * 24 * time.Hour),
			ReplaceUser:  "alice",
			OverrideUser: "bob",
		}},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob", "charlie")}
	r := NewResolver(schedStore, userStore)

	got, err := r.Resolve(context.Background(), "sched-1", baseTime)
	if err != nil {
		t.Fatal(err)
	}
	requireUsers(t, got, "bob")
}

func TestResolve_OverrideForNonOnCallUser(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": makeParticipants("rot-1", "alice", "bob"),
		},
		overrides: []store.ScheduleOverride{{
			ID:           "ovr-1",
			ScheduleID:   "sched-1",
			StartTime:    baseTime,
			EndTime:      baseTime.Add(7 * 24 * time.Hour),
			ReplaceUser:  "charlie", // charlie is not on-call
			OverrideUser: "dave",
		}},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob")}
	r := NewResolver(schedStore, userStore)

	got, err := r.Resolve(context.Background(), "sched-1", baseTime)
	if err != nil {
		t.Fatal(err)
	}
	// Alice is still on-call; override had no effect
	requireUsers(t, got, "alice")
}

func TestResolve_WeeklyRotationShiftLengthTwo(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 2, // 2-week shifts
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": makeParticipants("rot-1", "alice", "bob"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("alice", "bob")}
	r := NewResolver(schedStore, userStore)

	twoWeeks := 2 * 7 * 24 * time.Hour

	cases := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"week 0", 0, "alice"},
		{"week 1 (still alice)", 7 * 24 * time.Hour, "alice"},
		{"week 2 (bob)", twoWeeks, "bob"},
		{"week 3 (still bob)", twoWeeks + 7*24*time.Hour, "bob"},
		{"week 4 (wraps to alice)", 2 * twoWeeks, "alice"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Resolve(context.Background(), "sched-1", baseTime.Add(tc.offset))
			if err != nil {
				t.Fatal(err)
			}
			requireUsers(t, got, tc.want)
		})
	}
}

func TestResolve_FiveParticipantsWrapAround(t *testing.T) {
	schedStore := &mockScheduleStore{
		rotations: []store.Rotation{{
			ID:          "rot-1",
			ScheduleID:  "sched-1",
			Type:        store.RotationTypeWeekly,
			ShiftLength: 1,
			StartTime:   baseTime,
		}},
		participants: map[string][]store.RotationParticipant{
			"rot-1": makeParticipants("rot-1", "u0", "u1", "u2", "u3", "u4"),
		},
	}
	userStore := &mockUserStore{users: makeUsers("u0", "u1", "u2", "u3", "u4")}
	r := NewResolver(schedStore, userStore)

	oneWeek := 7 * 24 * time.Hour

	cases := []struct {
		name   string
		weeks  int
		want   string
	}{
		{"week 0", 0, "u0"},
		{"week 1", 1, "u1"},
		{"week 2", 2, "u2"},
		{"week 3", 3, "u3"},
		{"week 4", 4, "u4"},
		{"week 5 wraps", 5, "u0"},
		{"week 6", 6, "u1"},
		{"week 10 wraps", 10, "u0"},
		{"week 13", 13, "u3"},
		{"week 99", 99, "u4"}, // 99 % 5 == 4
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := baseTime.Add(time.Duration(tc.weeks) * oneWeek)
			got, err := r.Resolve(context.Background(), "sched-1", at)
			if err != nil {
				t.Fatal(err)
			}
			requireUsers(t, got, tc.want)
		})
	}
}
