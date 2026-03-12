#!/usr/bin/env bash
set -euo pipefail

# --- PageFire Live Demo -------------------------------------------------------
# Uses the EXISTING pagefire.db (preserves users + sessions so you stay logged in).
# Wipes all other data, sets up aru-jo on-call, and runs a fake app + health
# checker that fires real alerts through the integration webhook.
#
# Usage:
#   ./demo/live-demo.sh
#
# Then open http://localhost:3000 in your browser (you should already be logged in).
# Kill the fake app to trigger alerts, restart it to auto-resolve.
# -------------------------------------------------------------------------------

PAGEFIRE_PORT=3000
MYAPP_PORT=8080
NOTIFY_PORT=9090
API="http://localhost:${PAGEFIRE_PORT}/api/v1"
DB="pagefire.db"
PIDS=()
COOKIE_JAR="/tmp/pagefire-live-cookies.txt"

cd "$(dirname "$0")/.."

cleanup() {
  echo ""
  echo "Cleaning up..."
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  # Kill fake app and notification receiver
  kill $(lsof -ti:${MYAPP_PORT}) 2>/dev/null || true
  kill $(lsof -ti:${NOTIFY_PORT}) 2>/dev/null || true
  rm -f "${COOKIE_JAR}"
  echo "Done. PageFire is still running on :${PAGEFIRE_PORT}."
}
trap cleanup EXIT

log()  { echo "[==>] $1"; }
ok()   { echo " [ok] $1"; }
fire() { echo " [!!] $1"; }
jq_id()     { python3 -c "import sys,json; print(json.load(sys.stdin)['id'])"; }
jq_secret() { python3 -c "import sys,json; print(json.load(sys.stdin)['secret'])"; }

# --- Kill existing PageFire ---------------------------------------------------
log "Stopping existing PageFire (if running)..."
kill $(lsof -ti:${PAGEFIRE_PORT}) 2>/dev/null || true
sleep 1

# --- Wipe data (keep users + sessions) ----------------------------------------
log "Wiping data (keeping users & sessions)..."
sqlite3 "${DB}" <<'SQL'
DELETE FROM alert_logs;
DELETE FROM notification_queue;
DELETE FROM alerts;
DELETE FROM incident_updates;
DELETE FROM incident_services;
DELETE FROM incidents;
DELETE FROM routing_rules;
DELETE FROM integration_keys;
DELETE FROM services;
DELETE FROM escalation_step_targets;
DELETE FROM escalation_steps;
DELETE FROM escalation_policies;
DELETE FROM schedule_overrides;
DELETE FROM rotation_participants;
DELETE FROM rotations;
DELETE FROM schedules;
DELETE FROM team_members;
DELETE FROM teams;
DELETE FROM contact_methods;
DELETE FROM notification_rules;
DELETE FROM api_tokens;
DELETE FROM invite_tokens;
SQL
ok "Data wiped (users & sessions preserved)"

# --- Get aru-jo user ID -------------------------------------------------------
ARU_ID=$(sqlite3 "${DB}" "SELECT id FROM users WHERE email = 'aravindjyothi97@gmail.com';")
if [ -z "$ARU_ID" ]; then
  echo "Error: aru-jo user not found in DB"
  exit 1
fi
ok "Found aru-jo: ${ARU_ID}"

# --- Start notification receiver -----------------------------------------------
log "Starting notification receiver on :${NOTIFY_PORT}..."
python3 -u -c "
import json, http.server, datetime

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        payload = json.loads(body) if body else {}
        subject = payload.get('subject', '')
        alert_body = payload.get('body', '')
        user_name = payload.get('user_name', '')
        now = datetime.datetime.now().strftime('%H:%M:%S')
        print(flush=True)
        print('=== PAGE RECEIVED ===', flush=True)
        print(f'  To:      {user_name}', flush=True)
        print(f'  Time:    {now}', flush=True)
        print(f'  Subject: {subject}', flush=True)
        print(f'  Body:    {alert_body}', flush=True)
        print('=====================', flush=True)
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

# --- Start PageFire ------------------------------------------------------------
log "Starting PageFire on :${PAGEFIRE_PORT}..."
PAGEFIRE_PORT="${PAGEFIRE_PORT}" \
PAGEFIRE_DATABASE_URL="${DB}" \
PAGEFIRE_LOG_LEVEL="info" \
PAGEFIRE_ENGINE_INTERVAL_SECONDS=3 \
PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS=true \
./bin/pagefire serve 2>/tmp/pagefire-live.log &
PIDS+=($!)
sleep 2

if ! curl -sf "http://localhost:${PAGEFIRE_PORT}/healthz" >/dev/null 2>&1; then
  echo "Error: PageFire failed to start. Check /tmp/pagefire-live.log"
  exit 1
fi
ok "PageFire is running"

# --- Login and get API token ---------------------------------------------------
log "Logging in as aru-jo to get API token..."
curl -sf -X POST "${API}/auth/login" \
  -c "${COOKIE_JAR}" \
  -H "Content-Type: application/json" \
  -d '{"email":"aravindjyothi97@gmail.com","password":"demo-password-123"}' >/dev/null 2>&1 || {
  echo "Warning: Login failed. You may need to update the password in this script."
  echo "Continuing with session cookie from browser..."
}

API_TOKEN=$(curl -sf -X POST "${API}/auth/tokens" \
  -b "${COOKIE_JAR}" \
  -H "Content-Type: application/json" \
  -d '{"name":"live-demo-script"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
ok "Generated API token for scripting"

# --- Set up contact method + notification rule for aru-jo ----------------------
log "Setting up aru-jo's contact method & notifications..."

CM_ID=$(curl -sf -X POST "${API}/users/${ARU_ID}/contact-methods" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"type\":\"webhook\",\"value\":\"http://localhost:${NOTIFY_PORT}/notify\"}" | jq_id)
ok "Contact method: webhook -> localhost:${NOTIFY_PORT}"

curl -sf -X POST "${API}/users/${ARU_ID}/notification-rules" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"contact_method_id\":\"${CM_ID}\",\"delay_minutes\":0}" >/dev/null
ok "Notification rule: notify immediately"

# --- Create on-call schedule with aru-jo --------------------------------------
log "Creating on-call schedule..."

SCHEDULE_ID=$(curl -sf -X POST "${API}/schedules" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Primary On-Call","timezone":"America/Los_Angeles"}' | jq_id)
ok "Schedule: Primary On-Call"

NOW_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
ROTATION_ID=$(curl -sf -X POST "${API}/schedules/${SCHEDULE_ID}/rotations" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"name\":\"Weekly\",\"type\":\"weekly\",\"shift_length\":1,\"start_time\":\"${NOW_ISO}\"}" | jq_id)
ok "Rotation: weekly starting now"

curl -sf -X POST "${API}/schedules/${SCHEDULE_ID}/rotations/${ROTATION_ID}/participants" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"${ARU_ID}\",\"position\":0}" >/dev/null
ok "aru-jo is on-call (position 0)"

ONCALL=$(curl -sf "${API}/oncall/${SCHEDULE_ID}" \
  -H "Authorization: Bearer ${API_TOKEN}" | python3 -c "import sys,json; users=json.load(sys.stdin); print(users[0]['name'] if users else 'nobody')")
ok "Currently on-call: ${ONCALL}"

# --- Create escalation policy -------------------------------------------------
log "Creating escalation policy..."

EP_ID=$(curl -sf -X POST "${API}/escalation-policies" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Production Critical","repeat":2}' | jq_id)
ok "Policy: Production Critical (repeat 2x)"

STEP_ID=$(curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"step_number":0,"delay_minutes":1}' | jq_id)
ok "Step 0: notify immediately, re-escalate after 1 min"

curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps/${STEP_ID}/targets" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"target_type\":\"schedule\",\"target_id\":\"${SCHEDULE_ID}\"}" >/dev/null
ok "Target: Primary On-Call schedule (aru-jo)"

# --- Create service + integration key -----------------------------------------
log "Creating service..."

SVC_ID=$(curl -sf -X POST "${API}/services" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"name\":\"My API\",\"escalation_policy_id\":\"${EP_ID}\",\"description\":\"Production API server\"}" | jq_id)
ok "Service: My API"

IK_SECRET=$(curl -sf -X POST "${API}/services/${SVC_ID}/integration-keys" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Health Checker","type":"generic"}' | jq_secret)
ok "Integration key created"

# --- Start fake app ------------------------------------------------------------
log "Starting fake app on :${MYAPP_PORT}..."

kill $(lsof -ti:${MYAPP_PORT}) 2>/dev/null || true
sleep 1
go run -C demo myapp.go -port ${MYAPP_PORT} &
PIDS+=($!)
sleep 2
ok "Fake app running at http://localhost:${MYAPP_PORT}/health"

# --- Start health checker ------------------------------------------------------
echo ""
echo "==========================================================="
echo "  LIVE DEMO READY"
echo "==========================================================="
echo ""
echo "  PageFire UI:     http://localhost:${PAGEFIRE_PORT}"
echo "  Fake App:        http://localhost:${MYAPP_PORT}"
echo "  Notifications:   http://localhost:${NOTIFY_PORT}"
echo ""
echo "  You are logged in as aru-jo and ON-CALL."
echo ""
echo "  To trigger an alert:"
echo "    kill \$(lsof -ti:${MYAPP_PORT})"
echo ""
echo "  To restart the app (auto-resolves):"
echo "    go run demo/myapp.go &"
echo ""
echo "  Health checker polls every 5s. Watch for PAGE RECEIVED."
echo "  Press Ctrl+C to stop the demo."
echo ""
echo "==========================================================="
echo ""

LAST_ALERT_ID=""
APP_WAS_DOWN=""

while true; do
  if curl -sf "http://localhost:${MYAPP_PORT}/health" >/dev/null 2>&1; then
    # App is healthy
    if [ -n "$APP_WAS_DOWN" ]; then
      ok "App is back! Resolving alert..."
      if [ -n "$LAST_ALERT_ID" ]; then
        curl -sf -X POST "${API}/alerts/${LAST_ALERT_ID}/resolve" \
          -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" >/dev/null 2>&1 || true
        ok "Alert resolved"
      fi
      APP_WAS_DOWN=""
      LAST_ALERT_ID=""
    fi
  else
    # App is down — always POST, let server dedup handle duplicates.
    # If the alert was manually resolved in the UI, this creates a fresh one.
    RESPONSE=$(curl -sf -X POST \
      "http://localhost:${PAGEFIRE_PORT}/api/v1/integrations/${IK_SECRET}/alerts" \
      -H "Content-Type: application/json" \
      -d '{"summary":"API health check failed","details":"GET /health returned non-200 or connection refused","dedup_key":"api-health"}' 2>&1) || true
    if [ -n "$RESPONSE" ]; then
      NEW_ID=$(echo "$RESPONSE" | jq_id 2>/dev/null || echo "")
      if [ "$NEW_ID" != "$LAST_ALERT_ID" ]; then
        fire "App is DOWN! Alert created: ${NEW_ID}"
        fire "PageFire will escalate and page you..."
        LAST_ALERT_ID="$NEW_ID"
      fi
    fi
    APP_WAS_DOWN="true"
  fi
  sleep 5
done
