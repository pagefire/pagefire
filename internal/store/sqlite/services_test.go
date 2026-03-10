package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

func TestServiceCreate_Get(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	svc := &store.Service{
		Name:               "payment-api",
		Description:        "Handles payments",
		EscalationPolicyID: "ep-1",
	}

	if err := services.Create(ctx, svc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if svc.ID == "" {
		t.Fatal("expected ID to be auto-generated")
	}

	got, err := services.Get(ctx, svc.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != svc.ID {
		t.Errorf("ID = %q, want %q", got.ID, svc.ID)
	}
	if got.Name != "payment-api" {
		t.Errorf("Name = %q, want %q", got.Name, "payment-api")
	}
	if got.Description != "Handles payments" {
		t.Errorf("Description = %q, want %q", got.Description, "Handles payments")
	}
	if got.EscalationPolicyID != "ep-1" {
		t.Errorf("EscalationPolicyID = %q, want %q", got.EscalationPolicyID, "ep-1")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestServiceGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	_, err := services.Get(ctx, "nonexistent-id")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Get nonexistent: got err %v, want ErrNotFound", err)
	}
}

func TestServiceList(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	// Empty list.
	list, err := services.List(ctx)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List empty: got %d, want 0", len(list))
	}

	// Create two services; names chosen to verify ORDER BY name.
	for _, name := range []string{"bravo-svc", "alpha-svc"} {
		if err := services.Create(ctx, &store.Service{
			Name:               name,
			EscalationPolicyID: "ep-1",
		}); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	list, err = services.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List: got %d, want 2", len(list))
	}
	if list[0].Name != "alpha-svc" {
		t.Errorf("List[0].Name = %q, want %q (ordered by name)", list[0].Name, "alpha-svc")
	}
	if list[1].Name != "bravo-svc" {
		t.Errorf("List[1].Name = %q, want %q", list[1].Name, "bravo-svc")
	}
}

func TestServiceUpdate(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	svc := &store.Service{
		Name:               "old-name",
		Description:        "old-desc",
		EscalationPolicyID: "ep-1",
	}
	if err := services.Create(ctx, svc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	svc.Name = "new-name"
	svc.Description = "new-desc"
	svc.EscalationPolicyID = "ep-2"
	if err := services.Update(ctx, svc); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := services.Get(ctx, svc.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Name != "new-name" {
		t.Errorf("Name = %q, want %q", got.Name, "new-name")
	}
	if got.Description != "new-desc" {
		t.Errorf("Description = %q, want %q", got.Description, "new-desc")
	}
	if got.EscalationPolicyID != "ep-2" {
		t.Errorf("EscalationPolicyID = %q, want %q", got.EscalationPolicyID, "ep-2")
	}
}

func TestServiceUpdate_NotFound(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	err := services.Update(ctx, &store.Service{ID: "ghost", Name: "x", EscalationPolicyID: "ep-1"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Update nonexistent: got err %v, want ErrNotFound", err)
	}
}

func TestServiceDelete(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	svc := &store.Service{Name: "doomed", EscalationPolicyID: "ep-1"}
	if err := services.Create(ctx, svc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := services.Delete(ctx, svc.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := services.Get(ctx, svc.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Get after delete: got err %v, want ErrNotFound", err)
	}
}

func TestServiceDelete_NotFound(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	err := services.Delete(ctx, "nonexistent-id")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Delete nonexistent: got err %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// Integration key tests
// ---------------------------------------------------------------------------

func TestIntegrationKeyCreate(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()
	svc := createTestService(t, s)

	ik := &store.IntegrationKey{
		ServiceID: svc.ID,
		Name:      "grafana-key",
		Type:      "grafana",
	}
	if err := services.CreateIntegrationKey(ctx, ik); err != nil {
		t.Fatalf("CreateIntegrationKey: %v", err)
	}

	if ik.ID == "" {
		t.Error("expected ID to be auto-generated")
	}
	if len(ik.Secret) != 64 {
		t.Errorf("Secret length = %d, want 64 hex chars", len(ik.Secret))
	}
	// Verify it is valid hex (all chars in [0-9a-f]).
	for _, c := range ik.Secret {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("Secret contains non-hex char %q", string(c))
		}
	}
}

func TestIntegrationKeyList(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()
	svc := createTestService(t, s)

	// Empty list.
	keys, err := services.ListIntegrationKeys(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListIntegrationKeys empty: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("ListIntegrationKeys empty: got %d, want 0", len(keys))
	}

	// Create two keys.
	for _, name := range []string{"key-a", "key-b"} {
		if err := services.CreateIntegrationKey(ctx, &store.IntegrationKey{
			ServiceID: svc.ID,
			Name:      name,
			Type:      "generic",
		}); err != nil {
			t.Fatalf("CreateIntegrationKey %s: %v", name, err)
		}
	}

	keys, err = services.ListIntegrationKeys(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListIntegrationKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("ListIntegrationKeys: got %d, want 2", len(keys))
	}
}

func TestGetIntegrationKeyBySecret(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()
	svc := createTestService(t, s)

	ik := &store.IntegrationKey{
		ServiceID: svc.ID,
		Name:      "lookup-key",
		Type:      "prometheus",
	}
	if err := services.CreateIntegrationKey(ctx, ik); err != nil {
		t.Fatalf("CreateIntegrationKey: %v", err)
	}

	got, err := services.GetIntegrationKeyBySecret(ctx, ik.Secret)
	if err != nil {
		t.Fatalf("GetIntegrationKeyBySecret: %v", err)
	}

	if got.ID != ik.ID {
		t.Errorf("ID = %q, want %q", got.ID, ik.ID)
	}
	if got.ServiceID != svc.ID {
		t.Errorf("ServiceID = %q, want %q", got.ServiceID, svc.ID)
	}
	if got.Name != "lookup-key" {
		t.Errorf("Name = %q, want %q", got.Name, "lookup-key")
	}
	if got.Type != "prometheus" {
		t.Errorf("Type = %q, want %q", got.Type, "prometheus")
	}
	if got.Secret != ik.Secret {
		t.Errorf("Secret = %q, want %q", got.Secret, ik.Secret)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestGetIntegrationKeyBySecret_NotFound(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()

	_, err := services.GetIntegrationKeyBySecret(ctx, "bogus-secret-value")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetIntegrationKeyBySecret wrong secret: got err %v, want ErrNotFound", err)
	}
}

func TestIntegrationKeyDelete(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()
	svc := createTestService(t, s)

	ik := &store.IntegrationKey{
		ServiceID: svc.ID,
		Name:      "to-delete",
		Type:      "generic",
	}
	if err := services.CreateIntegrationKey(ctx, ik); err != nil {
		t.Fatalf("CreateIntegrationKey: %v", err)
	}

	if err := services.DeleteIntegrationKey(ctx, ik.ID); err != nil {
		t.Fatalf("DeleteIntegrationKey: %v", err)
	}

	// Verify gone via list.
	keys, err := services.ListIntegrationKeys(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListIntegrationKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("ListIntegrationKeys after delete: got %d, want 0", len(keys))
	}

	// Delete again should be not found.
	err = services.DeleteIntegrationKey(ctx, ik.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DeleteIntegrationKey twice: got err %v, want ErrNotFound", err)
	}
}

func TestDeleteServiceCascadesIntegrationKeys(t *testing.T) {
	s := newTestStore(t)
	services := s.Services()
	ctx := context.Background()
	svc := createTestService(t, s)

	// Create two integration keys for the service.
	for i := 0; i < 2; i++ {
		if err := services.CreateIntegrationKey(ctx, &store.IntegrationKey{
			ServiceID: svc.ID,
			Name:      "cascade-key",
			Type:      "generic",
		}); err != nil {
			t.Fatalf("CreateIntegrationKey: %v", err)
		}
	}

	// Verify keys exist.
	keys, err := services.ListIntegrationKeys(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListIntegrationKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys before delete, got %d", len(keys))
	}

	// Delete the service.
	if err := services.Delete(ctx, svc.ID); err != nil {
		t.Fatalf("Delete service: %v", err)
	}

	// Integration keys should be cascade-deleted.
	keys, err = services.ListIntegrationKeys(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListIntegrationKeys after cascade: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys after cascade delete, got %d", len(keys))
	}
}
