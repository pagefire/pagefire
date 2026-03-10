# Migrating from Grafana OnCall to PageFire

Grafana OnCall OSS enters maintenance mode and will be archived on March 24, 2026. Cloud Connection (free SMS, phone, mobile push) shuts down the same day. If you're looking for a self-hosted alternative, PageFire covers the core on-call workflow in a single Go binary with zero external dependencies.

This guide maps OnCall concepts to PageFire equivalents and walks you through recreating your setup.

## Concept Mapping

| Grafana OnCall | PageFire | Notes |
|---|---|---|
| Integration | Integration Key | Inbound webhook endpoint for a service |
| Alert Group | Alert (with `dedup_key`) | Dedup groups alerts by key; grouping by attributes is on the roadmap |
| Escalation Chain | Escalation Policy | Multi-step, with repeat/loop support |
| Escalation Chain Step | Escalation Step + Targets | Steps target users or on-call schedules |
| Route | — | Content-based routing is on the roadmap |
| Schedule | Schedule | Named on-call schedule with timezone |
| On-Call Shift | Rotation | Daily, weekly, or custom-hour rotations |
| Override | Schedule Override | Temporary user swap for a time window |
| Shift Swap | — | Self-service swaps are on the roadmap; use overrides for now |
| Personal Notification Rules | Notification Rules | Per-user delay + contact method |
| Contact Point (Slack, Email, etc.) | Contact Method | Email, webhook, Slack DM |
| Resolution Note | Alert Log | Audit trail entries on alerts |
| Outgoing Webhook | Webhook contact method | Event-driven outgoing webhooks are on the roadmap |
| Team | Team | Create teams and manage membership via API |

## Prerequisites

- Go 1.22+ (to build from source) or a [pre-built binary](https://github.com/pagefire/pagefire/releases)
- ~5 minutes

## Step 1: Start PageFire

Download the latest release for your platform, or build from source:

```bash
# Option A: Download binary (example for Linux amd64)
curl -LO https://github.com/pagefire/pagefire/releases/latest/download/pagefire-linux-amd64
chmod +x pagefire-linux-amd64
mv pagefire-linux-amd64 /usr/local/bin/pagefire

# Option B: Build from source
git clone https://github.com/pagefire/pagefire.git
cd pagefire
make build

# Option C: Docker
docker run -d -p 3000:3000 \
  -e PAGEFIRE_ADMIN_TOKEN=change-me-to-something-secret \
  ghcr.io/pagefire/pagefire:latest
```

Pick an admin token — this is your API password for all requests. It can be any string you choose:

```bash
export PAGEFIRE_ADMIN_TOKEN="change-me-to-something-secret"
pagefire serve
```

PageFire uses SQLite by default. Your database is created automatically at `./pagefire.db`.

For the rest of this guide, we'll use these variables:

```bash
TOKEN="$PAGEFIRE_ADMIN_TOKEN"
API="http://localhost:3000/api/v1"
```

## Step 2: Create Users

In OnCall, users come from Grafana's user system. In PageFire, you create them via the API.

For each team member who was in your OnCall rotation:

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
| Slack DM | `webhook` pointed at a Slack incoming webhook URL |
| Email | `email` (requires SMTP config — see below) |
| SMS | `webhook` pointed at a Twilio API endpoint or similar |
| Phone call | Not yet supported |
| Telegram | `webhook` pointed at Telegram bot API |
| Mobile push | `webhook` pointed at Pushover/ntfy/Gotify |

> **Note:** OnCall's Cloud Connection (which provided free SMS/phone/push) shuts down on March 24, 2026. If you were self-hosting Twilio for SMS, you can continue using it via a webhook contact method in PageFire.

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
| Shift swap | Use overrides; self-service swaps on the roadmap |
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

### Things that work differently
- **No Grafana dependency:** PageFire is standalone. No Grafana plugin needed.
- **No Celery/Redis/Postgres:** Single binary, SQLite. Nothing else to manage.
- **API-first:** No UI yet (on the roadmap). Everything is done via REST API.
- **Single admin token:** Instead of Grafana's user system, you set one API token via `PAGEFIRE_ADMIN_TOKEN` and include it in all requests. Per-user API tokens are on the roadmap.

### Features on the roadmap
- Content-based alert routing (OnCall's "Routes")
- Alert grouping by attributes
- Self-service shift swaps
- Team-based access control
- Event-driven outgoing webhooks
- Alert template customization
- Frontend UI
- ICS/iCal schedule export

## Configuration Reference

| Variable | Default | Description |
|---|---|---|
| `PAGEFIRE_ADMIN_TOKEN` | *(required)* | API token — include as `Authorization: Bearer <value>` in all requests |
| `PAGEFIRE_PORT` | `3000` | HTTP listen port |
| `PAGEFIRE_DATABASE_URL` | `./pagefire.db` | SQLite database path |
| `PAGEFIRE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `PAGEFIRE_ENGINE_INTERVAL_SECONDS` | `5` | How often the engine processes alerts |
| `PAGEFIRE_SMTP_HOST` | — | SMTP server for email notifications |
| `PAGEFIRE_SMTP_PORT` | `587` | SMTP port |
| `PAGEFIRE_SMTP_FROM` | — | Sender email address |
| `PAGEFIRE_SMTP_USERNAME` | — | SMTP auth username |
| `PAGEFIRE_SMTP_PASSWORD` | — | SMTP auth password |
| `PAGEFIRE_SLACK_BOT_TOKEN` | — | Slack bot token for DM notifications |
| `PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS` | `false` | Allow webhooks to private/localhost IPs |

## Getting Help

- GitHub: [github.com/pagefire/pagefire](https://github.com/pagefire/pagefire)
- Issues: [github.com/pagefire/pagefire/issues](https://github.com/pagefire/pagefire/issues)
