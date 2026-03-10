package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

func seedUser(t *testing.T, users store.UserStore, name, email string) *store.User {
	t.Helper()
	u := &store.User{
		Name:      name,
		Email:     email,
		Role:      "admin",
		Timezone:  "America/New_York",
		AvatarURL: "https://example.com/avatar.png",
	}
	if err := users.Create(context.Background(), u); err != nil {
		t.Fatalf("seed user %q: %v", name, err)
	}
	return u
}

func TestUserCreateAndGetByID(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := &store.User{
		Name:      "Alice",
		Email:     "alice@example.com",
		Role:      "admin",
		Timezone:  "UTC",
		AvatarURL: "https://example.com/alice.png",
	}

	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected ID to be assigned after Create")
	}

	got, err := users.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != u.ID {
		t.Errorf("ID = %q, want %q", got.ID, u.ID)
	}
	if got.Name != "Alice" {
		t.Errorf("Name = %q, want %q", got.Name, "Alice")
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "alice@example.com")
	}
	if got.Role != "admin" {
		t.Errorf("Role = %q, want %q", got.Role, "admin")
	}
	if got.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", got.Timezone, "UTC")
	}
	if got.AvatarURL != "https://example.com/alice.png" {
		t.Errorf("AvatarURL = %q, want %q", got.AvatarURL, "https://example.com/alice.png")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestUserGetByEmail(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "Bob", "bob@example.com")

	got, err := users.GetByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID = %q, want %q", got.ID, u.ID)
	}
	if got.Name != "Bob" {
		t.Errorf("Name = %q, want %q", got.Name, "Bob")
	}
	if got.Email != "bob@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "bob@example.com")
	}
}

func TestUserDuplicateEmail(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	seedUser(t, users, "First", "dup@example.com")

	second := &store.User{
		Name:     "Second",
		Email:    "dup@example.com",
		Role:     "user",
		Timezone: "UTC",
	}
	err := users.Create(ctx, second)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
}

func TestUserGetNotFound(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Get by non-existent ID",
			fn: func() error {
				_, err := users.Get(ctx, "no-such-id")
				return err
			},
		},
		{
			name: "GetByEmail non-existent",
			fn: func() error {
				_, err := users.GetByEmail(ctx, "nobody@example.com")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, store.ErrNotFound) {
				t.Errorf("err = %v, want store.ErrNotFound", err)
			}
		})
	}
}

func TestUserList(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	// List should return empty initially.
	list, err := users.List(ctx)
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 users, got %d", len(list))
	}

	// Insert users out of alphabetical order to verify ORDER BY name.
	seedUser(t, users, "Charlie", "charlie@example.com")
	seedUser(t, users, "Alice", "alice@example.com")
	seedUser(t, users, "Bob", "bob@example.com")

	list, err = users.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 users, got %d", len(list))
	}

	// Verify alphabetical ordering.
	wantNames := []string{"Alice", "Bob", "Charlie"}
	for i, want := range wantNames {
		if list[i].Name != want {
			t.Errorf("list[%d].Name = %q, want %q", i, list[i].Name, want)
		}
	}
}

func TestUserUpdate(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "Original", "orig@example.com")

	u.Name = "Updated"
	u.Email = "updated@example.com"
	u.Role = "user"
	u.Timezone = "Europe/London"
	u.AvatarURL = "https://example.com/new.png"

	if err := users.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := users.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}

	if got.Name != "Updated" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated")
	}
	if got.Email != "updated@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "updated@example.com")
	}
	if got.Role != "user" {
		t.Errorf("Role = %q, want %q", got.Role, "user")
	}
	if got.Timezone != "Europe/London" {
		t.Errorf("Timezone = %q, want %q", got.Timezone, "Europe/London")
	}
	if got.AvatarURL != "https://example.com/new.png" {
		t.Errorf("AvatarURL = %q, want %q", got.AvatarURL, "https://example.com/new.png")
	}
}

func TestUserUpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	err := users.Update(ctx, &store.User{ID: "nonexistent", Name: "X", Email: "x@x.com"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Update non-existent: err = %v, want store.ErrNotFound", err)
	}
}

func TestUserDelete(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "ToDelete", "delete@example.com")

	if err := users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := users.Get(ctx, u.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get after Delete: err = %v, want store.ErrNotFound", err)
	}
}

func TestUserDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	err := users.Delete(ctx, "no-such-id")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete non-existent: err = %v, want store.ErrNotFound", err)
	}
}

func TestContactMethodCreateAndList(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "CMUser", "cm@example.com")

	// List should be empty initially.
	methods, err := users.ListContactMethods(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListContactMethods (empty): %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected 0 contact methods, got %d", len(methods))
	}

	cm := &store.ContactMethod{
		UserID: u.ID,
		Type:   "email",
		Value:  "cm@example.com",
	}
	if err := users.CreateContactMethod(ctx, cm); err != nil {
		t.Fatalf("CreateContactMethod: %v", err)
	}
	if cm.ID == "" {
		t.Fatal("expected ID to be assigned after CreateContactMethod")
	}

	// Add a second contact method.
	cm2 := &store.ContactMethod{
		UserID: u.ID,
		Type:   "sms",
		Value:  "+15551234567",
	}
	if err := users.CreateContactMethod(ctx, cm2); err != nil {
		t.Fatalf("CreateContactMethod (sms): %v", err)
	}

	methods, err = users.ListContactMethods(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListContactMethods: %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected 2 contact methods, got %d", len(methods))
	}

	// Verify fields of the first contact method (order may vary, find by ID).
	found := false
	for _, m := range methods {
		if m.ID == cm.ID {
			found = true
			if m.UserID != u.ID {
				t.Errorf("UserID = %q, want %q", m.UserID, u.ID)
			}
			if m.Type != "email" {
				t.Errorf("Type = %q, want %q", m.Type, "email")
			}
			if m.Value != "cm@example.com" {
				t.Errorf("Value = %q, want %q", m.Value, "cm@example.com")
			}
			if m.Verified {
				t.Error("Verified should default to false")
			}
		}
	}
	if !found {
		t.Errorf("contact method %q not found in list", cm.ID)
	}
}

func TestContactMethodDelete(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "CMDelete", "cmdel@example.com")

	cm := &store.ContactMethod{
		UserID: u.ID,
		Type:   "email",
		Value:  "cmdel@example.com",
	}
	if err := users.CreateContactMethod(ctx, cm); err != nil {
		t.Fatalf("CreateContactMethod: %v", err)
	}

	if err := users.DeleteContactMethod(ctx, cm.ID); err != nil {
		t.Fatalf("DeleteContactMethod: %v", err)
	}

	methods, err := users.ListContactMethods(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListContactMethods after delete: %v", err)
	}
	if len(methods) != 0 {
		t.Errorf("expected 0 contact methods after delete, got %d", len(methods))
	}
}

func TestContactMethodDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	err := users.DeleteContactMethod(ctx, "no-such-cm")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("DeleteContactMethod non-existent: err = %v, want store.ErrNotFound", err)
	}
}

func TestNotificationRuleCreateAndList(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "NRUser", "nr@example.com")

	cm := &store.ContactMethod{
		UserID: u.ID,
		Type:   "email",
		Value:  "nr@example.com",
	}
	if err := users.CreateContactMethod(ctx, cm); err != nil {
		t.Fatalf("CreateContactMethod: %v", err)
	}

	// List should be empty initially.
	rules, err := users.ListNotificationRules(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListNotificationRules (empty): %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 notification rules, got %d", len(rules))
	}

	nr := &store.NotificationRule{
		UserID:          u.ID,
		ContactMethodID: cm.ID,
		DelayMinutes:    0,
	}
	if err := users.CreateNotificationRule(ctx, nr); err != nil {
		t.Fatalf("CreateNotificationRule: %v", err)
	}
	if nr.ID == "" {
		t.Fatal("expected ID to be assigned after CreateNotificationRule")
	}

	// Add a second rule with a delay.
	nr2 := &store.NotificationRule{
		UserID:          u.ID,
		ContactMethodID: cm.ID,
		DelayMinutes:    15,
	}
	if err := users.CreateNotificationRule(ctx, nr2); err != nil {
		t.Fatalf("CreateNotificationRule (delayed): %v", err)
	}

	rules, err = users.ListNotificationRules(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListNotificationRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 notification rules, got %d", len(rules))
	}

	// Verify fields by finding the first rule by ID.
	found := false
	for _, r := range rules {
		if r.ID == nr.ID {
			found = true
			if r.UserID != u.ID {
				t.Errorf("UserID = %q, want %q", r.UserID, u.ID)
			}
			if r.ContactMethodID != cm.ID {
				t.Errorf("ContactMethodID = %q, want %q", r.ContactMethodID, cm.ID)
			}
			if r.DelayMinutes != 0 {
				t.Errorf("DelayMinutes = %d, want 0", r.DelayMinutes)
			}
		}
	}
	if !found {
		t.Errorf("notification rule %q not found in list", nr.ID)
	}
}

func TestNotificationRuleDelete(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	u := seedUser(t, users, "NRDelete", "nrdel@example.com")

	cm := &store.ContactMethod{
		UserID: u.ID,
		Type:   "sms",
		Value:  "+15559876543",
	}
	if err := users.CreateContactMethod(ctx, cm); err != nil {
		t.Fatalf("CreateContactMethod: %v", err)
	}

	nr := &store.NotificationRule{
		UserID:          u.ID,
		ContactMethodID: cm.ID,
		DelayMinutes:    5,
	}
	if err := users.CreateNotificationRule(ctx, nr); err != nil {
		t.Fatalf("CreateNotificationRule: %v", err)
	}

	if err := users.DeleteNotificationRule(ctx, nr.ID); err != nil {
		t.Fatalf("DeleteNotificationRule: %v", err)
	}

	rules, err := users.ListNotificationRules(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListNotificationRules after delete: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 notification rules after delete, got %d", len(rules))
	}
}

func TestNotificationRuleDeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	users := s.Users()
	ctx := context.Background()

	err := users.DeleteNotificationRule(ctx, "no-such-nr")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("DeleteNotificationRule non-existent: err = %v, want store.ErrNotFound", err)
	}
}
