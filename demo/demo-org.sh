#!/usr/bin/env bash
set -euo pipefail

# --- PageFire Org Demo -----------------------------------------------------
# Simulates a real engineering org using PageFire.
#
# Org: "XYZ Corp"
#   Team:
#     - User AAA  (Senior SRE, primary on-call)
#     - User BBB  (Backend Engineer, secondary on-call)
#     - User CCC  (Engineering Manager, last resort)
#
#   Services:
#     - API Server      (port 8080)
#     - Payment Service  (port 8081)
#
#   On-Call Schedule:
#     - Weekly rotation: AAA <-> BBB (AAA is current)
#
#   Escalation Policy: "Production Critical"
#     Step 0 -> Primary On-Call schedule (AAA)  [immediate]
#     Step 1 -> BBB (secondary)                 [after step delay]
#     Step 2 -> CCC (manager)                   [after step delay]
#     Repeat: 1
#
#   Scenarios:
#     1. API Server goes down -> pages escalate through the chain
#     2. On-call acknowledges -> paging stops
#     3. Service recovers -> alert auto-resolves
#     4. Incident declared across services
#     5. Schedule override: AAA goes on vacation, BBB covers
#     6. Alert deduplication
# ---------------------------------------------------------------------------

PAGEFIRE_PORT=3001
API_PORT=8080
PAYMENT_PORT=8081
NOTIFY_PORT=9090
API="http://localhost:${PAGEFIRE_PORT}/api/v1"
PIDS=()
COOKIE_JAR="/tmp/pagefire-demo-cookies.txt"

cleanup() {
  echo ""
  echo "Cleaning up..."
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  for port in ${PAGEFIRE_PORT} ${API_PORT} ${PAYMENT_PORT} ${NOTIFY_PORT}; do
    kill $(lsof -ti:${port}) 2>/dev/null || true
  done
  rm -f /tmp/pagefire-demo.db "${COOKIE_JAR}"
  echo "Done. Logs preserved at /tmp/pagefire-demo.log"
}
trap cleanup EXIT

# --- Helpers ---------------------------------------------------------------
log()     { echo "[==>] $1"; }
ok()      { echo " [ok] $1"; }
fire()    { echo " [!!] $1"; }
narrate() { echo "  > $1"; }
heading() {
  echo ""
  echo "==========================================================="
  echo "  $1"
  echo "==========================================================="
  echo ""
}
pause()   { echo ""; echo "  (waiting ${1}s...)"; sleep "$1"; }
jq_id()   { python3 -c "import sys,json; print(json.load(sys.stdin)['id'])"; }
jq_secret() { python3 -c "import sys,json; print(json.load(sys.stdin)['secret'])"; }
jq_field() { python3 -c "import sys,json; print(json.load(sys.stdin).get('$1',''))"; }

# --- Clean stale state ------------------------------------------------------
rm -f /tmp/pagefire-demo.db /tmp/pagefire-demo.log "${COOKIE_JAR}"
for port in ${PAGEFIRE_PORT} ${API_PORT} ${PAYMENT_PORT} ${NOTIFY_PORT}; do
  kill $(lsof -ti:${port}) 2>/dev/null || true
done

# --- Check prerequisites ----------------------------------------------------
if ! command -v go &>/dev/null; then
  echo "Error: Go is required. Install from https://go.dev"
  exit 1
fi

# ============================================================================
heading "XYZ CORP -- PageFire Org Demo"
# ============================================================================

narrate "XYZ Corp has 3 engineers and 2 production services."
narrate "Setting up their on-call infrastructure with PageFire."
echo ""

# --- Build PageFire ----------------------------------------------------------
log "Building PageFire..."
cd "$(dirname "$0")/.."
make build 2>/dev/null
ok "Built ./bin/pagefire"

# --- Start notification receiver ---------------------------------------------
log "Starting notification receiver on :${NOTIFY_PORT}..."
python3 -u -c "
import json, http.server, sys, datetime

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        payload = json.loads(body) if body else {}
        subject = payload.get('subject', '')
        alert_body = payload.get('body', '')
        user_name = payload.get('user_name', 'unknown')
        now = datetime.datetime.now().strftime('%H:%M:%S')
        print(flush=True)
        print('--- PAGE RECEIVED ---', flush=True)
        print(f'  To:      {user_name}', flush=True)
        print(f'  Time:    {now}', flush=True)
        print(f'  Subject: {subject}', flush=True)
        print(f'  Body:    {alert_body}', flush=True)
        print('---------------------', flush=True)
        print(flush=True)
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{\"status\":\"received\"}\n')
    def log_message(self, *args): pass

print('Notification receiver listening on :${NOTIFY_PORT}')
http.server.HTTPServer(('', ${NOTIFY_PORT}), Handler).serve_forever()
" &
PIDS+=($!)
sleep 1
ok "Notification receiver ready"

# --- Start PageFire ----------------------------------------------------------
log "Starting PageFire on :${PAGEFIRE_PORT}..."
PAGEFIRE_PORT="${PAGEFIRE_PORT}" \
PAGEFIRE_DATABASE_URL="/tmp/pagefire-demo.db" \
PAGEFIRE_LOG_LEVEL="info" \
PAGEFIRE_ENGINE_INTERVAL_SECONDS=3 \
PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS=true \
./bin/pagefire serve 2>/tmp/pagefire-demo.log &
PIDS+=($!)
sleep 2

if ! curl -sf "http://localhost:${PAGEFIRE_PORT}/healthz" >/dev/null; then
  echo "Error: PageFire failed to start"
  exit 1
fi
ok "PageFire is running"

# --- Start fake services -----------------------------------------------------
log "Starting API Server on :${API_PORT}..."
go run -C demo myapp.go -port ${API_PORT} &
PIDS+=($!)
sleep 1
ok "API Server is running"

log "Starting Payment Service on :${PAYMENT_PORT}..."
go run -C demo myapp.go -port ${PAYMENT_PORT} &
PIDS+=($!)
sleep 1
ok "Payment Service is running"

# ============================================================================
heading "PHASE 1: Org Setup"
# ============================================================================

narrate "Creating the XYZ Corp team..."
echo ""

# --- Create admin user via setup endpoint (no auth required) -----------------
AAA_ID=$(curl -sf -X POST "${API}/auth/setup" \
  -c "${COOKIE_JAR}" \
  -H "Content-Type: application/json" \
  -d '{"name":"User AAA","email":"aaa@xyz.dev","password":"demo-password-123"}' | jq_id)
ok "Created User AAA -- Senior SRE (admin, via setup)"

# Generate an API token using the session cookie from setup
API_TOKEN=$(curl -sf -X POST "${API}/auth/tokens" \
  -b "${COOKIE_JAR}" \
  -H "Content-Type: application/json" \
  -d '{"name":"demo-script"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
ok "Generated API token for scripting"

# --- Create remaining users --------------------------------------------------
BBB_ID=$(curl -sf -X POST "${API}/users" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"User BBB","email":"bbb@xyz.dev","timezone":"America/New_York"}' | jq_id)
ok "Created User BBB -- Backend Engineer"

CCC_ID=$(curl -sf -X POST "${API}/users" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"User CCC","email":"ccc@xyz.dev","timezone":"America/Chicago"}' | jq_id)
ok "Created User CCC -- Engineering Manager"

# --- Contact methods & notification rules ------------------------------------
narrate "Setting up contact methods (all webhook for demo)..."

for user_tuple in "${AAA_ID}:AAA" "${BBB_ID}:BBB" "${CCC_ID}:CCC"; do
  uid="${user_tuple%%:*}"
  uname="${user_tuple##*:}"

  CM_ID=$(curl -sf -X POST "${API}/users/${uid}/contact-methods" \
    -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
    -d "{\"type\":\"webhook\",\"value\":\"http://localhost:${NOTIFY_PORT}/notify\"}" | jq_id)

  curl -sf -X POST "${API}/users/${uid}/notification-rules" \
    -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
    -d "{\"contact_method_id\":\"${CM_ID}\",\"delay_minutes\":0}" >/dev/null

  ok "${uname}: webhook + immediate notification rule"
done

# --- On-call schedule --------------------------------------------------------
echo ""
narrate "Creating on-call schedule with weekly rotation..."

SCHEDULE_ID=$(curl -sf -X POST "${API}/schedules" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Primary On-Call","timezone":"America/Los_Angeles"}' | jq_id)
ok "Created schedule: Primary On-Call"

NOW_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
ROTATION_ID=$(curl -sf -X POST "${API}/schedules/${SCHEDULE_ID}/rotations" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"name\":\"Weekly Rotation\",\"type\":\"weekly\",\"shift_length\":1,\"start_time\":\"${NOW_ISO}\"}" | jq_id)
ok "Created rotation: weekly, starting now"

curl -sf -X POST "${API}/schedules/${SCHEDULE_ID}/rotations/${ROTATION_ID}/participants" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"${AAA_ID}\",\"position\":0}" >/dev/null
ok "Added AAA (position 0 -- currently on-call)"

curl -sf -X POST "${API}/schedules/${SCHEDULE_ID}/rotations/${ROTATION_ID}/participants" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"${BBB_ID}\",\"position\":1}" >/dev/null
ok "Added BBB (position 1 -- on-call next week)"

ONCALL_NAME=$(curl -sf "${API}/oncall/${SCHEDULE_ID}" \
  -H "Authorization: Bearer ${API_TOKEN}" | python3 -c "import sys,json; users=json.load(sys.stdin); print(users[0]['name'] if users else 'nobody')")
ok "Currently on-call: ${ONCALL_NAME}"

# --- Escalation policy -------------------------------------------------------
echo ""
narrate "Creating multi-step escalation policy..."

EP_ID=$(curl -sf -X POST "${API}/escalation-policies" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Production Critical","repeat":1}' | jq_id)
ok "Created policy: Production Critical (repeat 1x)"

STEP0_ID=$(curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"step_number":0,"delay_minutes":0}' | jq_id)
curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps/${STEP0_ID}/targets" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"target_type\":\"schedule\",\"target_id\":\"${SCHEDULE_ID}\"}" >/dev/null
ok "Step 0: Page on-call schedule (AAA) -- immediate"

STEP1_ID=$(curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"step_number":1,"delay_minutes":0}' | jq_id)
curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps/${STEP1_ID}/targets" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"target_type\":\"user\",\"target_id\":\"${BBB_ID}\"}" >/dev/null
ok "Step 1: Page BBB (secondary) -- after step 0 delay"

STEP2_ID=$(curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"step_number":2,"delay_minutes":0}' | jq_id)
curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps/${STEP2_ID}/targets" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"target_type\":\"user\",\"target_id\":\"${CCC_ID}\"}" >/dev/null
ok "Step 2: Page CCC (manager) -- last resort"

# --- Services & integration keys ---------------------------------------------
echo ""
narrate "Creating services..."

API_SVC_ID=$(curl -sf -X POST "${API}/services" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"name\":\"API Server\",\"escalation_policy_id\":\"${EP_ID}\",\"description\":\"Core REST API\"}" | jq_id)
API_IK=$(curl -sf -X POST "${API}/services/${API_SVC_ID}/integration-keys" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Health Checker"}' | jq_secret)
ok "API Server -- integration key ready"

PAY_SVC_ID=$(curl -sf -X POST "${API}/services" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"name\":\"Payment Service\",\"escalation_policy_id\":\"${EP_ID}\",\"description\":\"Payment processing\"}" | jq_id)
PAY_IK=$(curl -sf -X POST "${API}/services/${PAY_SVC_ID}/integration-keys" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Health Checker"}' | jq_secret)
ok "Payment Service -- integration key ready"

echo ""
narrate "Org setup complete!"
echo ""
echo "  Team:       AAA (on-call) -> BBB (secondary) -> CCC (manager)"
echo "  Schedule:   Primary On-Call -- AAA this week, BBB next week"
echo "  Escalation: On-Call -> BBB -> CCC (repeat 1x)"
echo "  Services:   API Server (:${API_PORT}), Payment Service (:${PAYMENT_PORT})"

# ============================================================================
heading "PHASE 2: Alert Escalation Chain"
# ============================================================================

narrate "Simulating API Server outage..."
narrate "Killing API Server process..."
echo ""

kill $(lsof -ti:${API_PORT}) 2>/dev/null || true
sleep 2

fire "API Server is DOWN!"
echo ""

narrate "Firing alert via integration webhook..."
ALERT_RESPONSE=$(curl -sf -X POST \
  "http://localhost:${PAGEFIRE_PORT}/api/v1/integrations/${API_IK}/alerts" \
  -H "Content-Type: application/json" \
  -d '{"summary":"API Server health check failed","details":"GET /health returned connection refused","dedup_key":"api-health"}')
ALERT_ID=$(echo "$ALERT_RESPONSE" | jq_id)
ok "Alert created: ${ALERT_ID}"

narrate "Waiting for escalation chain to fire (watch the pages arrive)..."
narrate "Step 0 -> AAA (on-call), Step 1 -> BBB, Step 2 -> CCC"
echo ""

sleep 15

# ============================================================================
heading "PHASE 3: Acknowledge -- Stop the Paging"
# ============================================================================

narrate "AAA wakes up and acknowledges the alert..."
echo ""

curl -sf -X POST "${API}/alerts/${ALERT_ID}/acknowledge" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"${AAA_ID}\"}" >/dev/null
ok "AAA acknowledged the alert -- paging stopped"

ALERT_STATUS=$(curl -sf "${API}/alerts/${ALERT_ID}" \
  -H "Authorization: Bearer ${API_TOKEN}" | jq_field status)
narrate "Alert status: ${ALERT_STATUS}"

narrate "Checking alert audit log..."
curl -sf "${API}/alerts/${ALERT_ID}/logs" \
  -H "Authorization: Bearer ${API_TOKEN}" | python3 -c "
import sys, json
logs = json.load(sys.stdin)
for log in logs:
    print(f'    {log[\"event\"]:15s} {log[\"message\"]}')" 2>/dev/null || true
echo ""

pause 3

# ============================================================================
heading "PHASE 4: Resolve -- Service Recovers"
# ============================================================================

narrate "AAA fixes the issue and restarts the API Server..."
echo ""

go run -C demo myapp.go -port ${API_PORT} &
PIDS+=($!)
sleep 2

if curl -sf "http://localhost:${API_PORT}/health" >/dev/null 2>&1; then
  ok "API Server is back up!"
else
  fire "API Server failed to restart"
fi

narrate "Resolving the alert..."
curl -sf -X POST "${API}/alerts/${ALERT_ID}/resolve" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" >/dev/null
ok "Alert resolved"

ALERT_STATUS=$(curl -sf "${API}/alerts/${ALERT_ID}" \
  -H "Authorization: Bearer ${API_TOKEN}" | jq_field status)
narrate "Alert status: ${ALERT_STATUS}"

pause 3

# ============================================================================
heading "PHASE 5: Incident Management"
# ============================================================================

narrate "Meanwhile, Payment Service also had issues during the outage."
narrate "AAA declares a cross-service incident..."
echo ""

INCIDENT_ID=$(curl -sf -X POST "${API}/incidents" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"title\":\"Cascading failure: API + Payment degradation\",\"severity\":\"critical\",\"summary\":\"API Server outage caused Payment Service timeouts. Multiple customers affected.\",\"created_by\":\"${AAA_ID}\"}" | jq_id)
ok "Incident created: ${INCIDENT_ID}"
narrate "Severity: critical | Status: triggered"

pause 2

narrate "AAA posts investigation update..."
curl -sf -X POST "${API}/incidents/${INCIDENT_ID}/updates" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"status\":\"investigating\",\"message\":\"Identified root cause: API Server OOM kill. Restarted with increased memory limits.\",\"created_by\":\"${AAA_ID}\"}" >/dev/null
ok "Status -> investigating"

pause 2

narrate "BBB confirms payment service recovered..."
curl -sf -X POST "${API}/incidents/${INCIDENT_ID}/updates" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"status\":\"identified\",\"message\":\"Payment Service reconnected to API. All queued payments processing. No data loss confirmed.\",\"created_by\":\"${BBB_ID}\"}" >/dev/null
ok "Status -> identified"

pause 2

narrate "CCC marks incident resolved after monitoring period..."
curl -sf -X POST "${API}/incidents/${INCIDENT_ID}/updates" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"status\":\"resolved\",\"message\":\"All services nominal for 30 minutes. Postmortem scheduled for Thursday.\",\"created_by\":\"${CCC_ID}\"}" >/dev/null
ok "Status -> resolved"

echo ""
narrate "Incident timeline:"
curl -sf "${API}/incidents/${INCIDENT_ID}/updates" \
  -H "Authorization: Bearer ${API_TOKEN}" | python3 -c "
import sys, json
updates = json.load(sys.stdin)
for u in updates:
    print(f'    [{u[\"status\"]:14s}] {u[\"message\"]}')" 2>/dev/null || true

pause 3

# ============================================================================
heading "PHASE 6: Schedule Override -- Vacation Swap"
# ============================================================================

narrate "AAA is going on vacation next week."
narrate "Creating a schedule override so BBB covers the on-call shift."
echo ""

# Start 2 seconds in the past to avoid any timing edge case
OVERRIDE_START=$(date -u -v-2S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "-2 seconds" +"%Y-%m-%dT%H:%M:%SZ")
OVERRIDE_END=$(date -u -v+7d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "+7 days" +"%Y-%m-%dT%H:%M:%SZ")

curl -sf -X POST "${API}/schedules/${SCHEDULE_ID}/overrides" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"replace_user\":\"${AAA_ID}\",\"override_user\":\"${BBB_ID}\",\"start_time\":\"${OVERRIDE_START}\",\"end_time\":\"${OVERRIDE_END}\"}" >/dev/null
ok "Override created: BBB covers for AAA (7 days)"

sleep 1

NEW_ONCALL=$(curl -sf "${API}/oncall/${SCHEDULE_ID}" \
  -H "Authorization: Bearer ${API_TOKEN}" | python3 -c "import sys,json; users=json.load(sys.stdin); print(users[0]['name'] if users else 'nobody')")
ok "Currently on-call: ${NEW_ONCALL}"

narrate "If an alert fires now, BBB gets paged instead of AAA."

pause 3

# ============================================================================
heading "PHASE 7: Deduplication"
# ============================================================================

narrate "Multiple monitors detect the same issue and fire alerts."
narrate "PageFire deduplicates by dedup_key -- only one alert is created."
echo ""

DEDUP_RESP_1=$(curl -s -X POST \
  "http://localhost:${PAGEFIRE_PORT}/api/v1/integrations/${API_IK}/alerts" \
  -H "Content-Type: application/json" \
  -d '{"summary":"High latency on API","details":"p99 > 500ms","dedup_key":"api-latency"}')
DEDUP_ID_1=$(echo "$DEDUP_RESP_1" | jq_id)
ok "Alert 1: ${DEDUP_ID_1}"

DEDUP_RESP_2=$(curl -s -X POST \
  "http://localhost:${PAGEFIRE_PORT}/api/v1/integrations/${API_IK}/alerts" \
  -H "Content-Type: application/json" \
  -d '{"summary":"High latency on API","details":"p99 > 500ms","dedup_key":"api-latency"}')
DEDUP_ID_2=$(echo "$DEDUP_RESP_2" | jq_id)
ok "Alert 2: ${DEDUP_ID_2}"

if [ "$DEDUP_ID_1" = "$DEDUP_ID_2" ]; then
  ok "Same alert ID -- deduplicated! No duplicate pages."
else
  fire "Different IDs -- deduplication may have failed"
fi

curl -sf -X POST "${API}/alerts/${DEDUP_ID_1}/resolve" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" >/dev/null

pause 2

# ============================================================================
heading "Demo Complete"
# ============================================================================

echo "  Features demonstrated:"
echo ""
echo "    - Multi-user org with roles"
echo "    - On-call schedule with weekly rotation"
echo "    - Multi-step escalation (on-call -> secondary -> manager)"
echo "    - Alert lifecycle (trigger -> acknowledge -> resolve)"
echo "    - Incident management with status timeline"
echo "    - Schedule overrides (vacation coverage)"
echo "    - Alert deduplication"
echo ""
echo "  Logs:  tail -f /tmp/pagefire-demo.log"
echo "  Alerts: curl -s -H 'Authorization: Bearer ${API_TOKEN}' \\"
echo "            http://localhost:${PAGEFIRE_PORT}/api/v1/alerts | python3 -m json.tool"
echo ""
echo "  Press Ctrl+C to stop everything."
echo ""

wait
