# Mayday — Architecture & Technical Design

> "OneUptime's scope in GoAlert's footprint."
> Single Go binary. On-call + monitoring + status pages. MIT license.

---

## Executive Summary

**What is Mayday?** A single, self-hosted Go binary that answers three questions for engineering teams:

| Question | Product Pillar | Existing Alternatives |
|----------|---------------|----------------------|
| "Who gets paged at 3am?" | On-Call Engine | PagerDuty ($21+/user), GoAlert |
| "Is my service up?" | Uptime Monitoring | Uptime Kuma, Datadog Synthetics |
| "What do my users see?" | Status Pages | Atlassian Statuspage ($79/mo) |

**Why now?** Grafana OnCall OSS is being archived on March 24, 2026. Thousands of self-hosted teams need an alternative. We start with on-call (the wedge), then layer in monitoring and status pages.

**Key architectural bets:**
- **Three pillars, one binary** — each pillar ships independently but composes into a single workflow
- **Services as the linchpin** — the central entity all three pillars connect to
- **Shared platform layer** — incidents, notifications, engine are built once, used by all pillars
- **Dual-database** — SQLite (self-hosted) and Postgres (cloud/production) behind repository interfaces
- **No CGO, no Redis, no message queue** — zero external dependencies beyond the database

---

## Table of Contents

1. [Goals & Constraints](#1-goals--constraints)
2. [Product Pillars & Phasing](#2-product-pillars--phasing)
3. [System Overview](#3-system-overview)
4. [Shared Platform Layer](#4-shared-platform-layer)
5. [Product 1: On-Call Engine](#5-product-1-on-call-engine)
6. [Product 2: Uptime Monitoring](#6-product-2-uptime-monitoring)
7. [Product 3: Status Pages](#7-product-3-status-pages)
8. [Integration Seams](#8-integration-seams)
9. [API Design](#9-api-design)
10. [Web UI](#10-web-ui)
11. [Storage Layer](#11-storage-layer)
12. [Multi-Tenancy & Cloud Monetization](#12-multi-tenancy--cloud-monetization)
13. [Migration Tool (Grafana OnCall Import)](#13-migration-tool-grafana-oncall-import)
14. [Project Structure](#14-project-structure)
15. [Build & Packaging](#15-build--packaging)
16. [Extensibility](#16-extensibility)
17. [Key Architectural Decisions](#17-key-architectural-decisions)

---

## 1. Goals & Constraints

### Goals

- **Single binary**: `mayday serve` and you're running. No external dependencies except the database.
- **Three product pillars**: On-call scheduling/escalation, uptime monitoring, public status pages. Nothing else.
- **Phased delivery**: Each pillar ships as a complete, useful product. On-call ships first. Monitoring and status pages layer on top without rework.
- **10-minute setup**: Install → configure first schedule → receive first alert. No YAML, no Kubernetes, no infra knowledge required.
- **Self-hosted first**: Fully functional MIT-licensed self-hosted version. Cloud version monetizes convenience, not features.
- **Grafana OnCall migration**: Import schedules, escalation chains, and integrations from Grafana OnCall.
- **Production-grade reliability**: If Mayday fails to deliver a page at 3am, nothing else matters.

### Non-Goals (What We Are NOT Building)

- APM, distributed tracing, log aggregation, metrics pipelines
- A Datadog/Grafana/OneUptime clone
- A general-purpose alerting rule engine (we receive alerts or generate them from monitors — we don't evaluate PromQL)

### Constraints

- **No CGO**: Pure Go binary for easy cross-compilation. This constrains our SQLite driver choice.
- **Minimal infrastructure**: Self-hosted version needs nothing beyond the binary + a data directory. No Redis, no message queue, no external cache.
- **Database portability**: Must work with SQLite (single-node) AND Postgres (production/cloud). Same codebase, same schema.
- **Pillar independence**: Each product pillar must be usable standalone. A user who only wants on-call should never see monitoring UI. A user who only wants a status page should be able to have one.

---

## 2. Product Pillars & Phasing

Mayday is three products that compose into one workflow. Each pillar is independently useful, but their power is in the integration.

### The Three Pillars

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│   PILLAR 1: ON-CALL          PILLAR 2: MONITORING      PILLAR 3: STATUS│
│   "Who gets paged?"          "Is it up?"                "What do users  │
│                                                          see?"          │
│   ┌───────────────┐          ┌───────────────┐          ┌─────────────┐│
│   │ Schedules     │          │ HTTP checks   │          │ Public page ││
│   │ Rotations     │          │ TCP checks    │          │ Components  ││
│   │ Escalation    │◄─────────│ DNS checks    │─────────►│ Uptime graph││
│   │ Alerts        │ creates  │ SSL checks    │  feeds   │ Incidents   ││
│   │ Notifications │ alerts   │ Heartbeat     │  uptime  │ Subscribers ││
│   └───────┬───────┘          │ Ping          │          │ Custom DNS  ││
│           │                  └───────────────┘          └──────┬──────┘│
│           │                                                    │       │
│           └─────────── auto-updates ──────────────────────────►│       │
│                                                                        │
│   ═══════════════════ SHARED PLATFORM ═══════════════════════════      │
│   Services │ Incidents │ Users │ Notifications │ API │ Store │ Engine  │
│                                                                        │
└─────────────────────────────────────────────────────────────────────────┘
```

### The Full Loop (When All Three Pillars Are Live)

```
Monitor detects failure (Pillar 2)
  → Creates alert on service (Pillar 2 → Shared)
    → Escalation policy pages on-call engineer (Shared → Pillar 1)
      → Incident auto-created (Shared)
        → Status page component → "Major Outage" (Shared → Pillar 3)
          → Subscribers notified (Pillar 3)

Engineer acknowledges → Status page shows "Investigating"
Engineer resolves → Monitor confirms recovery
  → Incident auto-resolved → Status page → "Operational"
    → Subscribers notified of resolution
```

### Phased Delivery Plan

Each product pillar has its own release milestones. The phases are **sequential** — we ship Pillar 1 completely before starting Pillar 2 code.

#### Product 1: On-Call Engine

| Release | What Ships | Why This Order |
|---------|-----------|----------------|
| **v0.1** | Schedules, rotations, escalation policies, alerts, webhook/email/Slack notifications, dashboard, REST API | Core on-call loop. Capture Grafana OnCall refugees. |
| **v0.2** | Grafana OnCall migration tool, heartbeat monitors, improved dashboard | Lower switching cost to zero. Heartbeat is quick to build, high demand. |
| **v0.3** | Alert grouping (dedup_key), Slack interactive ack/resolve, Discord provider | Polish and community-requested channels. |
| **v0.4** | Twilio SMS/voice notifications, DTMF ack/resolve | Premium notification channels. Revenue enabler for cloud. |

#### Product 2: Uptime Monitoring

| Release | What Ships | Why This Order |
|---------|-----------|----------------|
| **v0.5** | HTTP monitors, check runner, uptime history, monitor → alert bridge | Connect monitoring to on-call. One tool instead of two. |
| **v0.6** | TCP/DNS/SSL/Ping monitors, flap detection, multi-region checks | Completeness. SSL expiry is high demand. |
| **v0.7** | Remote monitoring agents, alert rules (auto-ack, auto-resolve) | Production-grade for serious teams. |

#### Product 3: Status Pages

| Release | What Ships | Why This Order |
|---------|-----------|----------------|
| **v0.8** | Public status page, components (manual + auto from monitors), incident timeline | Close the full loop. "Powered by Mayday" virality. |
| **v0.9** | Subscriber notifications (email), custom branding, custom domains (self-hosted) | Differentiation from free alternatives. |
| **v1.0** | Cloud-hosted status pages with auto-SSL, managed custom domains, "Powered by" removal | Revenue-generating feature for cloud tier. |

#### Cross-Cutting Milestones

| Release | What Ships |
|---------|-----------|
| **v1.1** | Multi-instance HA (Postgres-only), OIDC/SSO |
| **v1.2** | Terraform provider, public API v2 |

### What Ships in v0.1 — The Concrete Boundary

**Schema (tables created at v0.1):**
- `users`, `contact_methods`, `notification_rules`
- `services`, `integration_keys`
- `escalation_policies`, `escalation_steps`, `escalation_step_targets`
- `schedules`, `rotations`, `rotation_participants`, `schedule_overrides`
- `alerts`, `alert_logs`
- `notification_queue`
- `incidents`, `incident_services` ← created but minimal (manual only, needed as shared object for future pillars)

**Schema NOT created at v0.1 (deferred to later pillars):**
- `monitors`, `monitor_checks` → v0.5
- `status_pages`, `status_page_components`, `status_page_subscribers`, `status_page_incidents`, `status_page_incident_updates` → v0.8

**API endpoints at v0.1:**
- Full CRUD: services, escalation-policies, schedules, rotations, overrides, users, contact-methods, notification-rules
- Alerts: create, list, get, acknowledge, resolve
- Integrations: inbound alert webhooks (generic, Grafana, Prometheus)
- On-call: who's on call now/at time T

**API endpoints NOT at v0.1:**
- `monitors/*` → v0.5
- `status-pages/*` → v0.8
- `heartbeat/*` → v0.2

**UI pages at v0.1:**
- Dashboard (active alerts, who's on-call now)
- Alerts (list, detail, ack/resolve)
- Services (CRUD, link to escalation policy)
- Escalation Policies (visual step editor)
- Schedules (calendar view, rotation management, overrides)
- Settings (users, integrations, notification channels)

**UI pages NOT at v0.1:**
- Monitors → v0.5
- Status Pages → v0.8
- Uptime graphs → v0.5

---

## 3. System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Mayday Binary                            │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ HTTP API │  │ Web UI   │  │ Engine   │  │ Monitor Runner│  │
│  │ (REST)   │  │(embedded)│  │ (ticker) │  │ (checks)      │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬────────┘  │
│       │              │             │               │            │
│       └──────────────┴─────────────┴───────────────┘            │
│                            │                                    │
│                    ┌───────┴───────┐                             │
│                    │  Store Layer  │                             │
│                    │ (Repository)  │                             │
│                    └───────┬───────┘                             │
│                            │                                    │
└────────────────────────────┼────────────────────────────────────┘
                             │
                   ┌─────────┴─────────┐
                   │   SQLite / Postgres │
                   └───────────────────┘
```

**Component availability by pillar:**

| Component | Pillar 1 (v0.1) | Pillar 2 (v0.5) | Pillar 3 (v0.8) |
|-----------|:---:|:---:|:---:|
| HTTP API | Yes | + monitor endpoints | + status page endpoints |
| Web UI | Yes | + monitor pages | + status page pages |
| Engine | Escalation + Notification processors | + Monitor state processor | + Status page auto-update |
| Monitor Runner | No | Yes | — |
| Store Layer | Core tables | + monitor tables | + status page tables |

### Core Components

1. **HTTP API** — RESTful API for all CRUD operations, inbound alert webhooks, and integration endpoints
2. **Web UI** — Server-rendered HTML (Go templates + HTMX) embedded in the binary via `go:embed`
3. **Engine** — Background ticker loop (every 5s) that processes escalations, checks notification timeouts, and manages alert lifecycle. New processor modules are added per pillar.
4. **Monitor Runner** — *(Pillar 2)* Goroutine-based check scheduler that executes HTTP/TCP/DNS/SSL checks at configured intervals
5. **Store Layer** — Repository interfaces with SQLite and Postgres implementations behind a common interface. New store interfaces added per pillar.

---

## 4. Shared Platform Layer

These components are shared across all three pillars. They ship in v0.1 and are designed to accommodate future pillars without rework.

### 4.1 Services — The Central Connection Point

**Services** are the linchpin entity that all three pillars connect to. A service represents "a thing that can break."

```
                    ┌─────────────┐
                    │   Service   │
                    │ "Payment API"│
                    └──────┬──────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
   ┌──────┴──────┐  ┌─────┴──────┐  ┌──────┴──────────┐
   │ Escalation  │  │  Monitors  │  │ Status Page     │
   │ Policy      │  │ (Pillar 2) │  │ Component       │
   │ (Pillar 1)  │  │            │  │ (Pillar 3)      │
   └─────────────┘  └────────────┘  └─────────────────┘
```

Design rules for services:
- Every service has **exactly one** escalation policy (set in v0.1)
- A service has **zero or more** monitors (attached in v0.5)
- A service maps to **zero or more** status page components (attached in v0.8)
- When a monitor fails, it creates an alert on the **service**, which triggers the service's escalation policy
- When a service has an active incident, its status page components auto-update

This means in v0.1 we build services → escalation policy linking. In v0.5 we add services → monitor linking. In v0.8 we add services → status page component linking. No rework.

### 4.2 Incidents — The Shared Object

Incidents are the bridge between all three pillars. They're created in v0.1 (manually or by alert grouping) but designed to accommodate future pillar needs.

```sql
CREATE TABLE incidents (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'investigating',
    -- investigating, identified, monitoring, resolved
    severity    TEXT NOT NULL DEFAULT 'minor',  -- minor, major, critical
    summary     TEXT,
    source      TEXT NOT NULL DEFAULT 'manual',
    -- manual (v0.1), monitor (v0.5), alert_group (v0.1)
    created_by  TEXT REFERENCES users(id),
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP
);

CREATE TABLE incident_services (
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    service_id  TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    PRIMARY KEY(incident_id, service_id)
);

CREATE TABLE incident_updates (
    id          TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,  -- matches incident status values
    message     TEXT NOT NULL,
    created_by  TEXT REFERENCES users(id),
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**How incidents evolve across pillars:**

| Pillar | Incident Capability |
|--------|-------------------|
| v0.1 (On-Call) | Manual creation. Alerts can be grouped into incidents. Incident timeline via `incident_updates`. |
| v0.5 (Monitoring) | Monitor failure auto-creates incident on linked service. Monitor recovery auto-resolves. |
| v0.8 (Status Pages) | Incidents publish to status page. `status_page_incidents` links internal incident to public view. Subscriber notifications triggered on incident create/update/resolve. |

### 4.3 Users, Auth & Contact Methods

```sql
CREATE TABLE users (
    id          TEXT PRIMARY KEY,  -- UUID
    name        TEXT NOT NULL,
    email       TEXT UNIQUE NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user',  -- admin, user
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    avatar_url  TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE contact_methods (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,  -- email, sms, voice, slack_dm, webhook
    value       TEXT NOT NULL,  -- email addr, phone #, webhook URL, slack user ID
    verified    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notification_rules (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_method_id TEXT NOT NULL REFERENCES contact_methods(id) ON DELETE CASCADE,
    delay_minutes     INTEGER NOT NULL DEFAULT 0  -- 0 = immediate, 5 = after 5 min
);
```

### 4.4 Notification System

The notification system is shared across all pillars but serves different purposes:

| Pillar | Notification Use |
|--------|-----------------|
| On-Call | Alert escalation — page users via their notification rules |
| Monitoring | *(Uses on-call notifications via alert pipeline — no separate notification path)* |
| Status Pages | Subscriber notifications — email subscribers when incidents are created/updated/resolved |

This means two notification flows share the same infrastructure:

1. **On-call notifications** — user-targeted, routed through notification rules, supports webhook/email/Slack/SMS/voice
2. **Subscriber notifications** — audience-targeted, email-only, triggered by incident status changes

Both use the same `notification_queue` table and dispatcher, but with different provider routing.

#### Notification Queue (Database-Backed)

```sql
CREATE TABLE notification_queue (
    id                TEXT PRIMARY KEY,
    alert_id          TEXT REFERENCES alerts(id) ON DELETE CASCADE,  -- NULL for subscriber notifications
    contact_method_id TEXT REFERENCES contact_methods(id) ON DELETE CASCADE,  -- NULL for subscriber notifications
    user_id           TEXT REFERENCES users(id) ON DELETE CASCADE,  -- NULL for subscriber notifications
    subscriber_id     TEXT,  -- NULL for on-call notifications, set for subscriber notifications (v0.8)
    type              TEXT NOT NULL DEFAULT 'alert',  -- alert (v0.1), subscriber (v0.8)
    destination_type  TEXT NOT NULL,  -- email, sms, voice, slack_dm, webhook
    destination       TEXT NOT NULL,  -- the actual address/URL/number
    subject           TEXT,
    body              TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending',  -- pending, sending, sent, delivered, failed
    attempts          INTEGER NOT NULL DEFAULT 0,
    max_attempts      INTEGER NOT NULL DEFAULT 3,
    next_attempt_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at           TIMESTAMP,
    provider_id       TEXT,  -- external message ID (Twilio SID, etc.)
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

#### Provider Interface

```go
type Provider interface {
    Type() string  // "email", "slack", "twilio_sms", "twilio_voice", "webhook"
    Send(ctx context.Context, msg Message) (providerID string, err error)
    ValidateTarget(target string) error
}

type Message struct {
    To      string   // destination (email, phone, webhook URL, Slack user)
    Subject string
    Body    string
    AlertID string   // set for alert notifications
    Actions []Action // ack, resolve URLs (for alert notifications)
}
```

**v0.1 providers**: Webhook, Email (SMTP), Slack
**v0.2 providers**: Discord
**v0.4 providers**: Twilio SMS, Twilio Voice

### 4.5 Engine — The Tick Loop

The engine is a background loop that runs every 5 seconds. It's composed of **processor modules** that are registered per pillar.

```go
type Processor interface {
    Name() string
    Tick(ctx context.Context) error
}

type Engine struct {
    processors []Processor
    interval   time.Duration  // 5 seconds
}
```

**Processors by pillar:**

| Processor | Pillar | Ships in | Responsibility |
|-----------|--------|----------|---------------|
| `EscalationProcessor` | On-Call | v0.1 | Advance alerts where `next_escalation_at < now()`. Queue notifications. |
| `NotificationProcessor` | Shared | v0.1 | Drain `notification_queue`. Dispatch via providers. Handle retries. |
| `HeartbeatProcessor` | On-Call | v0.2 | Check heartbeat monitors for overdue pings. Create alerts. |
| `MonitorStateProcessor` | Monitoring | v0.5 | Process state transitions: up→down creates alert, down→up resolves. |
| `StatusPageProcessor` | Status Pages | v0.8 | Auto-update status page components from incident status changes. |
| `CleanupProcessor` | Shared | v0.1 | Purge old resolved alerts, old check results, expired overrides. |

New processors are registered at startup based on what's compiled in. Adding a processor is:
1. Implement `Processor` interface
2. Register in `engine.New()`

---

## 5. Product 1: On-Call Engine

### 5.1 Data Model

#### Services & Integrations

```sql
CREATE TABLE services (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT,
    escalation_policy_id TEXT REFERENCES escalation_policies(id),
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE integration_keys (
    id          TEXT PRIMARY KEY,
    service_id  TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,  -- generic, grafana, prometheus, email
    secret      TEXT NOT NULL,  -- the actual API key/token
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

#### Escalation Policies

```sql
CREATE TABLE escalation_policies (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    repeat      INTEGER NOT NULL DEFAULT 0,  -- how many times to loop (0-5)
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE escalation_steps (
    id                   TEXT PRIMARY KEY,
    escalation_policy_id TEXT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
    step_number          INTEGER NOT NULL,
    delay_minutes        INTEGER NOT NULL DEFAULT 5,
    UNIQUE(escalation_policy_id, step_number)
);

CREATE TABLE escalation_step_targets (
    id                 TEXT PRIMARY KEY,
    escalation_step_id TEXT NOT NULL REFERENCES escalation_steps(id) ON DELETE CASCADE,
    target_type        TEXT NOT NULL,  -- user, schedule, team, webhook
    target_id          TEXT NOT NULL   -- FK to the target entity
);
```

#### Schedules & Rotations

```sql
CREATE TABLE schedules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rotations (
    id            TEXT PRIMARY KEY,
    schedule_id   TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    type          TEXT NOT NULL,  -- daily, weekly, custom
    shift_length  INTEGER NOT NULL DEFAULT 1,  -- in units of type (e.g., 1 week)
    start_time    TIMESTAMP NOT NULL,
    handoff_time  TIME NOT NULL DEFAULT '09:00',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rotation_participants (
    id            TEXT PRIMARY KEY,
    rotation_id   TEXT NOT NULL REFERENCES rotations(id) ON DELETE CASCADE,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL,  -- order in rotation
    UNIQUE(rotation_id, position)
);

CREATE TABLE schedule_overrides (
    id            TEXT PRIMARY KEY,
    schedule_id   TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    start_time    TIMESTAMP NOT NULL,
    end_time      TIMESTAMP NOT NULL,
    replace_user  TEXT REFERENCES users(id),  -- user being replaced (NULL = add)
    override_user TEXT NOT NULL REFERENCES users(id),  -- user taking over
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

#### Alerts

```sql
CREATE TABLE alerts (
    id                     TEXT PRIMARY KEY,
    service_id             TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    incident_id            TEXT REFERENCES incidents(id),  -- optional grouping into incident
    summary                TEXT NOT NULL,
    details                TEXT,
    status                 TEXT NOT NULL DEFAULT 'triggered',  -- triggered, acknowledged, resolved
    source                 TEXT NOT NULL DEFAULT 'manual',     -- manual, integration, monitor (v0.5), heartbeat (v0.2)
    dedup_key              TEXT,
    escalation_policy_snapshot TEXT,  -- JSON snapshot of escalation policy at alert creation time
    escalation_step        INTEGER NOT NULL DEFAULT 0,
    next_escalation_at     TIMESTAMP,
    loop_count             INTEGER NOT NULL DEFAULT 0,
    acknowledged_by        TEXT REFERENCES users(id),
    resolved_by            TEXT REFERENCES users(id),
    created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at        TIMESTAMP,
    resolved_at            TIMESTAMP,
    UNIQUE(service_id, dedup_key)
);

CREATE TABLE alert_logs (
    id          TEXT PRIMARY KEY,
    alert_id    TEXT NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    event       TEXT NOT NULL,  -- created, escalated, notified, acknowledged, resolved
    message     TEXT,
    user_id     TEXT REFERENCES users(id),
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 5.2 Alert Pipeline & Lifecycle

#### Flow

```
Inbound alert (webhook/integration/manual)
  │
  ▼
Deduplication (service_id + dedup_key)
  │
  ├── Duplicate → suppress, update existing alert
  │
  ▼
Create Alert (status: triggered)
  │
  ▼
Snapshot escalation policy → store as JSON in alert.escalation_policy_snapshot
  │
  ▼
Engine picks up alert (next tick, ~5s)
  │
  ▼
Read escalation step from SNAPSHOT (not live policy)
  │
  ▼
Resolve step targets
  │
  ├── Step target = Schedule → calculate who's on-call right now
  ├── Step target = User → use directly
  └── Step target = Team → all team members
  │
  ▼
For each resolved user:
  Apply user's notification rules (delay offsets)
  │
  ▼
Enqueue notifications in notification_queue
  │
  ▼
Notification dispatcher sends via contact method
  │
  ├── Webhook → HTTP POST
  ├── Email → SMTP
  ├── Slack → Slack API
  ├── SMS → Twilio API (v0.4)
  └── Voice → Twilio API (v0.4)
  │
  ▼
No ack within step delay?
  │
  ▼
Advance to next escalation step (repeat until policy exhausted or loop limit)
  │
  ▼
User acknowledges (via API, Slack, SMS reply)
  → Alert status → acknowledged, escalation stops
  │
  ▼
User resolves
  → Alert status → resolved
  → If linked to incident → update incident (v0.1)
  → If incident linked to status page → auto-update component (v0.8)
```

#### Escalation Policy Snapshot (ADR)

**Critical design pattern from Grafana OnCall:** When an alert is created, the entire escalation policy (steps + targets) is serialized as JSON and stored on the alert as `escalation_policy_snapshot`. The engine executes from this snapshot, NOT the live policy.

**Why:** If someone edits an escalation policy while an alert is in-flight, the running escalation should not change. You don't want "I removed step 3" to cause an active 3am escalation to skip a step. The snapshot captures the policy as it was when the alert was created.

```go
type EscalationSnapshot struct {
    PolicyID    string             `json:"policy_id"`
    PolicyName  string             `json:"policy_name"`
    Repeat      int                `json:"repeat"`
    Steps       []EscalationStepSnapshot `json:"steps"`
}

type EscalationStepSnapshot struct {
    StepNumber   int              `json:"step_number"`
    DelayMinutes int              `json:"delay_minutes"`
    Targets      []TargetSnapshot `json:"targets"`
}

type TargetSnapshot struct {
    Type string `json:"type"`  // user, schedule, team, webhook
    ID   string `json:"id"`
    Name string `json:"name"`  // for audit log readability
}
```

### 5.3 On-Call Schedule Engine

#### On-Call Calculation (Pure Computation)

On-call is NOT stored as state. It's calculated at query time from the rotation definition:

```
who_is_on_call(schedule_id, time T):
  1. Get all rotations for this schedule
  2. For each rotation:
     a. Check if T falls within the rotation's active period
     b. If yes:
        index = floor((T - rotation.start_time) / shift_duration) % len(participants)
        add participants[index]
  3. Apply overrides: for each active override at time T,
     replace/add/remove users as specified
  4. Return final set of on-call users
```

This approach (same as GoAlert) avoids stale state and makes the schedule system a pure function.

#### Rotation Types

| Type | Shift Duration | Example |
|------|---------------|---------|
| Daily | N days | 1-day shifts, handoff at 9am |
| Weekly | N weeks | 1-week shifts, handoff Monday 9am |
| Custom | N hours | 12-hour shifts for follow-the-sun |

---

## 6. Product 2: Uptime Monitoring

*Ships in v0.5. Schema and code do NOT exist until v0.5. This section documents the design for forward-compatibility.*

### 6.1 Data Model (Created at v0.5)

```sql
CREATE TABLE monitors (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,  -- http, tcp, dns, ssl, ping, heartbeat
    target          TEXT NOT NULL,  -- URL, host:port, domain, etc.
    interval_secs   INTEGER NOT NULL DEFAULT 60,
    timeout_secs    INTEGER NOT NULL DEFAULT 10,
    service_id      TEXT REFERENCES services(id),  -- THE KEY LINK: monitor → service → escalation policy
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    config          TEXT,  -- JSON: method, headers, expected_status, body_match, etc.
    consecutive_failures INTEGER NOT NULL DEFAULT 2,  -- flap detection threshold
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE monitor_checks (
    id          TEXT PRIMARY KEY,
    monitor_id  TEXT NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,  -- up, down, degraded
    latency_ms  INTEGER,
    status_code INTEGER,  -- HTTP status code (for HTTP monitors)
    error       TEXT,
    region      TEXT DEFAULT 'default',
    checked_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Heartbeat monitors** (v0.2) are a special case: they use the `monitors` table with `type = 'heartbeat'` but are created earlier because they don't need a monitor runner — they just track "last ping received" and alert if overdue. The `HeartbeatProcessor` in the engine handles this.

```sql
-- Heartbeat-specific table (v0.2, before full monitoring)
CREATE TABLE heartbeats (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    service_id      TEXT REFERENCES services(id),
    slug            TEXT UNIQUE NOT NULL,  -- URL path: /api/v1/heartbeat/{slug}
    interval_secs   INTEGER NOT NULL DEFAULT 300,  -- expected ping interval
    grace_secs      INTEGER NOT NULL DEFAULT 60,   -- grace period before alerting
    last_ping_at    TIMESTAMP,
    status          TEXT NOT NULL DEFAULT 'new',  -- new, up, down
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 6.2 Check Types

| Type | What it checks | Config | Ships in |
|------|---------------|--------|----------|
| HTTP | URL responds with expected status | method, url, expected_status, headers, body_match | v0.5 |
| TCP | Port is open and responsive | host, port | v0.6 |
| DNS | DNS record resolves correctly | domain, record_type, expected_value | v0.6 |
| SSL | Certificate is valid and not expiring | domain, warn_days_before_expiry | v0.6 |
| Ping | Host responds to ICMP | host | v0.6 |
| Heartbeat | Receives expected ping within timeout | interval, grace period | v0.2 |

### 6.3 Monitor Runner

```go
type MonitorRunner struct {
    store    Store
    checks   map[string]*time.Ticker  // monitor_id → ticker
    mu       sync.RWMutex
}

// On startup: load all enabled monitors, create tickers
// On monitor create/update: add/reset ticker
// On monitor delete: stop ticker
// Each tick: execute check, store result, evaluate state transition
```

### 6.4 Monitor → Alert Bridge (The Integration Seam)

When a monitor detects failure:

1. Monitor check returns `status = down`
2. Increment consecutive failure counter
3. If counter >= `consecutive_failures` threshold (flap detection):
   - Look up `monitor.service_id`
   - If service is set: create alert on that service with `source = 'monitor'`, `dedup_key = 'monitor:{monitor_id}'`
   - The alert triggers the service's escalation policy — **reusing all of Pillar 1's infrastructure**
4. When monitor recovers (consecutive successes):
   - Resolve the alert with matching dedup_key
   - If alert was grouped into incident → incident auto-resolves

**This is the key integration:** Pillar 2 creates data (alerts) that Pillar 1 already knows how to process. No new notification code needed.

### 6.5 Check Result Retention

- Last 1000 checks per monitor (configurable)
- Aggregated hourly/daily uptime percentages retained for 90 days
- Used for status page uptime display (Pillar 3)

---

## 7. Product 3: Status Pages

*Ships in v0.8. Schema and code do NOT exist until v0.8. This section documents the design for forward-compatibility.*

### 7.1 Data Model (Created at v0.8)

```sql
CREATE TABLE status_pages (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,  -- URL path: status.company.com or pagefire.dev/s/{slug}
    custom_domain   TEXT UNIQUE,
    branding        TEXT,  -- JSON: logo_url, accent_color, custom_css
    show_powered_by BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE status_page_components (
    id              TEXT PRIMARY KEY,
    status_page_id  TEXT NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    service_id      TEXT REFERENCES services(id),  -- THE KEY LINK: component → service → monitors
    position        INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE status_page_subscribers (
    id              TEXT PRIMARY KEY,
    status_page_id  TEXT NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    verified        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(status_page_id, email)
);

CREATE TABLE status_page_incidents (
    id              TEXT PRIMARY KEY,
    status_page_id  TEXT NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    incident_id     TEXT REFERENCES incidents(id),  -- THE KEY LINK: public incident → internal incident
    title           TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'investigating',
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at     TIMESTAMP
);

CREATE TABLE status_page_incident_updates (
    id                      TEXT PRIMARY KEY,
    status_page_incident_id TEXT NOT NULL REFERENCES status_page_incidents(id) ON DELETE CASCADE,
    status                  TEXT NOT NULL,
    message                 TEXT NOT NULL,
    created_at              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 7.2 Features

- **Components** — Map to services. Auto-update status from linked service's alert/incident state.
- **Incidents** — Public incident timeline with updates. Can be auto-created from internal incidents or manual.
- **Subscriber notifications** — Email notifications when incidents are created/updated/resolved. Uses the shared `notification_queue` with `type = 'subscriber'`.
- **Uptime graph** — 90-day uptime bar chart per component (from `monitor_checks` data via Pillar 2).
- **"Powered by Mayday"** — Badge on free tier. Removable on paid tier. **Virality loop.**
- **Custom branding** — Logo, accent color, custom CSS (paid tier).
- **Custom domain** — CNAME + auto-provisioned Let's Encrypt SSL (cloud paid tier).

### 7.3 Public Pages — Serving

Each status page is served at:
- Self-hosted: `http://localhost:3000/status/{slug}`
- Cloud: `https://{slug}.pagefire.dev` or custom domain with auto-SSL

Status pages are **read-only public HTML** — no auth required. They are server-rendered with their own minimal template set (no HTMX, no Alpine — pure HTML/CSS for fastest load and SEO).

### 7.4 Auto-Update Flow (The Integration Seam)

```
Internal incident status changes (Pillar 1)
  → StatusPageProcessor (engine tick)
    → Find status_page_incidents linked to this incident
      → Update status_page_incident status
        → Update component status based on linked service state
          → Queue subscriber notification emails
```

**What makes this work without rework:**
- Incidents already exist in v0.1 with `incident_services` linking
- Services already exist in v0.1
- The `StatusPageProcessor` just watches for incident status changes and propagates them
- Subscriber notifications use the same `notification_queue` and email provider from v0.1

### 7.5 Component Status Calculation

A component's status is derived from its linked service:

```
component_status(component):
  if component.service_id is NULL:
    return manual_status  -- admin sets it manually

  service = get_service(component.service_id)
  active_incidents = get_active_incidents(service.id)

  if any incident has severity = critical:
    return "major_outage"
  if any incident has severity = major:
    return "partial_outage"
  if any incident has severity = minor:
    return "degraded_performance"

  return "operational"
```

If Pillar 2 (Monitoring) is active, the monitor state also feeds in:

```
  if monitor is linked and monitor.status = down:
    return "major_outage"
  if monitor is linked and monitor.status = degraded:
    return "degraded_performance"
```

---

## 8. Integration Seams

This section documents exactly how the three pillars compose. These are the **contracts** between pillars — the interfaces that must remain stable.

### Seam 1: Monitor → Alert (Pillar 2 → Pillar 1)

**Direction**: Monitoring creates alerts that On-Call processes.

**Contract**: When a monitor detects failure, it creates an alert via the same `AlertStore.Create()` method used by integrations and manual alerts. The alert includes:
- `service_id` = monitor's linked service
- `source` = `"monitor"`
- `dedup_key` = `"monitor:{monitor_id}"` (prevents duplicate alerts for the same monitor)

**Why this works**: On-Call doesn't need to know about monitors. It processes all alerts the same way regardless of source.

### Seam 2: Alert/Incident → Status Page (Pillar 1 → Pillar 3)

**Direction**: Incident status changes trigger status page updates.

**Contract**: The `StatusPageProcessor` (engine module) watches for incident status changes and propagates them to linked status page incidents. The processor queries:
- `incidents` where `updated_at > last_processed_at`
- For each changed incident, finds `status_page_incidents` linked to it
- Updates component status and queues subscriber notifications

**Why this works**: The incident model exists from v0.1. Status pages just add a presentation layer on top.

### Seam 3: Monitor → Status Page (Pillar 2 → Pillar 3)

**Direction**: Monitor uptime data feeds status page uptime graphs.

**Contract**: Status page components link to services. Services link to monitors. The status page renderer queries `monitor_checks` for the linked monitor to build the 90-day uptime graph.

**Why this works**: The `service_id` foreign key on both `monitors` and `status_page_components` creates the join path without either pillar knowing about the other.

### Seam 4: Subscriber Notifications (Pillar 3 → Shared)

**Direction**: Status page events trigger notifications via the shared notification system.

**Contract**: Subscriber notifications are enqueued in `notification_queue` with `type = 'subscriber'`. The `NotificationProcessor` dispatches them using the email provider.

**Why this works**: Same queue, same dispatcher, same retry logic. The only difference is the destination (subscriber email vs user contact method) and the template.

---

## 9. API Design

### REST API (Chosen over GraphQL)

**Decision**: REST, not GraphQL.

Rationale:
- REST is simpler to implement and document
- REST is more accessible to the self-hosted audience (curl-friendly)
- HTMX-based UI doesn't benefit from GraphQL's batching
- We can always add GraphQL later; starting with REST is less lock-in

### API Endpoints by Pillar

#### Shared Platform (v0.1)

```
# Auth
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/me

# Users & Contact Methods
CRUD   /api/v1/users
CRUD   /api/v1/users/{id}/contact-methods
CRUD   /api/v1/users/{id}/notification-rules

# Services
CRUD   /api/v1/services

# Incidents
CRUD   /api/v1/incidents
POST   /api/v1/incidents/{id}/updates
```

#### Pillar 1: On-Call (v0.1)

```
# Alerts
POST   /api/v1/alerts                    # Create alert (manual)
GET    /api/v1/alerts                    # List alerts
GET    /api/v1/alerts/{id}               # Get alert
POST   /api/v1/alerts/{id}/acknowledge   # Ack alert
POST   /api/v1/alerts/{id}/resolve       # Resolve alert

# Inbound Integrations
POST   /api/v1/integrations/{key}/alerts     # Generic webhook
POST   /api/v1/integrations/{key}/grafana    # Grafana webhook format
POST   /api/v1/integrations/{key}/prometheus # Alertmanager format

# Integration Keys
CRUD   /api/v1/services/{id}/integration-keys

# Escalation Policies
CRUD   /api/v1/escalation-policies
CRUD   /api/v1/escalation-policies/{id}/steps

# Schedules
CRUD   /api/v1/schedules
CRUD   /api/v1/schedules/{id}/rotations
CRUD   /api/v1/schedules/{id}/overrides

# On-Call Resolution
GET    /api/v1/oncall/{schedule_id}      # Who's on call now
GET    /api/v1/oncall/{schedule_id}?at=  # Who's on call at time T
```

#### Pillar 1: Heartbeat (v0.2)

```
CRUD   /api/v1/heartbeats
POST   /api/v1/heartbeat/{slug}          # Receive heartbeat ping
```

#### Pillar 2: Monitoring (v0.5)

```
CRUD   /api/v1/monitors
GET    /api/v1/monitors/{id}/checks      # Check history
GET    /api/v1/monitors/{id}/uptime      # Uptime percentage
```

#### Pillar 3: Status Pages (v0.8)

```
CRUD   /api/v1/status-pages
CRUD   /api/v1/status-pages/{id}/components
CRUD   /api/v1/status-pages/{id}/incidents
POST   /api/v1/status-pages/{id}/incidents/{id}/updates
CRUD   /api/v1/status-pages/{id}/subscribers

# Public (no auth)
GET    /status/{slug}                    # Public status page HTML
GET    /api/v1/public/status/{slug}      # Public status page JSON (for embeds)
```

### Auth

- **API tokens** — Bearer tokens for programmatic access. Scoped to user or service account.
- **Session auth** — Cookie-based for the web UI.
- **OIDC/OAuth** — For cloud version SSO (v1.1).

---

## 10. Web UI

### Approach: Go Templates + HTMX (Not SPA)

**Decision**: Server-rendered HTML with HTMX for interactivity. Not React/Vue SPA.

Rationale:
- **Single binary simplicity**: No Node.js build step. Templates are Go code. `go:embed` just works.
- **Faster development for a solo founder**: No frontend/backend split, no state management library.
- **HTMX provides 90% of SPA interactivity**: Inline editing, real-time updates, form submissions without page reload.
- **Smaller binary**: No 2MB+ JavaScript bundle.

### UI Framework

- **Go `html/template`** — Template rendering
- **HTMX** — Dynamic UI without JavaScript
- **Tailwind CSS** — Styling (compiled at build time, embedded)
- **Alpine.js** — Minimal JS for dropdowns, modals, date pickers where HTMX isn't enough

### Pages by Pillar

#### v0.1 (On-Call)

- **Dashboard** — Overview: active alerts, on-call now, service status
- **Alerts** — List, filter, ack/resolve inline
- **Services** — CRUD, link to escalation policy
- **Escalation Policies** — Visual step editor
- **Schedules** — Calendar view, rotation management, override creation
- **Settings** — Users, integrations, notification channels

#### v0.5 (Monitoring) — Added

- **Monitors** — Check list, create/edit, uptime graphs
- **Dashboard** — + monitor status overview

#### v0.8 (Status Pages) — Added

- **Status Pages** — Component management, incident editor
- **Dashboard** — + status page overview

### Navigation Design

The sidebar/navigation should be **pillar-aware**: if monitoring and status pages aren't configured yet, those nav items should either be hidden or show an "enable" prompt. Users who only use on-call should never feel like they're using an incomplete product.

---

## 11. Storage Layer

### Dual-Database Design

Mayday must work with both SQLite and Postgres from the same codebase.

### Repository Pattern

```go
// store/store.go — grows per pillar
type Store interface {
    // Shared Platform (v0.1)
    Users() UserStore
    Notifications() NotificationStore
    Incidents() IncidentStore

    // Pillar 1: On-Call (v0.1)
    Alerts() AlertStore
    Services() ServiceStore
    EscalationPolicies() EscalationPolicyStore
    Schedules() ScheduleStore

    // Pillar 1: Heartbeat (v0.2)
    Heartbeats() HeartbeatStore

    // Pillar 2: Monitoring (v0.5)
    Monitors() MonitorStore

    // Pillar 3: Status Pages (v0.8)
    StatusPages() StatusPageStore
}

// Each sub-store is its own interface
type AlertStore interface {
    Create(ctx context.Context, alert *Alert) error
    Get(ctx context.Context, id string) (*Alert, error)
    List(ctx context.Context, filter AlertFilter) ([]Alert, error)
    Acknowledge(ctx context.Context, id string, userID string) error
    Resolve(ctx context.Context, id string, userID string) error
    FindPendingEscalations(ctx context.Context, before time.Time) ([]Alert, error)
}

// MonitorStore — doesn't exist until v0.5
type MonitorStore interface {
    Create(ctx context.Context, monitor *Monitor) error
    Get(ctx context.Context, id string) (*Monitor, error)
    List(ctx context.Context, filter MonitorFilter) ([]Monitor, error)
    RecordCheck(ctx context.Context, check *MonitorCheck) error
    GetUptimePercentage(ctx context.Context, monitorID string, since time.Time) (float64, error)
}
```

### Implementation Strategy

```
store/
  ├── store.go              # Interfaces (grows per pillar)
  ├── models.go             # Domain models (Go structs)
  ├── sqlite/
  │     ├── store.go        # SQLite implementation
  │     ├── alerts.go       # v0.1
  │     ├── schedules.go    # v0.1
  │     ├── monitors.go     # v0.5
  │     ├── status_pages.go # v0.8
  │     └── ...
  ├── postgres/
  │     ├── store.go
  │     ├── alerts.go
  │     └── ...
  └── migrations/
        ├── sqlite/
        │     ├── 0001_initial_platform.sql      # v0.1: users, services, incidents
        │     ├── 0002_oncall.sql                 # v0.1: schedules, escalation, alerts
        │     ├── 0003_heartbeats.sql             # v0.2
        │     ├── 0004_monitors.sql               # v0.5
        │     └── 0005_status_pages.sql           # v0.8
        └── postgres/
              ├── 0001_initial_platform.sql
              ├── 0002_oncall.sql
              ├── 0003_heartbeats.sql
              ├── 0004_monitors.sql
              └── 0005_status_pages.sql
```

### SQL Compatibility Rules

1. **Use standard SQL where possible** — `INSERT`, `UPDATE`, `SELECT`, `JOIN`, `WHERE`, `ORDER BY`, `LIMIT`
2. **Avoid Postgres-specific features in core queries** — No `FOR UPDATE SKIP LOCKED` (use application-level locking for SQLite), no `NOTIFY/LISTEN`, no `jsonb` operators
3. **Separate migration files** — Same logical schema, different DDL where syntax differs
4. **Feature flags for Postgres-only features** — Multi-instance engine coordination requires Postgres advisory locks. On SQLite, only one engine instance runs.

### SQLite Driver

**`modernc.org/sqlite`** (pure Go, no CGO). Rationale:
- Cross-compiles without CGO toolchain
- Well-maintained, production-tested (used by PocketBase, Litestream)
- WAL mode for concurrent reads
- Performance is sufficient for on-call scale

### Migration Strategy

**Forward-only migrations** using **goose** (pressly/goose). Migrations are embedded via `go:embed` and run on startup.

Each pillar's tables are added in their own migration file. This means:
- v0.1 binary runs migrations 0001-0002
- v0.2 binary runs migrations 0001-0003
- v0.5 binary runs migrations 0001-0004
- v0.8 binary runs migrations 0001-0005
- Upgrading from v0.2 → v0.5 automatically runs migration 0004

### SQLite → Postgres Migration Path

For users who start with SQLite and outgrow it:
1. `mayday export --format json > backup.json` — Full data export
2. Set `DATABASE_URL=postgres://...` in config
3. `mayday migrate` — Run Postgres migrations on new DB
4. `mayday import --format json < backup.json` — Import data

---

## 12. Multi-Tenancy & Cloud Monetization

### Design Philosophy

The self-hosted version is **single-tenant** (one organization). The cloud version is **multi-tenant** (many organizations on shared infrastructure). Same codebase, different configuration.

### Tenant Isolation Strategy

Row-level isolation with `organization_id` on every table.

```sql
-- Cloud-only: organization table
CREATE TABLE organizations (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    plan        TEXT NOT NULL DEFAULT 'free',  -- free, pro, business, enterprise
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Every table gets an org_id column in cloud mode
-- In self-hosted mode, org_id is a constant (single-org)
```

### How This Works in Code

```go
func (s *PostgresStore) ListAlerts(ctx context.Context, filter AlertFilter) ([]Alert, error) {
    orgID := auth.OrgIDFromContext(ctx)  // cloud: from JWT; self-hosted: constant
    query := `SELECT * FROM alerts WHERE organization_id = $1 AND ...`
    // ...
}
```

### Self-Hosted vs Cloud Feature Matrix

| Feature | Self-Hosted (Free) | Cloud Free | Cloud Pro ($19/mo) | Cloud Business ($49/mo) |
|---------|-------------------|------------|-------------------|----------------------|
| **On-Call** | | | | |
| Schedules | Unlimited | 1 | Unlimited | Unlimited |
| Escalation policies | Unlimited | 1 | Unlimited | Unlimited |
| Users | Unlimited | 2 | 10 | Unlimited |
| Notifications | Webhook, email, Slack | Same | + SMS/phone | + SMS/phone |
| **Monitoring** | | | | |
| Monitors | Unlimited | 5 | 50 | Unlimited |
| Check interval | 30s min | 60s min | 30s min | 30s min |
| Data retention | Unlimited | 30 days | 90 days | 1 year |
| **Status Pages** | | | | |
| Status pages | Unlimited | 1 (branded) | 3 (unbranded) | Unlimited |
| Custom domains | DIY | No | Yes | Yes |
| Subscribers | Unlimited | 100 | 1,000 | Unlimited |
| **Platform** | | | | |
| API access | Yes | Yes | Yes | Yes |
| SSO/SAML | No | No | No | Yes |

### What Lives in the Private Repo (Not OSS)

- Multi-tenant middleware (org isolation, JWT validation)
- Billing integration (Stripe)
- SSO/SAML provider
- Cloud-specific API rate limiting
- Managed Twilio pool (shared sender numbers)
- Custom domain SSL provisioning (Let's Encrypt automation)
- Cloud deployment infrastructure (Terraform, K8s manifests)

---

## 13. Migration Tool (Grafana OnCall Import)

*Ships in v0.2.*

### What We Import

From Grafana OnCall's public API (`/api/v1/`):

| Grafana OnCall Entity | Mayday Entity | Notes |
|----------------------|---------------|-------|
| Escalation Chain | Escalation Policy | Name mapping |
| Escalation Policy (steps) | Escalation Steps | Map 21 step types to our simpler model |
| On-Call Schedule | Schedule + Rotations | Flatten polymorphic model (iCal/Calendar/Web → rotations) |
| Integration | Service + Integration Key | Map integration types |
| User | User | Email-based matching |
| Notification Rules | Notification Rules | Map channel types |

### Grafana OnCall Step Type Mapping

Grafana OnCall has 21 step types. We map the commonly used ones:

| Grafana OnCall Step | Mayday Equivalent |
|--------------------|-------------------|
| `wait` | Wait step (delay_minutes) |
| `notify_persons` | Target → User(s) |
| `notify_on_call_from_schedule` | Target → Schedule |
| `notify_team_members` | Target → Team |
| `notify_person_next_each_time` | Target → User (round-robin — future feature, warn) |
| `repeat_escalation` | Policy repeat count |
| `resolve` | Auto-resolve action |
| `trigger_webhook` | Target → Webhook |
| `notify_user_group` | Target → Slack channel (if Slack configured) |
| `notify_if_time_from_to` | Warn: time-based rules not supported in v0.2 |
| Other Slack-specific steps | Skipped with warning |

### CLI Interface

```bash
# Export from Grafana OnCall (uses their API)
mayday migrate from-grafana-oncall \
    --url https://oncall.example.com \
    --token <grafana-api-token> \
    --output export.json

# Preview what will be imported
mayday migrate preview export.json

# Import into Mayday
mayday migrate import export.json
```

---

## 14. Project Structure

The project structure reflects the pillar-based architecture. Packages for future pillars are NOT created until their release — no empty directories.

### v0.1 Structure (What We Scaffold Now)

```
mayday/
├── cmd/
│   └── mayday/
│       └── main.go              # CLI entrypoint (cobra)
│
├── internal/
│   ├── app/
│   │   ├── app.go               # Application wiring, lifecycle, graceful shutdown
│   │   └── config.go            # Configuration (koanf: env vars, flags, optional file)
│   │
│   ├── engine/
│   │   ├── engine.go            # Main tick loop + Processor interface
│   │   ├── escalation.go        # EscalationProcessor
│   │   ├── notification.go      # NotificationProcessor
│   │   └── cleanup.go           # CleanupProcessor
│   │
│   ├── api/
│   │   ├── router.go            # HTTP router setup
│   │   ├── middleware/           # Auth, logging, rate limiting
│   │   ├── alerts.go
│   │   ├── services.go
│   │   ├── schedules.go
│   │   ├── escalation.go
│   │   ├── integrations.go      # Inbound integration handlers
│   │   ├── oncall.go
│   │   ├── users.go
│   │   └── incidents.go
│   │
│   ├── web/
│   │   ├── handler.go           # Static file serving, template rendering
│   │   ├── templates/
│   │   │   ├── layouts/
│   │   │   ├── pages/
│   │   │   └── partials/        # HTMX partials
│   │   └── static/              # CSS, JS (Tailwind, HTMX, Alpine)
│   │
│   ├── store/
│   │   ├── store.go             # Store interfaces
│   │   ├── models.go            # Domain models (Go structs)
│   │   ├── sqlite/
│   │   │   ├── store.go
│   │   │   ├── alerts.go
│   │   │   ├── schedules.go
│   │   │   ├── services.go
│   │   │   ├── escalation.go
│   │   │   ├── users.go
│   │   │   └── notifications.go
│   │   ├── postgres/
│   │   │   └── ... (mirrors sqlite/)
│   │   └── migrations/
│   │       ├── sqlite/
│   │       │   ├── 0001_initial_platform.sql
│   │       │   └── 0002_oncall.sql
│   │       └── postgres/
│   │           ├── 0001_initial_platform.sql
│   │           └── 0002_oncall.sql
│   │
│   ├── oncall/
│   │   └── resolver.go          # On-call calculation (pure functions)
│   │
│   └── notification/
│       ├── dispatcher.go        # Central dispatch
│       └── providers/
│             ├── webhook.go
│             ├── email.go
│             └── slack.go
│
├── web/                         # Frontend source (Tailwind build)
│   ├── tailwind.config.js
│   └── input.css
│
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── LICENSE                      # MIT
├── README.md
└── ARCHITECTURE.md              # This file
```

### Added at v0.2

```
├── internal/
│   ├── engine/
│   │   └── heartbeat.go         # HeartbeatProcessor
│   ├── api/
│   │   └── heartbeats.go
│   ├── store/
│   │   ├── sqlite/
│   │   │   └── heartbeats.go
│   │   └── migrations/
│   │       ├── sqlite/0003_heartbeats.sql
│   │       └── postgres/0003_heartbeats.sql
│   └── migrate/
│       ├── runner.go            # Migration runner
│       └── grafana_oncall.go    # Grafana OnCall importer
```

### Added at v0.5

```
├── internal/
│   ├── engine/
│   │   └── monitor_state.go     # MonitorStateProcessor
│   ├── api/
│   │   └── monitors.go
│   ├── monitor/
│   │   ├── runner.go            # Check scheduler
│   │   └── checkers/
│   │         ├── http.go
│   │         ├── tcp.go
│   │         ├── dns.go
│   │         └── ssl.go
│   └── store/
│       ├── sqlite/
│       │   └── monitors.go
│       └── migrations/
│           ├── sqlite/0004_monitors.sql
│           └── postgres/0004_monitors.sql
```

### Added at v0.8

```
├── internal/
│   ├── engine/
│   │   └── status_page.go       # StatusPageProcessor
│   ├── api/
│   │   └── status_pages.go
│   ├── statuspage/
│   │   ├── renderer.go          # Public page rendering (separate templates)
│   │   └── templates/           # Minimal HTML/CSS templates (no HTMX)
│   └── store/
│       ├── sqlite/
│       │   └── status_pages.go
│       └── migrations/
│           ├── sqlite/0005_status_pages.sql
│           └── postgres/0005_status_pages.sql
```

---

## 15. Build & Packaging

### Build Pipeline

```makefile
# Development
make dev          # Run with live reload (air)
make test         # Run tests
make lint         # golangci-lint

# Production
make build        # Build binary with embedded assets
make docker       # Build Docker image

# Release
make release      # Cross-compile for linux-amd64/arm64, darwin-amd64/arm64
```

### Embedding

```go
// internal/web/handler.go
//go:embed templates static
var embeddedFS embed.FS

// internal/store/migrations/
//go:embed sqlite/*.sql
var sqliteMigrationsFS embed.FS

//go:embed postgres/*.sql
var postgresMigrationsFS embed.FS
```

### Docker

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:3.19
COPY --from=builder /app/bin/mayday /usr/local/bin/mayday
EXPOSE 3000
ENTRYPOINT ["mayday"]
CMD ["serve"]
```

### One-Line Install

```bash
curl -fsSL https://get.pagefire.dev | sh
# Downloads the right binary for your OS/arch, moves to /usr/local/bin
```

---

## 16. Extensibility

### Notification Provider Plugin Model

Notification providers implement a Go interface and are compiled into the binary. No runtime plugins (keeps the single-binary promise).

Adding a new provider:
1. Create `internal/notification/providers/myprovider.go`
2. Implement `Provider` interface
3. Register in `internal/notification/registry.go`
4. Add migration for new contact method type
5. Rebuild

### Integration Key Types

Each inbound integration type implements an interface:

```go
type IntegrationHandler interface {
    Type() string
    ParseWebhook(r *http.Request) (*Alert, error)
}
```

Adding Datadog, New Relic, or other alert sources is a single file implementing this interface.

### Engine Processor Plugin Model

Each engine processor implements the `Processor` interface:

```go
type Processor interface {
    Name() string
    Tick(ctx context.Context) error
}
```

New pillars register new processors. The engine doesn't need to know what they do.

### Future Extension Points

- **Webhook actions** — Custom webhooks triggered on alert lifecycle events (created, acked, resolved)
- **Alert rules** — Simple rules engine for auto-ack, auto-resolve, severity mapping
- **Remote agents** — Lightweight monitoring agents that report to the central Mayday instance (for multi-region checks)
- **Terraform provider** — CRUD all entities via Terraform (v1.2)

---

## 17. Key Architectural Decisions

### ADR-1: REST over GraphQL

**Context**: GoAlert uses GraphQL successfully.
**Decision**: Start with REST.
**Rationale**: REST is simpler, curl-friendly, better for the self-hosted audience. HTMX-based UI doesn't benefit from GraphQL. We can add GraphQL later if demand warrants it.

### ADR-2: Go Templates + HTMX over React SPA

**Context**: GoAlert uses React with MUI.
**Decision**: Server-rendered templates with HTMX.
**Rationale**: Eliminates the frontend build step, reduces binary size, faster development for a solo founder. HTMX handles 90% of interactivity needs.

### ADR-3: Polling Engine over Event-Driven

**Context**: Could use channels/events for alert processing.
**Decision**: 5-second tick polling loop (same as GoAlert).
**Rationale**: Simpler, debuggable, restart-safe. Escalation delays are measured in minutes. A 5-second poll cycle is more than adequate.

### ADR-4: Database-as-Queue over External Message Broker

**Context**: Could use Redis, NATS, or RabbitMQ for notification dispatch.
**Decision**: Queue notifications in a database table.
**Rationale**: No infrastructure dependency. Survives restarts. GoAlert proves this pattern works. Twilio rate limits (~1 msg/sec) are far below any DB bottleneck.

### ADR-5: Pure Go SQLite (modernc) over CGO SQLite

**Context**: `mattn/go-sqlite3` is more mature but requires CGO.
**Decision**: Use `modernc.org/sqlite` (pure Go).
**Rationale**: Cross-compilation without CGO toolchain is essential for the "download and run" experience.

### ADR-6: Forward-Only Migrations

**Context**: Could support up/down migrations.
**Decision**: Forward-only (no rollback migrations).
**Rationale**: Down migrations are rarely tested and often buggy. GoAlert has 250+ migrations with this approach over 7+ years.

### ADR-7: Row-Level Multi-Tenancy over Schema-per-Tenant

**Context**: Need multi-tenancy for cloud version.
**Decision**: `organization_id` column on all tables with row-level filtering.
**Rationale**: Simpler schema management, single migration path. Adequate for thousands of tenants at on-call data volumes.

### ADR-8: On-Call Calculated at Query Time

**Context**: Could store "current on-call user" in a materialized view.
**Decision**: Calculate on-call from rotation definition at query time (same as GoAlert).
**Rationale**: No stale state. Schedule changes take effect immediately. The calculation is pure math.

### ADR-9: Escalation Policy Snapshot on Alert Creation

**Context**: Should alerts reference the live escalation policy or a frozen copy?
**Decision**: Snapshot the full escalation policy as JSON on the alert at creation time.
**Rationale**: Editing an escalation policy must not affect in-flight escalations. Learned from Grafana OnCall, which uses `raw_escalation_snapshot` for exactly this reason.

### ADR-10: Koanf over Viper for Configuration

**Context**: Viper is the most common Go config library.
**Decision**: Use knadh/koanf.
**Rationale**: 313% smaller binary, no global state, cleaner API. Viper v2 has been discussed since 2020 but remains unreleased. Hierarchy: CLI flags > env vars > config file > defaults.

### ADR-11: Pillar-Based Architecture with Services as Linchpin

**Context**: Could design each feature independently and integrate later.
**Decision**: Design all three pillars upfront but ship sequentially. Services are the central entity that all pillars connect to.
**Rationale**: Avoids rework when adding monitoring and status pages. The `service_id` FK on monitors, escalation policies, and status page components creates natural integration points without coupling pillar code.

---

## Appendix: Technology Stack Summary

| Component | Choice | Why |
|-----------|--------|-----|
| Language | Go 1.23+ | Single binary, great concurrency, DevOps ecosystem |
| HTTP Router | chi or stdlib | Lightweight, middleware-friendly |
| CLI | cobra | Standard Go CLI framework |
| Configuration | knadh/koanf | Lighter than Viper, no global state |
| Templates | html/template + HTMX | No frontend build step |
| CSS | Tailwind CSS | Utility-first, embedded at build time |
| SQLite Driver | modernc.org/sqlite | Pure Go, no CGO |
| Postgres Driver | pgx/v5 | Best Go Postgres driver |
| Migrations | pressly/goose | SQLite + Postgres, embed-friendly |
| SQL Codegen | sqlc (optional) | Type-safe SQL |
| Email | net/smtp + gomail | Built-in, no dependency |
| Slack | slack-go/slack | Official SDK |
| Twilio | twilio/twilio-go | Official SDK |
| Testing | stdlib + testify | Keep it simple |
| Linting | golangci-lint | Standard Go linter |
| Live Reload | air | Dev experience |

---

*This is a living document. Updated as architectural decisions evolve.*
