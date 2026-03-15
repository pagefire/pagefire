# Changelog

All notable changes to PageFire are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.3.0] - 2026-03-14

### Added

- **Twilio SMS and phone call notification providers.** Alerts can now notify on-call responders via SMS (truncated to 1600 chars) and voice calls (TwiML read-aloud). Providers activate automatically when `PAGEFIRE_TWILIO_*` env vars are set. E.164 phone number validation enforced in both API and frontend.
- **Incident-to-alert linking.** New `incident_alerts` table with API endpoints and UI on both incident detail and alert detail pages, allowing responders to associate related alerts with an incident.
- **"Send Test Alert" button on integration keys.** Fires a real test alert through the full pipeline (routing, escalation, notification) from the service detail page.
- **Alert filtering by source and date range.** New `source`, `created_after`, and `created_before` query parameters on the alerts list endpoint, with date picker and source dropdown in the frontend.
- **Cleanup processor.** Runs hourly to auto-purge resolved alerts older than 90 days, sent/failed notifications older than 30 days, and expired schedule overrides.
- **Health endpoint with DB connectivity check.** `GET /healthz` now verifies database connectivity and returns HTTP 503 if degraded.
- **Web frontend and auth system.** Full Preact + Vite SPA with embedded serving. Cookie-based session authentication (login/logout, argon2id passwords), user invite flow (token-based, 7-day expiry, single-use), and RBAC guards across all pages.
- **Teams support.** Team creation, membership management, and `?team_id=` scoped filtering on services, escalation policies, and schedules.
- **Content-based alert routing rules.** Route alerts to different escalation policies based on summary, details, or source fields using contains (case-insensitive) or regex matching. Rules evaluated in priority order with fallback to default policy.
- **Alert grouping.** Alerts sharing a `group_key` on a service are grouped together -- only the first triggers escalation, subsequent alerts are created but suppressed.
- **Docker Compose setup** for self-hosting (`docker-compose.yml`).
- **OpenAPI 3.0 spec** (`docs/openapi.yaml`).
- **Documentation:** Twilio setup guide (`docs/twilio-setup.md`), TLS/reverse proxy guide (`docs/tls-reverse-proxy.md`), Docker deployment guide (`docs/docker.md`), Grafana OnCall migration guide (`docs/migrating-from-grafana-oncall.md`).
- **CI/CD pipeline** with GitHub Actions, Dockerfile with multi-stage Node + Go build, and release workflow for Docker images and binary artifacts.
- **Legacy admin token fully deprecated.** All API access now requires per-user tokens or session cookies.

### Security

- **CSRF protection** via `Content-Type: application/json` validation on all JSON endpoints (requests without the correct content type are rejected).
- **Password complexity requirements:** minimum 8 characters, at least one uppercase letter, one lowercase letter, and one digit.
- **Session idle timeout** of 2 hours (alongside the existing 24-hour absolute session expiry).
- **SQLite `busy_timeout` applied via DSN** to all pool connections, fixing `SQLITE_BUSY` errors under concurrent load (previously set via PRAGMA after open, which only applied to one connection).
- **Cookie `Secure` flag** set on session cookies.
- **Input length limits** enforced across all endpoints (alerts, incidents, services, integrations, routing rules).
- **ReDoS protection:** regex patterns validated at routing rule creation time with a 1024-character limit.
- **Rate limiter hardening:** removed trust of `X-Real-IP` header; added cleanup to prevent unbounded memory growth.
- **SSRF protection** on outbound webhooks: blocks private/loopback IP ranges, with `AllowPrivateWebhooks` flag for local development.

### Fixed

- **SQLite timezone bug.** All `time.Time` values normalized to UTC at the store boundary, fixing silent query failures when comparing UTC-stored values against local-time parameters.
- **Escalation exhaustion loop.** `next_escalation_at` cleared to NULL when all steps are exhausted, preventing the engine from repeatedly picking up fully-escalated alerts.
- **Integration key deduplication.** Duplicate alerts now return HTTP 200 with the existing alert instead of an error.
- **Config env var mapping.** Only dot-maps known nested prefixes (`smtp_`, `slack_`, `engine_`), preserving underscores in flat keys like `admin_token`.
- **Alert acknowledge/resolve race conditions.** Operations made idempotent so concurrent clicks do not produce errors.
- **CI build order.** Node.js setup and `npm ci` now run before `vite build` in both CI and release workflows.

### Testing

- **Integration test:** full alert-to-escalation-to-notification flow (`TestSmoke_FullAlertFlow`).
- **Negative/edge-case tests:** expired sessions (`TestSessionOrTokenAuth_ExpiredSessionCookie`), revoked API tokens (`TestSessionOrTokenAuth_RevokedAPIToken`), deleted users during escalation (`TestEscalationProcessor_DeletedUserTarget`).
- **Load/stress tests:** 1000 concurrent alert creations (`TestStress_ConcurrentAlertCreation`), concurrent acknowledge/resolve under contention (`TestStress_ConcurrentAckResolve`), list queries under write load (`TestStress_ListUnderLoad`) -- all exercising WAL mode and `busy_timeout`.
- 152+ unit and API tests across store, API, engine, and provider layers.

## [v0.2.0] - 2026-03-06

_Initial public release with core on-call management: services, integration keys, alerts, incidents, escalation policies, schedules with rotations and overrides, webhook and email notifications._

## [v0.1.0-alpha] - 2026-02-28

_Initial scaffold with STRIDE-hardened API layer._

[v0.3.0]: https://github.com/pagefire/pagefire/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/pagefire/pagefire/compare/v0.1.0-alpha...v0.2.0
[v0.1.0-alpha]: https://github.com/pagefire/pagefire/releases/tag/v0.1.0-alpha
