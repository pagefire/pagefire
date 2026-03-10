-- +goose Up

CREATE TABLE teams (
    id          TEXT PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE team_members (
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    TEXT NOT NULL DEFAULT 'member',
    PRIMARY KEY (team_id, user_id)
);

ALTER TABLE services ADD COLUMN team_id TEXT REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE escalation_policies ADD COLUMN team_id TEXT REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE schedules ADD COLUMN team_id TEXT REFERENCES teams(id) ON DELETE SET NULL;

-- +goose Down

-- SQLite does not support DROP COLUMN, so we recreate without team_id
-- For development, dropping and re-migrating is simpler
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
