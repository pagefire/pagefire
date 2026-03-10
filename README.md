# PageFire

Open-source incident management platform. On-call scheduling, alert escalation, and notifications in a single Go binary.

**Coming from Grafana OnCall?** See the [Migration Guide](docs/migrating-from-grafana-oncall.md).

## Why PageFire?

- **Single binary** — no Docker compose, no 8GB RAM minimum, no microservices. Just download and run.
- **SQLite by default** — zero-dependency setup. Postgres support planned.
- **Opinionated simplicity** — on-call + alerts + escalation. No APM, no logs, no traces.

## Status

**Pre-release / active development.** Not yet ready for production use.

Current: on-call engine, alert escalation, notification dispatch (email, webhook, Slack).

## Quick Start

Requires Go 1.22+.

```bash
# Build
make build

# Run (SQLite, port 3000)
PAGEFIRE_ADMIN_TOKEN=your-secret-token ./bin/pagefire serve

# Or use go run
PAGEFIRE_ADMIN_TOKEN=your-secret-token make dev

# Run tests
make test
```

## Configuration

All configuration via environment variables with `PAGEFIRE_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_ADMIN_TOKEN` | *(required)* | Bearer token for API authentication |
| `PAGEFIRE_DATABASE_DRIVER` | `sqlite` | `sqlite` (postgres planned) |
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

## Demo

Two demo scripts are included to exercise PageFire end-to-end.

### Single-user demo (`demo/demo.sh`)

Starts PageFire, a fake app, and a notification receiver. A health checker polls the app every 5 seconds. Kill the app to trigger an alert and get paged; restart it to auto-resolve.

```bash
./demo/demo.sh
```

What it sets up:
- PageFire on `:3001`, fake app on `:8080`, notification receiver on `:9090`
- One user, one webhook contact method, one escalation policy, one service

Once running, try:
1. Kill the app: `kill $(lsof -ti:8080)`
2. Watch the alert fire and notification print to the terminal
3. Restart the app: `go run demo/myapp.go &`
4. Watch the alert auto-resolve

### Multi-user demo (`demo/demo-org.sh`)

Demonstrates a full team setup with three users, an on-call schedule with rotations, schedule overrides, and escalation through multiple steps.

```bash
./demo/demo-org.sh
```

What it sets up:
- Three users (AAA, BBB, CCC) with webhook contact methods
- An on-call schedule with a daily rotation
- An escalation policy with two steps (on-call schedule, then all users)
- A schedule override swapping the on-call user
- A health checker that fires alerts and verifies notifications route correctly

Both scripts clean up all processes and temp files on exit (Ctrl+C).

## Security

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
