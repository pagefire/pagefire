# PageFire: Open-source on-call for teams leaving Grafana OnCall

*March 2026*

Grafana is [archiving OnCall OSS](https://grafana.com/blog/2025/01/27/grafana-oncall-oss/) on March 24, 2026. Cloud Connection -- the free SMS, phone, and mobile push that made OnCall usable -- dies the same day. Thousands of self-hosted teams need to migrate somewhere. PageFire is an open-source alternative built specifically for this moment.

## What is PageFire?

PageFire is open-source on-call management and incident response packaged as a single Go binary. It handles alert routing, escalation policies, on-call schedules with rotations and overrides, multi-channel notifications (email, SMS, phone, Slack, webhooks), and incident tracking -- all backed by an embedded SQLite database. No Postgres, no Redis, no RabbitMQ, no container orchestration. Download a binary, run `pagefire serve`, and you have a working on-call system in under a minute. MIT licensed.

## Why we built it

The on-call tooling market has a hole in the middle.

On one end, you have PagerDuty and OpsGenie at $21-49/user/month. They work, but the pricing is punishing for small teams and startups. A 10-person on-call rotation costs $2,500-6,000/year for what is fundamentally "receive alert, call someone, escalate if they don't respond."

On the other end, the self-hosted options all have serious problems. Grafana OnCall was the best of them -- good UX, decent feature set, Apache 2.0 license. But it required PostgreSQL, Redis, RabbitMQ, and Celery workers. And now it's dead, with Grafana pushing users toward their paid Cloud IRM product. OneUptime tries to replace the entire observability stack (monitoring, status pages, on-call, logs, traces) but needs 10+ Docker containers and 8GB of RAM minimum. It's trying to be Datadog, and the complexity shows. GoAlert from Target is clean and well-built, but it's on-call only -- no monitoring, no status pages, no incident management.

We wanted something that doesn't exist yet: the scope of OneUptime in GoAlert's footprint. A single binary that handles on-call scheduling, alert routing, escalation, notifications, and incident management -- without requiring a PhD in Docker Compose to deploy it. PageFire uses ~50MB of RAM and stores everything in a single SQLite file. You can back up your entire on-call system by copying one file.

## Key features

- **On-call scheduling** -- daily, weekly, and custom rotations with participant ordering and timezone support
- **Schedule overrides** -- any user can swap shifts for PTO or handoffs, no admin required
- **Escalation policies** -- multi-step escalation with configurable delays and repeat loops (up to 5 cycles)
- **Alert routing** -- content-based routing rules (contains, regex) to direct alerts to the right escalation policy
- **Alert deduplication and grouping** -- `dedup_key` prevents duplicate alerts; `group_key` suppresses redundant escalations
- **Incident management** -- track multi-service outages with timeline updates, correspondence, and related alerts
- **Multi-channel notifications** -- email (SMTP), SMS and phone calls (Twilio), Slack DM, webhooks with retry and exponential backoff
- **Inbound integrations** -- native support for Grafana Alerting, Prometheus Alertmanager, and generic webhooks
- **Built-in web UI** -- manage everything from the browser. No separate frontend deployment.
- **Per-user auth** -- email/password login, argon2id hashing, server-side sessions, per-user API tokens, invite flow, RBAC
- **Security defaults** -- rate limiting, SSRF protection on outbound webhooks, CSP headers, parameterized queries

## Getting started

```bash
# Option 1: Docker
docker compose up -d

# Option 2: Binary
curl -LO https://github.com/pagefire/pagefire/releases/latest/download/pagefire-linux-amd64
chmod +x pagefire-linux-amd64
./pagefire-linux-amd64 serve
```

Open http://localhost:3000. The setup wizard creates your admin account. Create a service, generate an integration key, point your monitoring at it, and you're receiving alerts.

If you're migrating from Grafana OnCall, we have a [step-by-step migration guide](migrating-from-grafana-oncall.md) that maps every OnCall concept to its PageFire equivalent.

## How it compares

| | PageFire | PagerDuty | Grafana OnCall | OneUptime | GoAlert |
|---|---|---|---|---|---|
| **Deployment** | Single binary | SaaS only | Archived | 10+ containers | Single binary |
| **Database** | Embedded SQLite | N/A | PG + Redis + RabbitMQ | PG + Redis + ClickHouse | PostgreSQL |
| **RAM** | ~50 MB | N/A | ~2 GB | ~8 GB | ~100 MB |
| **License** | MIT | Proprietary | Apache 2.0 (dead) | MIT | Apache 2.0 |
| **Incidents** | Yes | Yes | Yes | Yes | No |
| **Self-hosted** | Yes | No | No longer maintained | Yes (complex) | Yes |
| **Price** | Free | $21/user/mo+ | Unsupported | $120/mo+ | Free |

## What's next

PageFire today covers on-call and incident management. We're building toward a unified platform:

- **Phase 2: Uptime monitoring** -- HTTP, TCP, ping, and DNS health checks with configurable thresholds. Failed checks automatically create alerts that flow through your existing escalation policies. Think built-in Uptime Kuma, no separate tool needed.
- **Phase 3: Status pages** -- public-facing status communication with auto-updates from incidents and monitors, subscriber notifications, custom domains, and embeddable widgets.
- **Postgres support** -- the store interface is already abstracted. Postgres is coming for teams that need HA.

The goal is one binary that replaces PagerDuty + Uptime Kuma + Atlassian Statuspage.

## Try it

PageFire is MIT licensed and free to self-host.

- **GitHub**: [github.com/pagefire/pagefire](https://github.com/pagefire/pagefire) -- star the repo if this is useful to you
- **Website**: [pagefire.dev](https://pagefire.dev)
- **Migration guide**: [Migrating from Grafana OnCall](https://github.com/pagefire/pagefire/blob/main/docs/migrating-from-grafana-oncall.md)

If you find bugs, open an issue. If you want to contribute, PRs are welcome -- just open an issue first to discuss. We're building this in the open and shipping fast.
