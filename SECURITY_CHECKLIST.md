# Pre-Commit Security Checklist

Every PR must pass this checklist before merge. Copy into your PR description and check each item.

## Input Validation
- [ ] All user-supplied strings are length-bounded (`MaxBytesReader` on request body)
- [ ] Integer parameters validated: positive, within reasonable bounds, error-checked
- [ ] Email addresses validated via `net/mail.ParseAddress()`
- [ ] URLs validated: scheme check (http/https), no private IPs (SSRF)
- [ ] Timezone strings validated via `time.LoadLocation()`
- [ ] Role values validated against whitelist (`admin`, `user`, `viewer`)
- [ ] No raw user input interpolated into SQL, SMTP headers, or log messages

## Authentication & Authorization
- [ ] New endpoints have auth middleware (or documented exception with justification)
- [ ] RBAC enforced: users can only access their own org's data
- [ ] Users can only modify their own contact methods / notification rules
- [ ] Integration key endpoints are write-only (alert creation only)
- [ ] No privilege escalation: can a regular user call this endpoint to gain admin access?

## Secrets & Sensitive Data
- [ ] Secrets never returned in API responses (except once on creation)
- [ ] Secrets never logged (check `slog.Info/Error` calls)
- [ ] Secrets never stored in plaintext if hash is sufficient
- [ ] Error messages don't leak internal details (DB schema, file paths, stack traces)
- [ ] No hardcoded credentials, tokens, or keys in source code

## External Interactions
- [ ] Outbound HTTP requests validate destination IP (no SSRF)
- [ ] Outbound webhooks are signed with HMAC-SHA256
- [ ] Inbound webhooks verify signatures where supported
- [ ] No self-referencing webhook loops possible
- [ ] External API credentials loaded from env vars, not config files

## Data & Query Safety
- [ ] All SQL uses parameterized queries (no string concatenation)
- [ ] List queries have max `LIMIT` enforced (default 50, max 1000)
- [ ] Multi-tenant queries include `organization_id` filter (cloud mode)
- [ ] Database constraints match application-level validation (defense in depth)
- [ ] Mutations create audit log entries

## Denial of Service
- [ ] Request body size is bounded (`MaxBytesReader`)
- [ ] Rate limiting is in place for the affected endpoint
- [ ] No unbounded loops, goroutine spawning, or memory allocation from user input
- [ ] Notification queue depth won't explode from this change
