package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/pagefire/pagefire/internal/store"
)

type teamStore struct {
	db *sql.DB
}

func (s *teamStore) Create(ctx context.Context, t *store.Team) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO teams (id, name, description) VALUES (?, ?, ?)`,
		t.ID, t.Name, t.Description,
	)
	return err
}

func (s *teamStore) Get(ctx context.Context, id string) (*store.Team, error) {
	t := &store.Team{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at FROM teams WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	return t, err
}

func (s *teamStore) List(ctx context.Context) ([]store.Team, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, created_at FROM teams ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []store.Team
	for rows.Next() {
		var t store.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *teamStore) Update(ctx context.Context, t *store.Team) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE teams SET name = ?, description = ? WHERE id = ?`,
		t.Name, t.Description, t.ID,
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

func (s *teamStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM teams WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *teamStore) AddMember(ctx context.Context, teamID, userID, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)`,
		teamID, userID, role,
	)
	return err
}

func (s *teamStore) RemoveMember(ctx context.Context, teamID, userID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
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

func (s *teamStore) ListMembers(ctx context.Context, teamID string) ([]store.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT team_id, user_id, role FROM team_members WHERE team_id = ? ORDER BY role, user_id`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []store.TeamMember
	for rows.Next() {
		var m store.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *teamStore) ListTeamsForUser(ctx context.Context, userID string) ([]store.Team, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.description, t.created_at
		 FROM teams t
		 JOIN team_members tm ON t.id = tm.team_id
		 WHERE tm.user_id = ?
		 ORDER BY t.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []store.Team
	for rows.Next() {
		var t store.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}
