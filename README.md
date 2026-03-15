# PageFire

**Open-source on-call management and incident response. Single binary, self-hosted.**

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go 1.22+">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT License">
  <img src="https://img.shields.io/github/v/release/pagefire/pagefire?style=flat&color=blue" alt="Release">
</p>

PageFire replaces PagerDuty, Grafana OnCall, and OneUptime with a single Go binary and an embedded SQLite database. No containers, no message queues, no 8 GB RAM footprint. Deploy it in seconds, own your data.

**Coming from Grafana OnCall?** It's being [archived on March 24, 2026](https://grafana.com/blog/2025/01/27/grafana-oncall-oss/). See the [Migration Guide](docs/migrating-from-grafana-oncall.md).

## Features

- **On-call scheduling** — daily, weekly, and custom rotations with participant ordering
- **Schedule overrides** — temporary user swaps for PTO or handoffs
- **Escalation policies** — multi-step escalation with configurable delays and repeat loops
- **Alert routing** — content-based routing rules (contains, regex) to direct alerts to different policies
- **Alert deduplication & grouping** — `dedup_key` prevents duplicates; `group_key` suppresses redundant escalations
- **Incident management** — track multi-service outages with timeline updates
- **Multi-channel notifications** — email (SMTP), SMS, phone calls (Twilio), Slack DM, webhooks
- **Inbound integrations** — Grafana, Prometheus Alertmanager, generic webhook
- **Per-user API tokens** — generated in the UI, prefixed `pf_`, hash-only storage
- **User invite flow** — admins invite by email, users set their own passwords
- **RBAC** — admin and member roles enforced across UI and API
- **Built-in web UI** — manage everything from the browser, no separate frontend to deploy

## Quick Start

### Docker

```bash
docker compose up -d
```

See [docs/docker.md](docs/docker.md) for volumes, environment variables, and production configuration.

### Binary

Download a [pre-built binary](https://github.com/pagefire/pagefire/releases) for your platform, then:

```bash
export PAGEFIRE_PORT=3000
./pagefire serve
```

Or build from source (requires Go 1.22+):

```bash
make build
./bin/pagefire serve
```

### First Run

Open **http://localhost:3000**. The setup wizard creates your admin account. From there:

1. Create a service and escalation policy
2. Generate an integration key on the service
3. Point your monitoring at the integration endpoint
4. Invite your team

```bash
# Fire a test alert
curl -X POST http://localhost:3000/api/v1/integrations/<key>/alerts \
  -H "Content-Type: application/json" \
  -d '{"summary": "High CPU on web-01", "dedup_key": "cpu-web-01"}'
```

## Screenshots

<!-- TODO: Add dashboard screenshot -->
<!-- TODO: Add service detail screenshot -->
<!-- TODO: Add on-call schedule screenshot -->
<!-- TODO: Add alert detail screenshot -->
<!-- TODO: Add incident timeline screenshot -->

## Why PageFire?

| | PageFire | PagerDuty | Grafana OnCall | OneUptime |
|---|---|---|---|---|
| **Deployment** | Single binary | SaaS only | Archived (March 2026) | 10+ Docker containers |
| **Database** | Embedded SQLite | N/A | PostgreSQL + Redis + RabbitMQ | PostgreSQL + Redis + ClickHouse |
| **RAM** | ~50 MB | N/A | ~2 GB | ~8 GB |
| **License** | MIT | Proprietary | Apache 2.0 (archived) | MIT |
| **Self-hosted** | Yes | No | No longer maintained | Yes (complex) |
| **Price** | Free (self-host) | $21/user/mo+ | Free (unsupported) | $120/mo+ |

## Configuration

All configuration is via environment variables with the `PAGEFIRE_` prefix.

| Variable | Default | Description |
|----------|---------|-------------|
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_DATABASE_DRIVER` | `sqlite` | Database driver (`sqlite`; Postgres planned) |
| `PAGEFIRE_DATABASE_URL` | `./pagefire.db` | Database connection string |
| `PAGEFIRE_DATA_DIR` | `.` | Data directory for SQLite |
| `PAGEFIRE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PAGEFIRE_ENGINE_INTERVAL_SECONDS` | `5` | How often the engine processes alerts |
| `PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS` | `false` | Allow outbound webhooks to private/localhost IPs |

## Notification Channels

| Channel | Provider | Setup |
|---------|----------|-------|
| Email | SMTP | Set `PAGEFIRE_SMTP_HOST`, `PAGEFIRE_SMTP_PORT`, `PAGEFIRE_SMTP_FROM`, `PAGEFIRE_SMTP_USERNAME`, `PAGEFIRE_SMTP_PASSWORD` |
| SMS | Twilio | [docs/twilio-setup.md](docs/twilio-setup.md) |
| Phone call | Twilio | [docs/twilio-setup.md](docs/twilio-setup.md) |
| Slack DM | Slack Bot | Set `PAGEFIRE_SLACK_BOT_TOKEN` |
| Webhook | Built-in | Add a webhook contact method in the UI |

## Integrations

PageFire accepts alerts from any monitoring tool that can send HTTP webhooks.

| Source | Endpoint | Notes |
|--------|----------|-------|
| **Grafana** | `POST /api/v1/integrations/{key}/grafana` | Native Grafana alerting format |
| **Prometheus Alertmanager** | `POST /api/v1/integrations/{key}/prometheus` | Configure as a webhook receiver |
| **Generic webhook** | `POST /api/v1/integrations/{key}/alerts` | JSON body with `summary`, optional `dedup_key`, `group_key` |

No authentication header required — the integration key in the URL is the credential.

## Documentation

| Guide | Description |
|-------|-------------|
| [Docker deployment](docs/docker.md) | Docker Compose setup, volumes, production config |
| [Twilio setup](docs/twilio-setup.md) | SMS and phone call notifications via Twilio |
| [TLS & reverse proxy](docs/tls-reverse-proxy.md) | HTTPS with Nginx, Caddy, or Traefik |
| [Migrating from Grafana OnCall](docs/migrating-from-grafana-oncall.md) | Concept mapping and step-by-step migration |
| [OpenAPI spec](docs/openapi.yaml) | Full API specification |

## API

PageFire exposes a REST API. All endpoints require authentication (session cookie or `pf_`-prefixed API token) except `/healthz` and inbound integration webhooks.

Generate an API token from **Profile & Settings > API Tokens** in the web UI:

```bash
curl http://localhost:3000/api/v1/alerts \
  -H "Authorization: Bearer pf_your-api-token"
```

Full API specification: [docs/openapi.yaml](docs/openapi.yaml)

## Security

- Argon2id password hashing, session-based auth with HttpOnly cookies
- Per-user API tokens with hash-only storage
- RBAC enforcement on all write operations
- Per-IP rate limiting (60/min integration, 1000/min API, 10/min login)
- SSRF protection on outbound webhooks (private IP blocking + DNS resolution)
- Security headers (CSP, X-Frame-Options, X-Content-Type-Options)
- All SQL queries parameterized
- One-time invite tokens with SHA-256 hashing and 7-day expiry

## Contributing

Contributions are welcome. Please open an issue first to discuss what you'd like to change.

## License

[MIT](LICENSE)
