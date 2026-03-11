package sqlite

import (
	"context"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

// newTestStore creates an in-memory SQLite store with migrations applied.
// It registers a cleanup function to close the store when the test finishes.
func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()

	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}

	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	t.Cleanup(func() { s.Close() })
	return s
}

// createTestUser creates a user and returns it. ID is auto-generated.
func createTestUser(t *testing.T, s *SQLiteStore, name, email string) *store.User {
	t.Helper()
	u := &store.User{Name: name, Email: email, Role: "user", Timezone: "UTC"}
	if err := s.Users().Create(context.Background(), u); err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return u
}

// createTestEscalationPolicy creates an escalation policy and returns it.
func createTestEscalationPolicy(t *testing.T, s *SQLiteStore, name string) *store.EscalationPolicy {
	t.Helper()
	ep := &store.EscalationPolicy{Name: name}
	if err := s.EscalationPolicies().Create(context.Background(), ep); err != nil {
		t.Fatalf("creating test escalation policy: %v", err)
	}
	return ep
}

// createTestService creates a service with a real escalation policy.
func createTestService(t *testing.T, s *SQLiteStore) *store.Service {
	t.Helper()
	ep := createTestEscalationPolicy(t, s, "test-ep")
	svc := &store.Service{Name: "test-svc", EscalationPolicyID: ep.ID}
	if err := s.Services().Create(context.Background(), svc); err != nil {
		t.Fatalf("creating test service: %v", err)
	}
	return svc
}
