# PageFire Landing Page Content

Content draft for pagefire.dev. Not a deployable page — structured sections for the website build.

---

## Hero

**Headline:**
Open-source on-call management. One binary. Zero complexity.

**Subheadline:**
Alert routing, escalation policies, on-call schedules, incident management, and multi-channel notifications — all in a single self-hosted binary.

**CTA buttons:**
- Get Started -> /docs/quickstart (or GitHub releases page)
- View on GitHub -> https://github.com/pagefire/pagefire

**Terminal snippet (below CTAs):**
```bash
curl -LO https://github.com/pagefire/pagefire/releases/latest/download/pagefire-linux-amd64
chmod +x pagefire-linux-amd64
./pagefire-linux-amd64 serve
# Open http://localhost:3000
```

---

## Problem Statement

PagerDuty costs $21-49/user/month for what should be table stakes. Grafana OnCall OSS was archived on March 24, 2026, leaving thousands of self-hosters without a maintained alternative. OneUptime tries to replace everything but needs 10+ Docker containers and 8 GB of RAM just to start.

You shouldn't need a distributed system to manage your on-call schedule.

---

## Feature Grid

Six boxes, equal weight. Icon + title + one-liner.

### On-Call Scheduling
Daily, weekly, and custom rotations with participant ordering. Schedule overrides for PTO and shift swaps — any user can create them, no admin needed.

### Escalation Policies
Multi-step escalation with configurable delays. Target specific users or whoever is on-call. Repeat loops until someone responds. Policy snapshots frozen per-alert so mid-incident changes don't cause chaos.

### Multi-Channel Notifications
SMS and phone calls via Twilio. Email over SMTP. Slack DMs. Outbound webhooks with SSRF protection. Per-user notification rules with configurable delays — Slack immediately, email after 5 minutes, phone call after 15.

### Incident Management
Track multi-service outages with severity levels, timeline updates, and correspondence notes. Link related alerts. Editable titles and summaries. Full audit trail of status changes with author attribution.

### Alert Routing
Integration keys per service. Content-based routing rules (contains, regex) to direct alerts to different escalation policies. Deduplication via `dedup_key`. Grouping via `group_key` to suppress redundant escalations. Native Grafana and Prometheus Alertmanager endpoints.

### Self-Hosted
Single Go binary, ~50 MB RAM, embedded SQLite. No Redis, no RabbitMQ, no Postgres required. Docker image available. Starts in seconds on a $5/month VPS. MIT licensed. Your data stays on your infrastructure.

---

## Comparison Table

|  | PageFire | PagerDuty | Grafana OnCall | OneUptime | Better Stack | GoAlert |
|---|---|---|---|---|---|---|
| **Self-hosted** | Yes | No | Archived | Yes | No | Yes |
| **Single binary** | Yes | N/A | No | No (10+ containers) | N/A | Yes |
| **SMS / Phone** | Yes (Twilio) | Yes | Archived | Yes | Yes | Yes |
| **Incidents** | Yes | Yes | Yes | Yes | Yes | No |
| **Status pages** | Coming soon | Separate product ($) | No | Yes | Yes | No |
| **Monitoring** | Coming soon | No | Via Grafana | Yes | Yes | No |
| **License** | MIT | Proprietary | Apache 2.0 (archived) | MIT | Proprietary | Apache 2.0 |
| **Min RAM** | ~50 MB | N/A | ~2 GB | ~8 GB | N/A | ~50 MB |
| **Price** | Free | $21/user/mo+ | Unsupported | $120/mo+ | $29/mo+ | Free |

---

## "Why PageFire?"

Four points. No paragraph text — just the facts.

**Single binary, <50 MB, starts in seconds.**
Download, run, done. No containers, no message queues, no external databases. One process, one SQLite file.

**SQLite by default. Postgres when you need it.**
Zero-config storage that works out of the box. Postgres support available when you outgrow a single node.

**MIT licensed. No vendor lock-in.**
Read the source. Fork it. Self-host forever. No open-core bait-and-switch on core features.

**Built for teams leaving Grafana OnCall.**
Concept-for-concept mapping. Migration guide included. Same workflow — schedules, escalations, integrations — without the Grafana dependency or the 6-container stack.

---

## Grafana OnCall Migration Callout

Standalone section with a distinct visual treatment (banner or card).

**Heading:**
Grafana OnCall OSS was archived March 24, 2026. PageFire is your migration path.

**Body:**
OnCall's open-source version is no longer maintained. Cloud Connection (free SMS, phone, mobile push) is shut down. Grafana is pushing everyone to paid Cloud IRM.

PageFire maps every core OnCall concept — integrations, escalation chains, schedules, rotations, overrides, notification rules — to a single binary you own and run yourself. No Grafana dependency. No Celery. No Redis.

**CTA:**
Read the Migration Guide -> /docs/migrating-from-grafana-oncall

---

## How It Works

Four steps. Keep it concrete.

**1. Deploy in 30 seconds**
Download the binary or `docker compose up`. Open localhost:3000. Create your admin account in the setup wizard.

**2. Set up your on-call rotation**
Create a schedule, add a weekly rotation, assign your team. Create an escalation policy: notify on-call first, escalate to the backup after 5 minutes.

**3. Connect your monitoring**
Generate an integration key on your service. Point Grafana, Prometheus Alertmanager, or any webhook-capable tool at the endpoint. No auth headers — the key in the URL is the credential.

**4. Get paged**
Alert fires. Engine evaluates escalation policy. On-call user gets notified via their preferred channel (Slack, email, SMS, phone). Acknowledge or resolve from the web UI or API.

```bash
# Fire a test alert
curl -X POST http://localhost:3000/api/v1/integrations/<key>/alerts \
  -H "Content-Type: application/json" \
  -d '{"summary": "High CPU on web-01", "dedup_key": "cpu-web-01"}'
```

---

## Integrations

**Inbound (alert sources):**
- Grafana Alerting — native endpoint
- Prometheus Alertmanager — native endpoint
- Generic webhook — any tool that sends JSON over HTTP

**Outbound (notifications):**
- Email (SMTP)
- SMS (Twilio)
- Phone calls (Twilio)
- Slack DMs
- Webhooks (Pushover, ntfy, Telegram bots, anything)

---

## Security

Not a full section — a compact list in the footer area or a sidebar.

- Argon2id password hashing
- Server-side sessions with HttpOnly cookies
- Per-user API tokens with hash-only storage (prefixed `pf_`)
- RBAC enforcement on all write operations
- Per-IP rate limiting (login, API, integrations)
- SSRF protection on outbound webhooks
- Parameterized SQL queries
- One-time invite tokens with SHA-256 hashing

---

## Roadmap

Short, honest. Three phases.

**Now: On-Call & Incidents** (Phase 1 — shipping)
Schedules, escalation policies, alert routing, incident management, multi-channel notifications. Full Grafana OnCall replacement.

**Next: Uptime Monitoring** (Phase 2)
HTTP, TCP, ping, and DNS health checks. Auto-alert on failure. Built-in Uptime Kuma — no separate tool needed.

**Later: Status Pages** (Phase 3)
Public and private status pages. Auto-update from incidents and monitors. Subscriber notifications. Custom domains. Embeddable widgets.

---

## Footer CTA

**Heading:**
Stop paying per-seat for on-call.

**CTA buttons:**
- Get Started -> GitHub releases
- Star on GitHub -> https://github.com/pagefire/pagefire
