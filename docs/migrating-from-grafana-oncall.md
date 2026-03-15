# Migrating from Grafana OnCall to PageFire

Welcome. If you're here because Grafana OnCall OSS is being archived on March 24, 2026, you're in the right place. Cloud Connection (free SMS, phone, mobile push) shuts down the same day, and the project will no longer receive updates.

PageFire is a self-hosted, open-source alternative that covers the core on-call workflow — scheduling, escalations, alerting, and incident management — in a single Go binary with zero external dependencies. No Grafana plugin, no Redis, no RabbitMQ. SMS and phone call notifications are built in via Twilio.

This guide maps OnCall concepts to PageFire equivalents and walks you through recreating your setup. Most teams can complete the migration in under 30 minutes.

## Concept Mapping

| Grafana OnCall | PageFire | Notes |
|---|---|---|
| Integration | Integration Key | Inbound webhook endpoint for a service |
| Alert Group | Alert (with `dedup_key` + `group_key`) | Dedup prevents duplicates; `group_key` groups related alerts (only first escalates) |
| Escalation Chain | Escalation Policy | Multi-step, with repeat/loop support |
| Escalation Chain Step | Escalation Step + Targets | Steps target users or on-call schedules |
| Route | Routing Rule | Content-based routing by summary, details, or source (contains/regex) |
| Schedule | Schedule | Named on-call schedule with timezone |
| On-Call Shift | Rotation | Daily, weekly, or custom-hour rotations |
| Override | Schedule Override | Temporary user swap for a time window |
| Shift Swap | Schedule Override | Any user can create overrides to swap shifts (no admin needed) |
| Personal Notification Rules | Notification Rules | Per-user delay + contact method |
| Contact Point (Slack, Email, etc.) | Contact Method | Email, webhook, Slack DM |
| Resolution Note | Alert Log | Audit trail entries on alerts |
| Outgoing Webhook | Webhook contact method | Event-driven outgoing webhooks are on the roadmap |
| Team | Team | Create teams and manage membership via API |
| Incident | Incident | Track multi-service outages with timeline updates and severity levels |

## Prerequisites

- Docker (recommended) or Go 1.22+ (to build from source) or a [pre-built binary](https://github.com/pagefire/pagefire/releases)
- ~30 minutes for a full migration

## Step 1: Start PageFire

The fastest way to get running is Docker Compose:

```bash
git clone https://github.com/pagefire/pagefire.git
cd pagefire
docker compose up -d
```

This starts PageFire on port 3000 with a persistent data volume. See [docs/docker.md](docker.md) for production configuration, including how to set environment variables for SMTP, Twilio, and Slack.

Alternatively, download a binary or build from source:

```bash
# Option A: Download binary (example for Linux amd64)
curl -LO https://github.com/pagefire/pagefire/releases/latest/download/pagefire-linux-amd64
chmod +x pagefire-linux-amd64
mv pagefire-linux-amd64 /usr/local/bin/pagefire
pagefire serve

# Option B: Build from source
git clone https://github.com/pagefire/pagefire.git
cd pagefire
make build
./bin/pagefire serve
```

PageFire uses SQLite by default. Your database is created automatically at `./pagefire.db`.

Open **http://localhost:3000** in your browser. On first run, you'll see a setup wizard to create your admin account (name, email, password).

For the API examples below, generate a per-user API token from **Profile & Settings > API Tokens** in the web UI:

```bash
TOKEN="pf_your-api-token"
API="http://localhost:3000/api/v1"
```

## Step 2: Create Users

In OnCall, users come from Grafana's user system. In PageFire, you create users through the web UI or API. Admins create users with name, email, and role — the system generates a one-time invite link so each user sets their own password.

**Via web UI:** Go to **Users** and click **Add User**. You'll get an invite link to share with the new team member.

**Via API:**

```bash
curl -s -X POST "$API/users" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice Chen",
    "email": "alice@example.com",
    "timezone": "America/Los_Angeles"
  }'
```

The response includes an `invite_url`. Share this with the user — they'll open it in their browser to set their password and activate their account.

Save the `id` from the response — you'll need it in the next steps.

## Step 3: Set Up Contact Methods and Notification Rules

In OnCall, you configured "Personal Notification Rules" with steps like: Slack DM -> wait 5 min -> SMS -> wait 10 min -> phone call.

In PageFire, you set up contact methods (where to reach someone) and notification rules (when to use each method) per user:

```bash
USER_ID="<user-id-from-step-2>"

# Add a webhook contact method (replaces Slack/SMS/etc.)
curl -s -X POST "$API/users/$USER_ID/contact-methods" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type": "webhook", "value": "https://your-slack-webhook-url"}'

# Add an email contact method
curl -s -X POST "$API/users/$USER_ID/contact-methods" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type": "email", "value": "alice@example.com"}'
```

Then create notification rules to control timing. Use the `id` from each contact method response:

```bash
# Notify via webhook immediately when an alert fires
curl -s -X POST "$API/users/$USER_ID/notification-rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"contact_method_id": "<webhook-cm-id>", "delay_minutes": 0}'

# Notify via email after 5 minutes if not acknowledged
curl -s -X POST "$API/users/$USER_ID/notification-rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"contact_method_id": "<email-cm-id>", "delay_minutes": 5}'
```

### Notification channel mapping

| OnCall Channel | PageFire Equivalent |
|---|---|
| Slack DM | `slack_dm` (native — set `PAGEFIRE_SLACK_BOT_TOKEN`) |
| Email | `email` (native — requires SMTP config) |
| SMS | `sms` (native — requires Twilio config, see [Twilio setup guide](twilio-setup.md)) |
| Phone call | `phone` (native — requires Twilio config, see [Twilio setup guide](twilio-setup.md)) |
| Telegram | `webhook` pointed at Telegram bot API |
| Mobile push | `webhook` pointed at Pushover/ntfy/Gotify |

> **Note:** OnCall's Cloud Connection (which provided free SMS/phone/push) shuts down on March 24, 2026. PageFire has native SMS and phone call support via Twilio — no third-party relay needed. See the [Twilio setup guide](twilio-setup.md) to configure it.

## Step 4: Create an Escalation Policy

OnCall's "Escalation Chains" map to PageFire's "Escalation Policies."

```bash
# Create the policy
EP_ID=$(curl -s -X POST "$API/escalation-policies" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Production Alerts", "repeat": 2}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# Add step 1: notify on-call user, re-escalate after 5 min if no response
STEP1_ID=$(curl -s -X POST "$API/escalation-policies/$EP_ID/steps" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"step_number": 0, "delay_minutes": 5}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# We'll add the schedule target in Step 5 after creating the schedule.

# Add step 2: notify a specific user as backup
STEP2_ID=$(curl -s -X POST "$API/escalation-policies/$EP_ID/steps" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"step_number": 1, "delay_minutes": 10}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

curl -s -X POST "$API/escalation-policies/$EP_ID/steps/$STEP2_ID/targets" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_type\": \"user\", \"target_id\": \"$USER_ID\"}"
```

### Escalation step mapping

| OnCall Step Type | PageFire Equivalent |
|---|---|
| Notify users from on-call schedule | Step target with `target_type: "schedule"` |
| Notify specific user | Step target with `target_type: "user"` |
| Wait N minutes | `delay_minutes` on the step |
| Repeat escalation chain | `repeat` on the escalation policy (0-5) |
| Notify Slack channel | Not yet supported as an escalation step |
| Trigger outgoing webhook | On the roadmap |

## Step 5: Create an On-Call Schedule

OnCall's web-based schedule builder with rotation layers maps to PageFire's schedules with rotations:

```bash
# Create the schedule
SCHED_ID=$(curl -s -X POST "$API/schedules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Primary On-Call", "timezone": "America/Los_Angeles"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# Create a weekly rotation starting now
ROT_ID=$(curl -s -X POST "$API/schedules/$SCHED_ID/rotations" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Weekly Rotation\",
    \"type\": \"weekly\",
    \"shift_length\": 1,
    \"start_time\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
    \"handoff_time\": \"09:00\"
  }" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# Add participants in rotation order
curl -s -X POST "$API/schedules/$SCHED_ID/rotations/$ROT_ID/participants" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\": \"$USER_ID\", \"position\": 0}"

# Add more participants at position 1, 2, etc.
```

Now wire the schedule into your escalation policy (the step 1 target from Step 4):

```bash
curl -s -X POST "$API/escalation-policies/$EP_ID/steps/$STEP1_ID/targets" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_type\": \"schedule\", \"target_id\": \"$SCHED_ID\"}"
```

### Schedule feature mapping

| OnCall Feature | PageFire Equivalent |
|---|---|
| Rotation layer | Rotation (daily/weekly/custom) |
| Override | Schedule Override (`POST /schedules/{id}/overrides`) |
| Shift swap | Schedule Override (all users can create overrides from the UI) |
| ICS/iCal export | On the roadmap |
| Schedule quality report | On the roadmap |

## Step 6: Create a Service and Integration Key

In OnCall, an "Integration" is an inbound alert endpoint. In PageFire, you create a Service (linked to an escalation policy) and then create an Integration Key on that service:

```bash
# Create the service
SVC_ID=$(curl -s -X POST "$API/services" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"Production API\", \"escalation_policy_id\": \"$EP_ID\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# Create an integration key (the secret is only shown once)
curl -s -X POST "$API/services/$SVC_ID/integration-keys" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Grafana Alerting"}'
```

Save the `secret` from the response — **it is only shown once**. This secret is the URL path for inbound alerts:

```
http://localhost:3000/api/v1/integrations/<secret>/grafana
```

### Integration type mapping

| OnCall Integration | PageFire Endpoint |
|---|---|
| Grafana Alerting | `POST /api/v1/integrations/{key}/grafana` |
| Alertmanager / Prometheus | `POST /api/v1/integrations/{key}/prometheus` |
| Generic Webhook | `POST /api/v1/integrations/{key}/alerts` |
| Datadog, PagerDuty, Sentry, etc. | Use generic webhook with field mapping |

For OnCall integrations that don't have a native PageFire equivalent (Datadog, Sentry, etc.), use the generic webhook endpoint. Most monitoring tools can send a JSON payload with `summary`, `details`, and `dedup_key` fields:

```bash
curl -X POST "http://localhost:3000/api/v1/integrations/<secret>/alerts" \
  -H "Content-Type: application/json" \
  -d '{
    "summary": "High CPU on web-01",
    "details": "CPU usage at 95% for 5 minutes",
    "dedup_key": "cpu-web-01"
  }'
```

## Step 7: Update Your Alert Sources

Point your existing alert sources at PageFire's integration URLs:

**Grafana Alerting:** In Grafana, go to Alerting > Contact Points. Edit your OnCall contact point and change the URL to your PageFire integration endpoint.

**Prometheus Alertmanager:** Update your `alertmanager.yml`:

```yaml
receivers:
  - name: pagefire
    webhook_configs:
      - url: 'http://pagefire:3000/api/v1/integrations/<your-secret>/prometheus'
```

**Other tools:** Replace the OnCall webhook URL with the PageFire generic webhook URL in your monitoring tool's notification settings.

## Step 8: Test the Setup

Check who's on-call:

```bash
curl -s "$API/oncall/$SCHED_ID" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

Fire a test alert:

```bash
curl -X POST "http://localhost:3000/api/v1/integrations/<secret>/alerts" \
  -H "Content-Type: application/json" \
  -d '{"summary": "Test alert from migration", "dedup_key": "migration-test"}'
```

Check alerts:

```bash
curl -s "$API/alerts" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

## What's Different

### Things that work the same
- Alerts flow in via webhooks, get routed through escalation steps, and notify the on-call user
- Schedules rotate users on daily/weekly/custom cadences
- Overrides let you temporarily swap who's on-call
- Deduplication prevents duplicate alerts from the same source
- Escalation policies loop through steps and repeat
- Incidents can track multi-service outages with timeline updates

### Things that work differently
- **No Grafana dependency:** PageFire is standalone. No Grafana plugin needed.
- **No Celery/Redis/Postgres:** Single binary, SQLite. Nothing else to manage.
- **Built-in web UI:** Full dashboard for managing alerts, services, schedules, teams, and users.
- **Per-user auth:** Email/password login with session cookies. Per-user API tokens (prefixed `pf_`) for scripts. Admin and member roles with RBAC enforcement.
- **User invite flow:** Admins create users and share an invite link. Users set their own passwords — admins never handle credentials.
- **Native SMS and phone calls:** Built-in Twilio integration — no Cloud Connection or external relay.

## What's Not Supported Yet

PageFire focuses on on-call management today. The following features are on the roadmap but not yet available:

| Feature | Status | Notes |
|---|---|---|
| Uptime monitoring (HTTP/TCP/ping) | Planned | Health-check based monitoring is next after on-call |
| Public status pages | Planned | Hosted and self-hosted status pages |
| Event-driven outgoing webhooks | Planned | Trigger webhooks on alert lifecycle events |
| Alert template customization | Planned | Customize notification content per service |
| ICS/iCal schedule export | Planned | Export schedules to calendar apps |
| Terraform provider | Planned | Manage configuration as code |
| Mobile app | Not planned yet | Use webhook notifications to Pushover/ntfy as a workaround |
| Slack channel notifications | Not planned yet | Escalation steps can target users only, not channels |

If any of these are critical for your migration, [open an issue](https://github.com/pagefire/pagefire/issues) — it helps us prioritize.

## Configuration Reference

All configuration is via environment variables. See [docs/docker.md](docker.md) for Docker Compose examples.

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_DATABASE_DRIVER` | `sqlite` | Database driver (`sqlite`; Postgres planned) |
| `PAGEFIRE_DATABASE_URL` | `./pagefire.db` | Database connection string |
| `PAGEFIRE_DATA_DIR` | `.` | Data directory for SQLite |
| `PAGEFIRE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PAGEFIRE_ENGINE_INTERVAL_SECONDS` | `5` | How often the engine processes alerts |
| `PAGEFIRE_SMTP_HOST` | — | SMTP server for email notifications |
| `PAGEFIRE_SMTP_PORT` | `587` | SMTP port |
| `PAGEFIRE_SMTP_FROM` | — | Sender email address |
| `PAGEFIRE_SMTP_USERNAME` | — | SMTP auth username |
| `PAGEFIRE_SMTP_PASSWORD` | — | SMTP auth password |
| `PAGEFIRE_TWILIO_ACCOUNT_SID` | — | Twilio account SID (enables SMS and phone) |
| `PAGEFIRE_TWILIO_AUTH_TOKEN` | — | Twilio auth token |
| `PAGEFIRE_TWILIO_FROM_NUMBER` | — | Twilio phone number in E.164 format (e.g. `+12025551234`) |
| `PAGEFIRE_SLACK_BOT_TOKEN` | — | Slack bot token for DM notifications |
| `PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS` | `false` | Allow webhooks to private/localhost IPs |

## Getting Help

If you run into issues during migration, we want to hear about it:

- GitHub: [github.com/pagefire/pagefire](https://github.com/pagefire/pagefire)
- Issues: [github.com/pagefire/pagefire/issues](https://github.com/pagefire/pagefire/issues)

If you're coming from Grafana OnCall and there's a feature gap that blocks your migration, please open an issue with the `grafana-oncall-migration` label. These get priority attention.
