-- +goose Up

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT UNIQUE NOT NULL,
    role       TEXT NOT NULL DEFAULT 'user',
    timezone   TEXT NOT NULL DEFAULT 'UTC',
    avatar_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE contact_methods (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    value      TEXT NOT NULL,
    verified   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, type, value)
);

CREATE TABLE notification_rules (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_method_id TEXT NOT NULL REFERENCES contact_methods(id) ON DELETE CASCADE,
    delay_minutes     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE incidents (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'triggered',
    severity    TEXT NOT NULL DEFAULT 'critical',
    summary     TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'manual',
    created_by  TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE incident_services (
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    service_id  TEXT NOT NULL,
    PRIMARY KEY (incident_id, service_id)
);

CREATE TABLE incident_updates (
    id          TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,
    message     TEXT NOT NULL,
    created_by  TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_queue (
    id                TEXT PRIMARY KEY,
    alert_id          TEXT,
    user_id           TEXT,
    contact_method_id TEXT,
    type              TEXT NOT NULL DEFAULT 'alert',
    destination_type  TEXT NOT NULL,
    destination       TEXT NOT NULL,
    subject           TEXT NOT NULL DEFAULT '',
    body              TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'pending',
    attempts          INTEGER NOT NULL DEFAULT 0,
    max_attempts      INTEGER NOT NULL DEFAULT 3,
    next_attempt_at   TIMESTAMPTZ,
    sent_at           TIMESTAMPTZ,
    provider_id       TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_queue_pending ON notification_queue(status, next_attempt_at);

-- +goose Down

DROP TABLE IF EXISTS notification_queue;
DROP TABLE IF EXISTS incident_updates;
DROP TABLE IF EXISTS incident_services;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS notification_rules;
DROP TABLE IF EXISTS contact_methods;
DROP TABLE IF EXISTS users;
