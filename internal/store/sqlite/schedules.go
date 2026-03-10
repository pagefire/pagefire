package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type scheduleStore struct {
	db *sql.DB
}

func (s *scheduleStore) Create(ctx context.Context, sched *store.Schedule) error {
	if sched.ID == "" {
		sched.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schedules (id, name, description, timezone) VALUES (?, ?, ?, ?)`,
		sched.ID, sched.Name, sched.Description, sched.Timezone,
	)
	return err
}

func (s *scheduleStore) Get(ctx context.Context, id string) (*store.Schedule, error) {
	sched := &store.Schedule{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, timezone, created_at FROM schedules WHERE id = ?`, id,
	).Scan(&sched.ID, &sched.Name, &sched.Description, &sched.Timezone, &sched.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return sched, err
}

func (s *scheduleStore) List(ctx context.Context) ([]store.Schedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, timezone, created_at FROM schedules ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []store.Schedule
	for rows.Next() {
		var sched store.Schedule
		if err := rows.Scan(&sched.ID, &sched.Name, &sched.Description, &sched.Timezone, &sched.CreatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, sched)
	}
	return schedules, rows.Err()
}

func (s *scheduleStore) Update(ctx context.Context, sched *store.Schedule) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET name = ?, description = ?, timezone = ? WHERE id = ?`,
		sched.Name, sched.Description, sched.Timezone, sched.ID,
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

func (s *scheduleStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *scheduleStore) CreateRotation(ctx context.Context, r *store.Rotation) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rotations (id, schedule_id, name, type, shift_length, start_time, handoff_time) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ScheduleID, r.Name, r.Type, r.ShiftLength, r.StartTime.UTC(), r.HandoffTime,
	)
	return err
}

func (s *scheduleStore) GetRotation(ctx context.Context, id string) (*store.Rotation, error) {
	r := &store.Rotation{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, schedule_id, name, type, shift_length, start_time, handoff_time, created_at FROM rotations WHERE id = ?`, id,
	).Scan(&r.ID, &r.ScheduleID, &r.Name, &r.Type, &r.ShiftLength, &r.StartTime, &r.HandoffTime, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return r, err
}

func (s *scheduleStore) ListRotations(ctx context.Context, scheduleID string) ([]store.Rotation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, schedule_id, name, type, shift_length, start_time, handoff_time, created_at FROM rotations WHERE schedule_id = ?`, scheduleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rotations []store.Rotation
	for rows.Next() {
		var r store.Rotation
		if err := rows.Scan(&r.ID, &r.ScheduleID, &r.Name, &r.Type, &r.ShiftLength, &r.StartTime, &r.HandoffTime, &r.CreatedAt); err != nil {
			return nil, err
		}
		rotations = append(rotations, r)
	}
	return rotations, rows.Err()
}

func (s *scheduleStore) DeleteRotation(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM rotations WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *scheduleStore) CreateParticipant(ctx context.Context, p *store.RotationParticipant) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rotation_participants (id, rotation_id, user_id, position) VALUES (?, ?, ?, ?)`,
		p.ID, p.RotationID, p.UserID, p.Position,
	)
	return err
}

func (s *scheduleStore) ListParticipants(ctx context.Context, rotationID string) ([]store.RotationParticipant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, rotation_id, user_id, position FROM rotation_participants WHERE rotation_id = ? ORDER BY position`, rotationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []store.RotationParticipant
	for rows.Next() {
		var p store.RotationParticipant
		if err := rows.Scan(&p.ID, &p.RotationID, &p.UserID, &p.Position); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (s *scheduleStore) DeleteParticipant(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM rotation_participants WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *scheduleStore) CreateOverride(ctx context.Context, o *store.ScheduleOverride) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schedule_overrides (id, schedule_id, start_time, end_time, replace_user, override_user) VALUES (?, ?, ?, ?, ?, ?)`,
		o.ID, o.ScheduleID, o.StartTime.UTC(), o.EndTime.UTC(), o.ReplaceUser, o.OverrideUser,
	)
	return err
}

func (s *scheduleStore) ListOverrides(ctx context.Context, scheduleID string) ([]store.ScheduleOverride, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, schedule_id, start_time, end_time, replace_user, override_user, created_at FROM schedule_overrides WHERE schedule_id = ? ORDER BY start_time`, scheduleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []store.ScheduleOverride
	for rows.Next() {
		var o store.ScheduleOverride
		if err := rows.Scan(&o.ID, &o.ScheduleID, &o.StartTime, &o.EndTime, &o.ReplaceUser, &o.OverrideUser, &o.CreatedAt); err != nil {
			return nil, err
		}
		overrides = append(overrides, o)
	}
	return overrides, rows.Err()
}

func (s *scheduleStore) ListActiveOverrides(ctx context.Context, scheduleID string, at time.Time) ([]store.ScheduleOverride, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, schedule_id, start_time, end_time, replace_user, override_user, created_at
		 FROM schedule_overrides
		 WHERE schedule_id = ? AND start_time <= ? AND end_time > ?
		 ORDER BY start_time`,
		scheduleID, at.UTC(), at.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []store.ScheduleOverride
	for rows.Next() {
		var o store.ScheduleOverride
		if err := rows.Scan(&o.ID, &o.ScheduleID, &o.StartTime, &o.EndTime, &o.ReplaceUser, &o.OverrideUser, &o.CreatedAt); err != nil {
			return nil, err
		}
		overrides = append(overrides, o)
	}
	return overrides, rows.Err()
}

func (s *scheduleStore) DeleteOverride(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM schedule_overrides WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}
