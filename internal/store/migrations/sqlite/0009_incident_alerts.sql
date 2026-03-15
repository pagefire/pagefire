-- +goose Up
CREATE TABLE incident_alerts (
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id    TEXT NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    PRIMARY KEY (incident_id, alert_id)
);

CREATE INDEX idx_incident_alerts_alert ON incident_alerts(alert_id);

-- +goose Down
DROP TABLE IF EXISTS incident_alerts;
