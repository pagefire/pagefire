<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go 1.22+">
  <img src="https://img.shields.io/badge/SQLite-embedded-003B57?style=flat&logo=sqlite&logoColor=white" alt="SQLite">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT License">
  <img src="https://img.shields.io/badge/status-pre--release-orange?style=flat" alt="Pre-release">
</p>

# PageFire

Open-source incident management platform. On-call scheduling, alert escalation, and notifications in a single Go binary.

**Coming from Grafana OnCall?** See the [Migration Guide](docs/migrating-from-grafana-oncall.md).

## Why PageFire?

| | PageFire | PagerDuty | Grafana OnCall | OneUptime |
|---|---|---|---|---|
| **Deployment** | Single binary | SaaS only | Archived (March 2026) | 10+ Docker containers |
| **Database** | Embedded SQLite | N/A | PostgreSQL + Redis + RabbitMQ | PostgreSQL + Redis + ClickHouse |
| **RAM required** | ~50 MB | N/A | ~2 GB | ~8 GB |
| **License** | MIT | Proprietary | Apache 2.0 (archived) | MIT |
| **Self-hosted** | Yes | No | No longer maintained | Yes (complex) |
| **Pricing** | Free (self-host) | $21/user/mo+ | Free (unsupported) | $120/mo+ |

## Features

- **Alert management** — create, acknowledge, resolve, with full audit log
- **Alert deduplication** — `dedup_key` prevents duplicate alerts for the same issue
- **Alert grouping** — `group_key` groups related alerts; only the first triggers escalation
- **Content-based routing** — route alerts to different escalation policies based on summary, details, or source (contains or regex match)
- **Escalation policies** — multi-step escalation with configurable delays and repeat loops
- **On-call schedules** — daily/weekly/custom rotations with participant ordering
- **Schedule overrides** — temporary user swaps for PTO, handoffs, etc.
- **Teams** — organize users, services, policies, and schedules by team
- **Incidents** — track multi-service outages with timeline updates
- **Notifications** — email (SMTP), Slack DM, and webhook dispatch
- **Inbound integrations** — generic webhook, Grafana, and Prometheus Alertmanager

## Status

**Pre-release / active development.** Not yet ready for production use.

## Quick Start

### 1. Build

```bash
make build
```

Or download a [pre-built binary](https://github.com/pagefire/pagefire/releases) for your platform. Requires Go 1.22+ to build from source.

### 2. Pick an admin token

PageFire uses a single shared API token for authentication. You choose it — it can be any string. Every API request must include this token.

```bash
export PAGEFIRE_ADMIN_TOKEN="change-me-to-something-secret"
```

### 3. Start the server

```bash
./bin/pagefire serve
# Or: make dev
```

PageFire starts on port 3000 and creates a SQLite database at `./pagefire.db`.

### 4. Make your first API call

```bash
# Health check (no auth required)
curl http://localhost:3000/healthz

# Create a user (auth required)
curl -X POST http://localhost:3000/api/v1/users \
  -H "Authorization: Bearer $PAGEFIRE_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "email": "alice@example.com"}'
```

All API endpoints require the header `Authorization: Bearer <your-token>` except the health check and inbound integration webhooks.

### 5. Run tests

```bash
make test
```

## Configuration

All configuration is via environment variables with the `PAGEFIRE_` prefix.

### Required

| Variable | Description |
|----------|-------------|
| `PAGEFIRE_ADMIN_TOKEN` | API token for authentication. You choose this value. All API requests must include `Authorization: Bearer <this-value>`. |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_DATABASE_DRIVER` | `sqlite` | Database driver (`sqlite`; Postgres planned) |
| `PAGEFIRE_DATABASE_URL` | `./pagefire.db` | Database connection string |
| `PAGEFIRE_DATA_DIR` | `.` | Data directory for SQLite |
| `PAGEFIRE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PAGEFIRE_ENGINE_INTERVAL_SECONDS` | `5` | How often the engine processes alerts |
| `PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS` | `false` | Allow outbound webhooks to private/localhost IPs (useful for local dev) |

### Email notifications (SMTP)

| Variable | Default | Description |
|----------|---------|-------------|
| `PAGEFIRE_SMTP_HOST` | — | SMTP server hostname |
| `PAGEFIRE_SMTP_PORT` | `587` | SMTP port |
| `PAGEFIRE_SMTP_FROM` | — | Sender email address |
| `PAGEFIRE_SMTP_USERNAME` | — | SMTP auth username |
| `PAGEFIRE_SMTP_PASSWORD` | — | SMTP auth password |

### Slack notifications

| Variable | Default | Description |
|----------|---------|-------------|
| `PAGEFIRE_SLACK_BOT_TOKEN` | — | Slack bot token for DM notifications |

## API

All endpoints require `Authorization: Bearer <your-token>` except health check and integration webhooks.

All list endpoints support `?limit=` and `?offset=` query parameters for pagination (max 1000 results).

```
GET  /healthz                                    # Health check (no auth)

# Inbound integrations (authenticated by integration key, not admin token)
POST /api/v1/integrations/{key}/alerts           # Generic webhook
POST /api/v1/integrations/{key}/grafana          # Grafana webhook
POST /api/v1/integrations/{key}/prometheus       # Prometheus Alertmanager webhook

# Teams
GET/POST       /api/v1/teams
GET/PUT/DELETE /api/v1/teams/{id}
GET/POST       /api/v1/teams/{id}/members
DELETE         /api/v1/teams/{id}/members/{userID}

# Users
GET/POST            /api/v1/users
GET/PUT/DELETE      /api/v1/users/{id}
GET/POST            /api/v1/users/{id}/contact-methods
DELETE              /api/v1/users/{id}/contact-methods/{cmID}
GET/POST            /api/v1/users/{id}/notification-rules
DELETE              /api/v1/users/{id}/notification-rules/{ruleID}

# Services (supports ?team_id= filter)
GET/POST            /api/v1/services
GET/PUT/DELETE      /api/v1/services/{id}
GET/POST            /api/v1/services/{id}/integration-keys
DELETE              /api/v1/services/{id}/integration-keys/{keyID}
GET/POST            /api/v1/services/{id}/routing-rules
DELETE              /api/v1/services/{id}/routing-rules/{ruleID}

# Escalation Policies (supports ?team_id= filter)
GET/POST            /api/v1/escalation-policies
GET/PUT/DELETE      /api/v1/escalation-policies/{id}
GET/POST            /api/v1/escalation-policies/{id}/steps
DELETE              /api/v1/escalation-policies/{id}/steps/{stepID}
GET/POST            /api/v1/escalation-policies/{id}/steps/{stepID}/targets
DELETE              /api/v1/escalation-policies/{id}/steps/{stepID}/targets/{targetID}

# Schedules (supports ?team_id= filter)
GET/POST            /api/v1/schedules
GET/PUT/DELETE      /api/v1/schedules/{id}
GET/POST            /api/v1/schedules/{id}/rotations
DELETE              /api/v1/schedules/{id}/rotations/{rotID}
GET/POST            /api/v1/schedules/{id}/rotations/{rotID}/participants
DELETE              /api/v1/schedules/{id}/rotations/{rotID}/participants/{partID}
GET/POST            /api/v1/schedules/{id}/overrides
DELETE              /api/v1/schedules/{id}/overrides/{overrideID}

# Alerts (supports ?status=, ?service_id=, ?group_key= filters)
GET/POST            /api/v1/alerts
GET                 /api/v1/alerts/{id}
POST                /api/v1/alerts/{id}/acknowledge
POST                /api/v1/alerts/{id}/resolve
GET                 /api/v1/alerts/{id}/logs

# Incidents (supports ?status= filter)
GET/POST            /api/v1/incidents
GET/PUT             /api/v1/incidents/{id}
GET/POST            /api/v1/incidents/{id}/updates

# On-Call
GET                 /api/v1/oncall/{scheduleID}
```

### Alert deduplication

Send a `dedup_key` with your alert. If an active (non-resolved) alert already exists with the same `dedup_key` on the same service, the existing alert is returned instead of creating a duplicate.

### Alert grouping

Send a `group_key` with your alert. When multiple alerts share the same `group_key` on a service, only the first alert triggers escalation. Subsequent alerts in the group are created but their escalation is suppressed (no duplicate notifications). Once all alerts in the group are resolved, a new alert with the same `group_key` will escalate normally.

### Content-based routing

Create routing rules on a service to direct alerts to different escalation policies based on content. Rules are evaluated in priority order; the first match wins. If no rule matches, the service's default escalation policy is used.

Supported match types: `contains` (case-insensitive) and `regex`.

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

Two demo scripts exercise PageFire end-to-end.

### Single-user demo (`demo/demo.sh`)

Starts PageFire, a fake app, and a notification receiver. A health checker polls the app every 5 seconds. Kill the app to trigger an alert; restart it to auto-resolve.

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
- API token auth with constant-time comparison
- Per-IP rate limiting (60/min integration, 1000/min API)
- SSRF protection on all outbound webhooks (private IP blocking + DNS resolution)
- Email header injection prevention
- Integration key secrets masked in API responses (full secret shown once on creation)
- Request body limits (1MB), query bounds (max 1000)
- Security headers on all responses (CSP, X-Frame-Options, X-Content-Type-Options)
- Regex pattern validation at routing rule creation time
- All SQL queries parameterized (no string concatenation)

## Contributing

Contributions are welcome! Please open an issue first to discuss what you'd like to change.

## License

[MIT](LICENSE)
