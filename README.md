<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go 1.22+">
  <img src="https://img.shields.io/badge/SQLite-embedded-003B57?style=flat&logo=sqlite&logoColor=white" alt="SQLite">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT License">
  <img src="https://img.shields.io/badge/status-pre--release-orange?style=flat" alt="Pre-release">
</p>

# PageFire

Open-source incident management platform. On-call scheduling, alert escalation, and notifications — with a built-in web UI — in a single Go binary.

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

- **Web UI** — built-in dashboard for managing alerts, services, schedules, teams, and users
- **Alert management** — create, acknowledge, resolve, with full audit log
- **Alert deduplication** — `dedup_key` prevents duplicate alerts for the same issue
- **Alert grouping** — `group_key` groups related alerts; only the first triggers escalation
- **Content-based routing** — route alerts to different escalation policies based on summary, details, or source (contains or regex match)
- **Escalation policies** — multi-step escalation with configurable delays and repeat loops
- **On-call schedules** — daily/weekly/custom rotations with participant ordering
- **Schedule overrides** — temporary user swaps for PTO, handoffs — available to all users, not just admins
- **Teams** — organize users and manage team membership with roles
- **Incidents** — track multi-service outages with timeline updates
- **Notifications** — email (SMTP), Slack DM, and webhook dispatch with per-user contact methods and notification rules
- **Inbound integrations** — generic webhook, Grafana, and Prometheus Alertmanager
- **Authentication** — email/password login, session-based auth, per-user API tokens, user invite flow
- **RBAC** — admin and member roles with permission enforcement across UI and API

## Status

**Pre-release / active development.** Not yet ready for production use.

## Quick Start

### 1. Build

```bash
make build
```

Or download a [pre-built binary](https://github.com/pagefire/pagefire/releases) for your platform. Requires Go 1.22+ to build from source.

### 2. Start the server

```bash
./bin/pagefire serve
# Or: make dev
```

PageFire starts on port 3000 and creates a SQLite database at `./pagefire.db`.

### 3. Set up your account

Open **http://localhost:3000** in your browser. On first run, you'll see a setup wizard to create your admin account (name, email, password).

After setup, you'll be logged in and can:
- Create services and escalation policies
- Set up on-call schedules with rotations
- Invite team members (they'll receive an invite link to set their own password)
- Configure your contact methods and notification rules in **Profile & Settings**

### 4. Connect your monitoring

Create a service, then generate an integration key. Use the key to send alerts from your monitoring tools:

```bash
# Fire an alert via integration webhook (no auth required — the key IS the auth)
curl -X POST http://localhost:3000/api/v1/integrations/<your-key>/alerts \
  -H "Content-Type: application/json" \
  -d '{"summary": "High CPU on web-01", "dedup_key": "cpu-web-01"}'
```

### 5. API access

For programmatic access, generate a per-user API token from **Profile & Settings > API Tokens** in the web UI, then use it in your scripts:

```bash
curl http://localhost:3000/api/v1/alerts \
  -H "Authorization: Bearer pf_your-api-token"
```

### 6. Run tests

```bash
make test
```

## Authentication

PageFire supports three authentication methods:

| Method | Use Case |
|--------|----------|
| **Session cookie** | Web UI — set automatically on login |
| **Per-user API token** | Scripts and integrations — generated in Profile & Settings, prefixed with `pf_` |

### User invite flow

Admins create users with name, email, and role. The system generates a one-time invite link. The new user opens the link and sets their own password. Admins never see or set other users' passwords.

### Roles

| Role | Permissions |
|------|-------------|
| **Admin** | Full access: create/edit/delete users, services, escalation policies, schedules, teams |
| **Member** | Read access to all resources. Can acknowledge/resolve alerts, create schedule overrides, manage own profile settings |

## Configuration

All configuration is via environment variables with the `PAGEFIRE_` prefix.

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

All endpoints require authentication (session cookie or API token) except health check and inbound integration webhooks.

All list endpoints support `?limit=` and `?offset=` query parameters for pagination (max 1000 results).

```
GET  /healthz                                    # Health check (no auth)

# Authentication
POST /api/v1/auth/login                          # Login (email + password)
POST /api/v1/auth/logout                         # Logout (clears session)
GET  /api/v1/auth/me                             # Current user info
GET  /api/v1/auth/setup                          # Check if setup is needed
POST /api/v1/auth/setup                          # Create first admin user
PUT  /api/v1/auth/password                       # Change own password
POST /api/v1/auth/tokens                         # Generate API token
GET  /api/v1/auth/tokens                         # List API tokens
DELETE /api/v1/auth/tokens/{id}                  # Revoke API token
GET  /api/v1/auth/invite/{token}                 # Validate invite token
POST /api/v1/auth/invite/{token}                 # Accept invite (set password)

# Inbound integrations (authenticated by integration key, not user auth)
POST /api/v1/integrations/{key}/alerts           # Generic webhook
POST /api/v1/integrations/{key}/grafana          # Grafana webhook
POST /api/v1/integrations/{key}/prometheus       # Prometheus Alertmanager webhook

# Teams
GET/POST       /api/v1/teams
GET/PUT/DELETE /api/v1/teams/{id}
GET/POST       /api/v1/teams/{id}/members
DELETE         /api/v1/teams/{id}/members/{userID}

# Users (admin only for create/update/delete)
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

# Schedule Overrides (accessible to all authenticated users)
GET/POST            /api/v1/schedule-overrides/{id}/overrides
DELETE              /api/v1/schedule-overrides/{id}/overrides/{overrideID}

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
  api/                 HTTP handlers, auth middleware, rate limiting
  auth/                Password hashing, session management, API tokens
  store/               Domain models, repository interfaces
    sqlite/            SQLite implementation
    migrations/        Goose migrations (sqlite + postgres)
  engine/              Background processors (escalation, notifications, cleanup)
  oncall/              On-call schedule resolver
  notification/        Dispatcher + providers (email, webhook, slack)
web/                   Preact + Vite frontend (embedded in binary)
```

## Demo

Two demo scripts exercise PageFire end-to-end. These scripts use the setup endpoint and per-user API tokens for API automation.

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
- Session-based auth with HttpOnly cookies, argon2id password hashing
- Per-user API tokens with secure generation and hash-only storage
- RBAC enforcement on all write operations (admin vs member roles)
- Per-IP rate limiting (60/min integration, 1000/min API, 10/min login)
- SSRF protection on all outbound webhooks (private IP blocking + DNS resolution)
- Email header injection prevention
- Integration key secrets masked in API responses (full secret shown once on creation)
- Request body limits (1MB), query bounds (max 1000)
- Security headers on all responses (CSP, X-Frame-Options, X-Content-Type-Options)
- Regex pattern validation at routing rule creation time
- All SQL queries parameterized (no string concatenation)
- One-time invite tokens with SHA-256 hashing and 7-day expiry

## Contributing

Contributions are welcome! Please open an issue first to discuss what you'd like to change.

## License

[MIT](LICENSE)
