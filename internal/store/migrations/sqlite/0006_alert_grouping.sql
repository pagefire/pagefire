-- +goose Up
ALTER TABLE alerts ADD COLUMN group_key TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_alerts_group_key ON alerts(service_id, group_key)
    WHERE group_key != '' AND status != 'resolved';

-- +goose Down
DROP INDEX IF EXISTS idx_alerts_group_key;
-- SQLite does not support DROP COLUMN; this is acceptable for development.
