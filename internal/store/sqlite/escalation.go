package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type escalationPolicyStore struct {
	db *sql.DB
}

func (s *escalationPolicyStore) Create(ctx context.Context, ep *store.EscalationPolicy) error {
	if ep.ID == "" {
		ep.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO escalation_policies (id, name, description, repeat) VALUES (?, ?, ?, ?)`,
		ep.ID, ep.Name, ep.Description, ep.Repeat,
	)
	return err
}

func (s *escalationPolicyStore) Get(ctx context.Context, id string) (*store.EscalationPolicy, error) {
	ep := &store.EscalationPolicy{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, repeat, created_at FROM escalation_policies WHERE id = ?`, id,
	).Scan(&ep.ID, &ep.Name, &ep.Description, &ep.Repeat, &ep.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return ep, err
}

func (s *escalationPolicyStore) List(ctx context.Context) ([]store.EscalationPolicy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, repeat, created_at FROM escalation_policies ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []store.EscalationPolicy
	for rows.Next() {
		var ep store.EscalationPolicy
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.Description, &ep.Repeat, &ep.CreatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, ep)
	}
	return policies, rows.Err()
}

func (s *escalationPolicyStore) Update(ctx context.Context, ep *store.EscalationPolicy) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE escalation_policies SET name = ?, description = ?, repeat = ? WHERE id = ?`,
		ep.Name, ep.Description, ep.Repeat, ep.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *escalationPolicyStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM escalation_policies WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *escalationPolicyStore) CreateStep(ctx context.Context, step *store.EscalationStep) error {
	if step.ID == "" {
		step.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO escalation_steps (id, escalation_policy_id, step_number, delay_minutes) VALUES (?, ?, ?, ?)`,
		step.ID, step.EscalationPolicyID, step.StepNumber, step.DelayMinutes,
	)
	return err
}

func (s *escalationPolicyStore) ListSteps(ctx context.Context, policyID string) ([]store.EscalationStep, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, escalation_policy_id, step_number, delay_minutes FROM escalation_steps WHERE escalation_policy_id = ? ORDER BY step_number`,
		policyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []store.EscalationStep
	for rows.Next() {
		var step store.EscalationStep
		if err := rows.Scan(&step.ID, &step.EscalationPolicyID, &step.StepNumber, &step.DelayMinutes); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *escalationPolicyStore) DeleteStep(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM escalation_steps WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *escalationPolicyStore) CreateStepTarget(ctx context.Context, target *store.EscalationStepTarget) error {
	if target.ID == "" {
		target.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO escalation_step_targets (id, escalation_step_id, target_type, target_id) VALUES (?, ?, ?, ?)`,
		target.ID, target.EscalationStepID, target.TargetType, target.TargetID,
	)
	return err
}

func (s *escalationPolicyStore) ListStepTargets(ctx context.Context, stepID string) ([]store.EscalationStepTarget, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, escalation_step_id, target_type, target_id FROM escalation_step_targets WHERE escalation_step_id = ?`,
		stepID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []store.EscalationStepTarget
	for rows.Next() {
		var t store.EscalationStepTarget
		if err := rows.Scan(&t.ID, &t.EscalationStepID, &t.TargetType, &t.TargetID); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (s *escalationPolicyStore) DeleteStepTarget(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM escalation_step_targets WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// GetFullPolicy returns a complete snapshot of the escalation policy tree.
func (s *escalationPolicyStore) GetFullPolicy(ctx context.Context, id string) (*store.EscalationSnapshot, error) {
	ep, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	steps, err := s.ListSteps(ctx, id)
	if err != nil {
		return nil, err
	}

	snapshot := &store.EscalationSnapshot{
		PolicyID:   ep.ID,
		PolicyName: ep.Name,
		Repeat:     ep.Repeat,
	}

	for _, step := range steps {
		targets, err := s.ListStepTargets(ctx, step.ID)
		if err != nil {
			return nil, err
		}

		stepSnap := store.EscalationStepSnapshot{
			StepNumber:   step.StepNumber,
			DelayMinutes: step.DelayMinutes,
		}
		for _, t := range targets {
			stepSnap.Targets = append(stepSnap.Targets, store.TargetSnapshot{
				TargetType: t.TargetType,
				TargetID:   t.TargetID,
			})
		}
		snapshot.Steps = append(snapshot.Steps, stepSnap)
	}

	return snapshot, nil
}
