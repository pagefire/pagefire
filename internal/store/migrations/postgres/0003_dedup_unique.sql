-- +goose Up

-- Enforce dedup uniqueness at the DB level for non-empty dedup keys on active alerts.
-- This prevents race conditions where two concurrent requests with the same dedup_key
-- bypass the application-level check-then-insert and create duplicate alerts.
CREATE UNIQUE INDEX idx_alerts_active_dedup
    ON alerts(service_id, dedup_key)
    WHERE dedup_key != '' AND status != 'resolved';

-- +goose Down

DROP INDEX IF EXISTS idx_alerts_active_dedup;
