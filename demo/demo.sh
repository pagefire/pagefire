#!/usr/bin/env bash
set -euo pipefail

# --- PageFire Demo --------------------------------------------------------
# Simulates a developer using PageFire to monitor their app.
#
# What happens:
#   1. Starts PageFire (port 3001)
#   2. Starts a fake app (port 8080)
#   3. Starts a notification receiver (port 9090) -- prints alerts you'd get
#   4. Sets up PageFire: user, contact method, escalation policy, service
#   5. Starts a health checker that monitors the app
#   6. You kill the app -> health check fails -> PageFire alert -> you get paged
#   7. Restart the app -> health check passes -> alert resolves
# ---------------------------------------------------------------------------

PAGEFIRE_PORT=3001
MYAPP_PORT=8080
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
  for port in ${PAGEFIRE_PORT} ${MYAPP_PORT} ${NOTIFY_PORT}; do
    kill $(lsof -ti:${port}) 2>/dev/null || true
  done
  rm -f /tmp/pagefire-demo.db "${COOKIE_JAR}"
  echo "Done. Logs preserved at /tmp/pagefire-demo.log"
}
trap cleanup EXIT

log()  { echo "[==>] $1"; }
ok()   { echo " [ok] $1"; }
fire() { echo "[!!] $1"; }

# --- Clean stale state -----------------------------------------------------
rm -f /tmp/pagefire-demo.db /tmp/pagefire-demo.log "${COOKIE_JAR}"
for port in ${PAGEFIRE_PORT} ${MYAPP_PORT} ${NOTIFY_PORT}; do
  kill $(lsof -ti:${port}) 2>/dev/null || true
done

# --- Check prerequisites ---------------------------------------------------
if ! command -v go &>/dev/null; then
  echo "Error: Go is required. Install from https://go.dev"
  exit 1
fi

# --- Build PageFire ---------------------------------------------------------
log "Building PageFire..."
cd "$(dirname "$0")/.."
make build 2>/dev/null
ok "Built ./bin/pagefire"

# --- Start notification receiver --------------------------------------------
log "Starting notification receiver on :${NOTIFY_PORT}..."

python3 -u -c "
import json, http.server, sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        payload = json.loads(body) if body else {}
        subject = payload.get('subject', '')
        alert_body = payload.get('body', '')
        user_name = payload.get('user_name', '')
        import datetime
        now = datetime.datetime.now().strftime('%H:%M:%S')
        print(flush=True)
        print('--- PAGE RECEIVED ---', flush=True)
        if user_name:
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
ok "Notification receiver listening on :${NOTIFY_PORT}"

# --- Start PageFire ---------------------------------------------------------
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

# --- Start fake app ---------------------------------------------------------
log "Starting app on :${MYAPP_PORT}..."
go run -C demo myapp.go &
PIDS+=($!)
sleep 1
ok "App is running -- GET http://localhost:${MYAPP_PORT}/health returns 200"

# --- Set up PageFire --------------------------------------------------------
echo ""
log "Setting up PageFire..."

# Create admin user via setup endpoint (no auth required for first user)
USER_ID=$(curl -sf -X POST "${API}/auth/setup" \
  -c "${COOKIE_JAR}" \
  -H "Content-Type: application/json" \
  -d '{"name":"User A","email":"a@demo.dev","password":"demo-password-123"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created admin user: User A (${USER_ID})"

# Generate an API token using the session cookie from setup
API_TOKEN=$(curl -sf -X POST "${API}/auth/tokens" \
  -b "${COOKIE_JAR}" \
  -H "Content-Type: application/json" \
  -d '{"name":"demo-script"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
ok "Generated API token"

CM_ID=$(curl -sf -X POST "${API}/users/${USER_ID}/contact-methods" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"type\":\"webhook\",\"value\":\"http://localhost:${NOTIFY_PORT}/notify\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created contact method: webhook -> localhost:${NOTIFY_PORT}"

curl -sf -X POST "${API}/users/${USER_ID}/notification-rules" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"contact_method_id\":\"${CM_ID}\",\"delay_minutes\":0}" >/dev/null
ok "Created notification rule: notify immediately"

EP_ID=$(curl -sf -X POST "${API}/escalation-policies" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Default Escalation","repeat":2}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created escalation policy: Default Escalation (repeat 2x)"

STEP_ID=$(curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"step_number":0,"delay_minutes":1}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created escalation step: notify after 0 min, re-escalate after 1 min"

curl -sf -X POST "${API}/escalation-policies/${EP_ID}/steps/${STEP_ID}/targets" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"target_type\":\"user\",\"target_id\":\"${USER_ID}\"}" >/dev/null
ok "Added target: User A"

SVC_ID=$(curl -sf -X POST "${API}/services" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"name\":\"Demo App\",\"escalation_policy_id\":\"${EP_ID}\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created service: Demo App"

IK_SECRET=$(curl -sf -X POST "${API}/services/${SVC_ID}/integration-keys" \
  -H "Authorization: Bearer ${API_TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Health Checker"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['secret'])")
ok "Created integration key for health checker"

echo ""
log "Setup complete. Running:"
echo "  PageFire:       http://localhost:${PAGEFIRE_PORT}"
echo "  App:            http://localhost:${MYAPP_PORT}"
echo "  Notifications:  http://localhost:${NOTIFY_PORT}"
echo ""

# --- Start health checker ---------------------------------------------------
log "Starting health checker (polls app every 5s)..."
echo ""

ALERT_FIRED=""

health_check() {
  while true; do
    if curl -sf "http://localhost:${MYAPP_PORT}/health" >/dev/null 2>&1; then
      if [ -n "$ALERT_FIRED" ]; then
        ok "App is back! Resolving alert..."
        curl -sf -X POST \
          "http://localhost:${PAGEFIRE_PORT}/api/v1/alerts/${ALERT_ID}/resolve" \
          -H "Authorization: Bearer ${API_TOKEN}" \
          -H "Content-Type: application/json" \
          -d "{\"user_id\":\"${USER_ID}\"}" >/dev/null 2>&1 || true
        ok "Alert resolved. No more pages."
        ALERT_FIRED=""
      fi
    else
      if [ -z "$ALERT_FIRED" ]; then
        fire "App is DOWN! Firing alert to PageFire..."
        RESPONSE=$(curl -sf -X POST \
          "http://localhost:${PAGEFIRE_PORT}/api/v1/integrations/${IK_SECRET}/alerts" \
          -H "Content-Type: application/json" \
          -d '{"summary":"App health check failed","details":"GET /health returned non-200 or timed out","dedup_key":"app-health"}' 2>&1) || true
        if [ -n "$RESPONSE" ]; then
          ALERT_ID=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
          fire "Alert created: ${ALERT_ID}"
          fire "Waiting for PageFire to escalate and notify you..."
        fi
        ALERT_FIRED="true"
      fi
    fi
    sleep 5
  done
}

health_check &
PIDS+=($!)

echo "--------------------------------------------------------"
echo ""
echo "  The health checker is monitoring the app."
echo ""
echo "  Try this:"
echo "    1. Kill the app:    kill \$(lsof -ti:${MYAPP_PORT})"
echo "       -> Watch the alert fire and notification arrive"
echo ""
echo "    2. Restart the app: go run demo/myapp.go &"
echo "       -> Health check passes again"
echo ""
echo "    3. Check alerts:  curl -s -H 'Authorization: Bearer ${API_TOKEN}' \\"
echo "                        http://localhost:${PAGEFIRE_PORT}/api/v1/alerts | python3 -m json.tool"
echo ""
echo "    4. View logs:     tail -f /tmp/pagefire-demo.log"
echo ""
echo "  Press Ctrl+C to stop everything."
echo ""
echo "--------------------------------------------------------"

wait
