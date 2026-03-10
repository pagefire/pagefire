-- +goose Up

CREATE TABLE services (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    escalation_policy_id TEXT NOT NULL,
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE integration_keys (
    id         TEXT PRIMARY KEY,
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL DEFAULT 'generic',
    secret     TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE escalation_policies (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    repeat      INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE escalation_steps (
    id                   TEXT PRIMARY KEY,
    escalation_policy_id TEXT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
    step_number          INTEGER NOT NULL,
    delay_minutes        INTEGER NOT NULL DEFAULT 5,
    UNIQUE(escalation_policy_id, step_number)
);

CREATE TABLE escalation_step_targets (
    id                 TEXT PRIMARY KEY,
    escalation_step_id TEXT NOT NULL REFERENCES escalation_steps(id) ON DELETE CASCADE,
    target_type        TEXT NOT NULL,
    target_id          TEXT NOT NULL
);

CREATE TABLE schedules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rotations (
    id           TEXT PRIMARY KEY,
    schedule_id  TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT 'weekly',
    shift_length INTEGER NOT NULL DEFAULT 1,
    start_time   TIMESTAMP NOT NULL,
    handoff_time TEXT NOT NULL DEFAULT '09:00',
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rotation_participants (
    id          TEXT PRIMARY KEY,
    rotation_id TEXT NOT NULL REFERENCES rotations(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    UNIQUE(rotation_id, position)
);

CREATE TABLE schedule_overrides (
    id            TEXT PRIMARY KEY,
    schedule_id   TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    start_time    TIMESTAMP NOT NULL,
    end_time      TIMESTAMP NOT NULL,
    replace_user  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    override_user TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE alerts (
    id                         TEXT PRIMARY KEY,
    service_id                 TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    status                     TEXT NOT NULL DEFAULT 'triggered',
    summary                    TEXT NOT NULL,
    details                    TEXT NOT NULL DEFAULT '',
    source                     TEXT NOT NULL DEFAULT 'api',
    dedup_key                  TEXT NOT NULL DEFAULT '',
    escalation_policy_snapshot TEXT NOT NULL DEFAULT '{}',
    escalation_step            INTEGER NOT NULL DEFAULT 0,
    loop_count                 INTEGER NOT NULL DEFAULT 0,
    next_escalation_at         TIMESTAMP,
    acknowledged_by            TEXT REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at            TIMESTAMP,
    resolved_at                TIMESTAMP,
    created_at                 TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_alerts_service_dedup ON alerts(service_id, dedup_key);
CREATE INDEX idx_alerts_pending_escalation ON alerts(status, next_escalation_at);

CREATE TABLE alert_logs (
    id         TEXT PRIMARY KEY,
    alert_id   TEXT NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    event      TEXT NOT NULL,
    message    TEXT NOT NULL DEFAULT '',
    user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add FK from services to escalation_policies (deferred to avoid circular dependency with CREATE order)
-- SQLite doesn't support ALTER TABLE ADD CONSTRAINT, so the FK is enforced at the application level.

-- +goose Down

DROP TABLE IF EXISTS alert_logs;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS schedule_overrides;
DROP TABLE IF EXISTS rotation_participants;
DROP TABLE IF EXISTS rotations;
DROP TABLE IF EXISTS schedules;
DROP TABLE IF EXISTS escalation_step_targets;
DROP TABLE IF EXISTS escalation_steps;
DROP TABLE IF EXISTS escalation_policies;
DROP TABLE IF EXISTS integration_keys;
DROP TABLE IF EXISTS services;
