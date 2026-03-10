package pagefire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pagefire/pagefire/internal/api"
	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store/sqlite"
)

const smokeToken = "smoke-test-token"

// TestSmoke_FullAlertFlow boots the full router and walks through:
// healthz → create user → create contact method → create notification rule →
// create escalation policy → add step → add target → create service →
// create integration key → fire alert via integration webhook → verify alert exists.
func TestSmoke_FullAlertFlow(t *testing.T) {
	ctx := context.Background()

	// Boot in-memory store + router (same wiring as app.New, minus engine/server).
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	dispatcher := notification.NewDispatcher()
	router := api.NewRouter(s, resolver, dispatcher, smokeToken)

	// Helper to make requests and decode JSON.
	do := func(method, path string, body any, token string) (int, map[string]any) {
		t.Helper()
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req = httptest.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		var result map[string]any
		_ = json.NewDecoder(rr.Body).Decode(&result)
		return rr.Code, result
	}

	// 1. Health check
	code, body := do("GET", "/healthz", nil, "")
	if code != 200 {
		t.Fatalf("healthz: want 200, got %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("healthz: want status=ok, got %v", body["status"])
	}

	// 2. Create user
	code, body = do("POST", "/api/v1/users", map[string]string{
		"name": "Alice", "email": "alice@example.com", "timezone": "UTC",
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create user: want 201, got %d — %v", code, body)
	}
	userID := body["id"].(string)

	// 3. Create contact method
	code, body = do("POST", "/api/v1/users/"+userID+"/contact-methods", map[string]string{
		"type": "email", "value": "alice@company.com",
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create contact method: want 201, got %d — %v", code, body)
	}
	cmID := body["id"].(string)

	// 4. Create notification rule
	code, body = do("POST", "/api/v1/users/"+userID+"/notification-rules", map[string]any{
		"contact_method_id": cmID, "delay_minutes": 0,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create notification rule: want 201, got %d — %v", code, body)
	}

	// 5. Create escalation policy
	code, body = do("POST", "/api/v1/escalation-policies", map[string]any{
		"name": "Default", "repeat": 1,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create escalation policy: want 201, got %d — %v", code, body)
	}
	policyID := body["id"].(string)

	// 6. Add escalation step
	code, body = do("POST", "/api/v1/escalation-policies/"+policyID+"/steps", map[string]any{
		"step_number": 0, "delay_minutes": 5,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create escalation step: want 201, got %d — %v", code, body)
	}
	stepID := body["id"].(string)

	// 7. Add step target (user)
	code, body = do("POST", "/api/v1/escalation-policies/"+policyID+"/steps/"+stepID+"/targets", map[string]string{
		"target_type": "user", "target_id": userID,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create step target: want 201, got %d — %v", code, body)
	}

	// 8. Create service
	code, body = do("POST", "/api/v1/services", map[string]string{
		"name": "API Service", "escalation_policy_id": policyID,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create service: want 201, got %d — %v", code, body)
	}
	serviceID := body["id"].(string)
	_ = serviceID

	// 9. Create integration key
	code, body = do("POST", "/api/v1/services/"+serviceID+"/integration-keys", map[string]string{
		"name": "Monitoring",
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create integration key: want 201, got %d — %v", code, body)
	}
	secret := body["secret"].(string)
	if len(secret) == 0 {
		t.Fatal("integration key: expected non-empty secret")
	}

	// 10. Fire alert via integration webhook (no Bearer token — auth by key)
	code, body = do("POST", "/api/v1/integrations/"+secret+"/alerts", map[string]string{
		"summary": "CPU > 90%", "details": "Host web-1 at 95% for 5m",
	}, "") // no auth token — the key IS the auth
	if code != 201 {
		t.Fatalf("fire alert: want 201, got %d — %v", code, body)
	}
	alertID := body["id"].(string)
	if alertID == "" {
		t.Fatal("fire alert: expected non-empty alert ID")
	}
	if body["status"] != "triggered" {
		t.Fatalf("fire alert: want status=triggered, got %v", body["status"])
	}
	if body["summary"] != "CPU > 90%" {
		t.Fatalf("fire alert: want summary='CPU > 90%%', got %v", body["summary"])
	}

	// 11. Verify alert appears in list
	code, _ = do("GET", "/api/v1/alerts", nil, smokeToken)
	if code != 200 {
		t.Fatalf("list alerts: want 200, got %d", code)
	}

	// 12. Acknowledge alert
	code, body = do("POST", "/api/v1/alerts/"+alertID+"/acknowledge", map[string]string{
		"user_id": userID,
	}, smokeToken)
	if code != 200 {
		t.Fatalf("acknowledge alert: want 200, got %d — %v", code, body)
	}

	// 13. Resolve alert
	code, body = do("POST", "/api/v1/alerts/"+alertID+"/resolve", map[string]string{
		"user_id": userID,
	}, smokeToken)
	if code != 200 {
		t.Fatalf("resolve alert: want 200, got %d — %v", code, body)
	}

	// 14. Verify alert is resolved
	code, body = do("GET", "/api/v1/alerts/"+alertID, nil, smokeToken)
	if code != 200 {
		t.Fatalf("get alert: want 200, got %d", code)
	}
	if body["status"] != "resolved" {
		t.Fatalf("get alert: want status=resolved, got %v", body["status"])
	}

	t.Log("smoke test passed: full alert lifecycle complete")
}
