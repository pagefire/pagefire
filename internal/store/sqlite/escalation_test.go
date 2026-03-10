package sqlite

import (
	"context"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

func TestEscalationPolicy_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{
		Name:        "P1 Policy",
		Description: "Critical alerts",
		Repeat:      3,
	}
	if err := s.EscalationPolicies().Create(ctx, ep); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ep.ID == "" {
		t.Fatal("expected ID to be set after Create")
	}

	got, err := s.EscalationPolicies().Get(ctx, ep.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "P1 Policy" {
		t.Errorf("Name = %q, want %q", got.Name, "P1 Policy")
	}
	if got.Description != "Critical alerts" {
		t.Errorf("Description = %q, want %q", got.Description, "Critical alerts")
	}
	if got.Repeat != 3 {
		t.Errorf("Repeat = %d, want 3", got.Repeat)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestEscalationPolicy_GetNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.EscalationPolicies().Get(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("Get non-existent: got %v, want ErrNotFound", err)
	}
}

func TestEscalationPolicy_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	// Empty list.
	got, err := eps.List(ctx)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List empty: got %d, want 0", len(got))
	}

	// Create two policies; List orders by name.
	if err := eps.Create(ctx, &store.EscalationPolicy{Name: "Bravo"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := eps.Create(ctx, &store.EscalationPolicy{Name: "Alpha"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err = eps.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List: got %d items, want 2", len(got))
	}
	if got[0].Name != "Alpha" || got[1].Name != "Bravo" {
		t.Errorf("List order: got [%q, %q], want [Alpha, Bravo]", got[0].Name, got[1].Name)
	}
}

func TestEscalationPolicy_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "Original", Repeat: 1}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create: %v", err)
	}

	ep.Name = "Updated"
	ep.Description = "new desc"
	ep.Repeat = 5
	if err := eps.Update(ctx, ep); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := eps.Get(ctx, ep.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("Name = %q, want Updated", got.Name)
	}
	if got.Description != "new desc" {
		t.Errorf("Description = %q, want 'new desc'", got.Description)
	}
	if got.Repeat != 5 {
		t.Errorf("Repeat = %d, want 5", got.Repeat)
	}
}

func TestEscalationPolicy_UpdateNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.EscalationPolicies().Update(ctx, &store.EscalationPolicy{ID: "nonexistent"})
	if err != store.ErrNotFound {
		t.Fatalf("Update non-existent: got %v, want ErrNotFound", err)
	}
}

func TestEscalationPolicy_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "To Delete"}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := eps.Delete(ctx, ep.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := eps.Get(ctx, ep.ID)
	if err != store.ErrNotFound {
		t.Fatalf("Get after delete: got %v, want ErrNotFound", err)
	}
}

func TestEscalationPolicy_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.EscalationPolicies().Delete(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("Delete non-existent: got %v, want ErrNotFound", err)
	}
}

func TestEscalationStep_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "Policy"}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create policy: %v", err)
	}

	// Create steps out of order to verify ordering by step_number.
	step2 := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 2, DelayMinutes: 10}
	step1 := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 1, DelayMinutes: 5}
	if err := eps.CreateStep(ctx, step2); err != nil {
		t.Fatalf("CreateStep 2: %v", err)
	}
	if err := eps.CreateStep(ctx, step1); err != nil {
		t.Fatalf("CreateStep 1: %v", err)
	}
	if step1.ID == "" || step2.ID == "" {
		t.Fatal("expected step IDs to be set")
	}

	steps, err := eps.ListSteps(ctx, ep.ID)
	if err != nil {
		t.Fatalf("ListSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("ListSteps: got %d, want 2", len(steps))
	}
	if steps[0].StepNumber != 1 {
		t.Errorf("first step number = %d, want 1", steps[0].StepNumber)
	}
	if steps[1].StepNumber != 2 {
		t.Errorf("second step number = %d, want 2", steps[1].StepNumber)
	}
	if steps[0].DelayMinutes != 5 {
		t.Errorf("first step delay = %d, want 5", steps[0].DelayMinutes)
	}
}

func TestEscalationStep_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "Policy"}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create policy: %v", err)
	}

	step := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 1, DelayMinutes: 5}
	if err := eps.CreateStep(ctx, step); err != nil {
		t.Fatalf("CreateStep: %v", err)
	}

	if err := eps.DeleteStep(ctx, step.ID); err != nil {
		t.Fatalf("DeleteStep: %v", err)
	}

	steps, err := eps.ListSteps(ctx, ep.ID)
	if err != nil {
		t.Fatalf("ListSteps: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("ListSteps after delete: got %d, want 0", len(steps))
	}
}

func TestEscalationStep_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.EscalationPolicies().DeleteStep(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("DeleteStep non-existent: got %v, want ErrNotFound", err)
	}
}

func TestEscalationStepTarget_CreateAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "Policy"}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create policy: %v", err)
	}
	step := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 1, DelayMinutes: 5}
	if err := eps.CreateStep(ctx, step); err != nil {
		t.Fatalf("CreateStep: %v", err)
	}

	t1 := &store.EscalationStepTarget{
		EscalationStepID: step.ID,
		TargetType:       store.TargetTypeUser,
		TargetID:         "user-1",
	}
	t2 := &store.EscalationStepTarget{
		EscalationStepID: step.ID,
		TargetType:       store.TargetTypeSchedule,
		TargetID:         "schedule-1",
	}
	if err := eps.CreateStepTarget(ctx, t1); err != nil {
		t.Fatalf("CreateStepTarget 1: %v", err)
	}
	if err := eps.CreateStepTarget(ctx, t2); err != nil {
		t.Fatalf("CreateStepTarget 2: %v", err)
	}
	if t1.ID == "" || t2.ID == "" {
		t.Fatal("expected target IDs to be set")
	}

	targets, err := eps.ListStepTargets(ctx, step.ID)
	if err != nil {
		t.Fatalf("ListStepTargets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("ListStepTargets: got %d, want 2", len(targets))
	}

	// Verify target data is correct (check that both types are present).
	types := map[string]bool{}
	for _, tgt := range targets {
		types[tgt.TargetType] = true
		if tgt.EscalationStepID != step.ID {
			t.Errorf("target.EscalationStepID = %q, want %q", tgt.EscalationStepID, step.ID)
		}
	}
	if !types[store.TargetTypeUser] || !types[store.TargetTypeSchedule] {
		t.Errorf("expected both target types, got %v", types)
	}
}

func TestEscalationStepTarget_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "Policy"}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create policy: %v", err)
	}
	step := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 1, DelayMinutes: 5}
	if err := eps.CreateStep(ctx, step); err != nil {
		t.Fatalf("CreateStep: %v", err)
	}
	target := &store.EscalationStepTarget{
		EscalationStepID: step.ID,
		TargetType:       store.TargetTypeUser,
		TargetID:         "user-1",
	}
	if err := eps.CreateStepTarget(ctx, target); err != nil {
		t.Fatalf("CreateStepTarget: %v", err)
	}

	if err := eps.DeleteStepTarget(ctx, target.ID); err != nil {
		t.Fatalf("DeleteStepTarget: %v", err)
	}

	targets, err := eps.ListStepTargets(ctx, step.ID)
	if err != nil {
		t.Fatalf("ListStepTargets: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("ListStepTargets after delete: got %d, want 0", len(targets))
	}
}

func TestEscalationStepTarget_DeleteNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.EscalationPolicies().DeleteStepTarget(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("DeleteStepTarget non-existent: got %v, want ErrNotFound", err)
	}
}

func TestEscalation_GetFullPolicy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	// Build a full policy tree: policy -> 2 steps -> targets on each step.
	ep := &store.EscalationPolicy{Name: "Full Policy", Description: "full", Repeat: 2}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create policy: %v", err)
	}

	step1 := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 1, DelayMinutes: 0}
	step2 := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 2, DelayMinutes: 10}
	if err := eps.CreateStep(ctx, step1); err != nil {
		t.Fatalf("CreateStep 1: %v", err)
	}
	if err := eps.CreateStep(ctx, step2); err != nil {
		t.Fatalf("CreateStep 2: %v", err)
	}

	if err := eps.CreateStepTarget(ctx, &store.EscalationStepTarget{
		EscalationStepID: step1.ID, TargetType: store.TargetTypeUser, TargetID: "user-a",
	}); err != nil {
		t.Fatalf("CreateStepTarget s1t1: %v", err)
	}
	if err := eps.CreateStepTarget(ctx, &store.EscalationStepTarget{
		EscalationStepID: step2.ID, TargetType: store.TargetTypeSchedule, TargetID: "sched-b",
	}); err != nil {
		t.Fatalf("CreateStepTarget s2t1: %v", err)
	}

	snap, err := eps.GetFullPolicy(ctx, ep.ID)
	if err != nil {
		t.Fatalf("GetFullPolicy: %v", err)
	}

	if snap.PolicyID != ep.ID {
		t.Errorf("PolicyID = %q, want %q", snap.PolicyID, ep.ID)
	}
	if snap.PolicyName != "Full Policy" {
		t.Errorf("PolicyName = %q, want %q", snap.PolicyName, "Full Policy")
	}
	if snap.Repeat != 2 {
		t.Errorf("Repeat = %d, want 2", snap.Repeat)
	}
	if len(snap.Steps) != 2 {
		t.Fatalf("Steps count = %d, want 2", len(snap.Steps))
	}
	if snap.Steps[0].StepNumber != 1 || snap.Steps[1].StepNumber != 2 {
		t.Errorf("step ordering incorrect: [%d, %d]", snap.Steps[0].StepNumber, snap.Steps[1].StepNumber)
	}
	if len(snap.Steps[0].Targets) != 1 {
		t.Fatalf("Step 1 targets = %d, want 1", len(snap.Steps[0].Targets))
	}
	if snap.Steps[0].Targets[0].TargetType != store.TargetTypeUser {
		t.Errorf("Step 1 target type = %q, want %q", snap.Steps[0].Targets[0].TargetType, store.TargetTypeUser)
	}
	if len(snap.Steps[1].Targets) != 1 {
		t.Fatalf("Step 2 targets = %d, want 1", len(snap.Steps[1].Targets))
	}
	if snap.Steps[1].Targets[0].TargetType != store.TargetTypeSchedule {
		t.Errorf("Step 2 target type = %q, want %q", snap.Steps[1].Targets[0].TargetType, store.TargetTypeSchedule)
	}
}

func TestEscalation_GetFullPolicyNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.EscalationPolicies().GetFullPolicy(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Fatalf("GetFullPolicy non-existent: got %v, want ErrNotFound", err)
	}
}

func TestEscalation_DeletePolicyCascade(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eps := s.EscalationPolicies()

	ep := &store.EscalationPolicy{Name: "Cascade Policy"}
	if err := eps.Create(ctx, ep); err != nil {
		t.Fatalf("Create policy: %v", err)
	}
	step := &store.EscalationStep{EscalationPolicyID: ep.ID, StepNumber: 1, DelayMinutes: 5}
	if err := eps.CreateStep(ctx, step); err != nil {
		t.Fatalf("CreateStep: %v", err)
	}
	target := &store.EscalationStepTarget{
		EscalationStepID: step.ID, TargetType: store.TargetTypeUser, TargetID: "user-1",
	}
	if err := eps.CreateStepTarget(ctx, target); err != nil {
		t.Fatalf("CreateStepTarget: %v", err)
	}

	// Delete the policy; steps and targets should cascade.
	if err := eps.Delete(ctx, ep.ID); err != nil {
		t.Fatalf("Delete policy: %v", err)
	}

	steps, err := eps.ListSteps(ctx, ep.ID)
	if err != nil {
		t.Fatalf("ListSteps after cascade: %v", err)
	}
	if len(steps) != 0 {
		t.Errorf("steps after cascade: got %d, want 0", len(steps))
	}

	targets, err := eps.ListStepTargets(ctx, step.ID)
	if err != nil {
		t.Fatalf("ListStepTargets after cascade: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("targets after cascade: got %d, want 0", len(targets))
	}
}
