# PageFire

Open-source incident management platform. On-call scheduling, alert escalation, and notifications in a single Go binary.

## Why PageFire?

- **Single binary** — no Docker compose, no 8GB RAM minimum, no microservices. Just download and run.
- **SQLite by default** — zero-dependency setup. Postgres available for HA deployments.
- **Opinionated simplicity** — on-call + alerts + escalation. No APM, no logs, no traces.

## Status

**Pre-release / active development.** Not yet ready for production use.

Current: on-call engine, alert escalation, notification dispatch (email, webhook, Slack).

## Quick Start

```bash
# Build
make build

# Run (SQLite, port 3000)
PAGEFIRE_ADMIN_TOKEN=your-secret-token ./bin/pagefire serve

# Or use go run
PAGEFIRE_ADMIN_TOKEN=your-secret-token make dev
```

## Configuration

All configuration via environment variables with `PAGEFIRE_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_ADMIN_TOKEN` | *(required)* | Bearer token for API authentication |
| `PAGEFIRE_DATABASE_DRIVER` | `sqlite` | `sqlite` or `postgres` |
| `PAGEFIRE_DATABASE_URL` | `./pagefire.db` | Database connection string |
| `PAGEFIRE_DATA_DIR` | `.` | Data directory for SQLite |
| `PAGEFIRE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PAGEFIRE_SMTP_HOST` | — | SMTP server for email notifications |
| `PAGEFIRE_SMTP_PORT` | `587` | SMTP port |
| `PAGEFIRE_SMTP_FROM` | — | Sender email address |
| `PAGEFIRE_SMTP_USERNAME` | — | SMTP auth username |
| `PAGEFIRE_SMTP_PASSWORD` | — | SMTP auth password |
| `PAGEFIRE_SLACK_BOT_TOKEN` | — | Slack bot token for DM notifications |

## API

All endpoints require `Authorization: Bearer <PAGEFIRE_ADMIN_TOKEN>` except health check and integration webhooks.

```
GET  /healthz                                    # Health check (no auth)

POST /api/v1/integrations/{key}/alerts           # Generic webhook (auth by key)
POST /api/v1/integrations/{key}/grafana          # Grafana webhook
POST /api/v1/integrations/{key}/prometheus        # Prometheus Alertmanager webhook

GET/POST       /api/v1/users
GET/PUT        /api/v1/users/{id}
POST           /api/v1/users/{id}/contact-methods
POST           /api/v1/users/{id}/notification-rules

GET/POST       /api/v1/services
GET/PUT/DELETE /api/v1/services/{id}
GET/POST       /api/v1/services/{id}/integration-keys

GET/POST       /api/v1/escalation-policies
GET/PUT/DELETE /api/v1/escalation-policies/{id}
GET/POST       /api/v1/escalation-policies/{id}/steps
GET/POST       /api/v1/escalation-policies/{id}/steps/{stepID}/targets

GET/POST       /api/v1/schedules
GET/PUT/DELETE /api/v1/schedules/{id}
GET/POST       /api/v1/schedules/{id}/rotations
GET/POST       /api/v1/schedules/{id}/overrides

GET/POST       /api/v1/alerts
GET            /api/v1/alerts/{id}
POST           /api/v1/alerts/{id}/acknowledge
POST           /api/v1/alerts/{id}/resolve

GET/POST       /api/v1/incidents
GET/PUT        /api/v1/incidents/{id}
GET/POST       /api/v1/incidents/{id}/updates

GET            /api/v1/oncall/{scheduleID}
```

## Project Structure

```
cmd/pagefire/          CLI entrypoint (cobra)
internal/
  app/                 Config, dependency wiring, lifecycle
  api/                 HTTP handlers, auth, rate limiting
  store/               Domain models, repository interfaces
    sqlite/            SQLite implementation
    migrations/        Goose migrations (sqlite + postgres)
  engine/              Background processors (escalation, notifications, cleanup)
  oncall/              On-call schedule resolver
  notification/        Dispatcher + providers (email, webhook, slack)
```

## Security

See [SECURITY_CHECKLIST.md](SECURITY_CHECKLIST.md) for the pre-commit security checklist.

Hardened against STRIDE threat model findings:
- Bearer token auth with constant-time comparison
- Per-IP rate limiting (60/min integration, 1000/min API)
- SSRF protection on all outbound webhooks
- Email header injection prevention
- Integration key secrets masked in API responses
- Request body limits (1MB), query bounds (max 1000)
- Security headers on all responses

## License

[MIT](LICENSE)
