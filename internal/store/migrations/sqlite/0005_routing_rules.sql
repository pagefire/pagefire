-- +goose Up
CREATE TABLE routing_rules (
    id                   TEXT PRIMARY KEY,
    service_id           TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    priority             INTEGER NOT NULL DEFAULT 0,
    condition_field      TEXT NOT NULL,  -- "summary", "details", "source"
    condition_match_type TEXT NOT NULL,  -- "contains", "regex"
    condition_value      TEXT NOT NULL,
    escalation_policy_id TEXT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_routing_rules_service ON routing_rules(service_id, priority);

-- +goose Down
DROP INDEX IF EXISTS idx_routing_rules_service;
DROP TABLE IF EXISTS routing_rules;
