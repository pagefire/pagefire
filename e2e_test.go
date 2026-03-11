package pagefire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pagefire/pagefire/internal/api"
	"github.com/pagefire/pagefire/internal/auth"
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
	authSvc := auth.NewService(s.Users(), s.DB())
	router := api.NewRouter(s, resolver, dispatcher, authSvc, smokeToken)

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
		"name": "Alice", "email": "alice@example.com", "timezone": "UTC", "password": "TestPass123!",
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

// TestSmoke_RoutingAndGrouping exercises routing rules and alert grouping end-to-end.
func TestSmoke_RoutingAndGrouping(t *testing.T) {
	ctx := context.Background()

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
	authSvc := auth.NewService(s.Users(), s.DB())
	router := api.NewRouter(s, resolver, dispatcher, authSvc, smokeToken)

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

	doList := func(method, path string, body any, token string) (int, []map[string]any) {
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
		var result []map[string]any
		_ = json.NewDecoder(rr.Body).Decode(&result)
		return rr.Code, result
	}

	// 1. Create two escalation policies
	code, body := do("POST", "/api/v1/escalation-policies", map[string]any{
		"name": "Default", "repeat": 0,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create default EP: %d — %v", code, body)
	}
	defaultEPID := body["id"].(string)

	code, body = do("POST", "/api/v1/escalation-policies", map[string]any{
		"name": "Database", "repeat": 0,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create db EP: %d — %v", code, body)
	}
	dbEPID := body["id"].(string)

	// 2. Create service with default EP
	code, body = do("POST", "/api/v1/services", map[string]string{
		"name": "API", "escalation_policy_id": defaultEPID,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create service: %d — %v", code, body)
	}
	svcID := body["id"].(string)

	// 3. Create routing rule: summary contains "database" → use DB EP
	code, body = do("POST", "/api/v1/services/"+svcID+"/routing-rules", map[string]any{
		"condition_field": "summary", "condition_match_type": "contains",
		"condition_value": "database", "escalation_policy_id": dbEPID,
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create routing rule: %d — %v", code, body)
	}

	// 4. Create integration key
	code, body = do("POST", "/api/v1/services/"+svcID+"/integration-keys", map[string]string{
		"name": "Monitor",
	}, smokeToken)
	if code != 201 {
		t.Fatalf("create integration key: %d — %v", code, body)
	}
	secret := body["secret"].(string)

	// 5. Fire alert matching routing rule
	code, body = do("POST", "/api/v1/integrations/"+secret+"/alerts", map[string]string{
		"summary": "database connection timeout", "dedup_key": "db-1",
	}, "")
	if code != 201 {
		t.Fatalf("fire routed alert: %d — %v", code, body)
	}
	snapshot := body["escalation_policy_snapshot"].(string)
	if !strings.Contains(snapshot, dbEPID) {
		t.Fatalf("routed alert should use DB EP, snapshot: %s", snapshot)
	}

	// 6. Fire alert NOT matching — should use default EP
	code, body = do("POST", "/api/v1/integrations/"+secret+"/alerts", map[string]string{
		"summary": "high CPU", "dedup_key": "cpu-1",
	}, "")
	if code != 201 {
		t.Fatalf("fire default alert: %d — %v", code, body)
	}
	snapshot2 := body["escalation_policy_snapshot"].(string)
	if !strings.Contains(snapshot2, defaultEPID) {
		t.Fatalf("default alert should use default EP, snapshot: %s", snapshot2)
	}

	// 7. Alert grouping: fire two alerts with same group_key
	code, body = do("POST", "/api/v1/integrations/"+secret+"/alerts", map[string]string{
		"summary": "disk full host-1", "group_key": "disk-full",
	}, "")
	if code != 201 {
		t.Fatalf("fire grouped alert 1: %d — %v", code, body)
	}
	groupAlert1ID := body["id"].(string)

	code, body = do("POST", "/api/v1/integrations/"+secret+"/alerts", map[string]string{
		"summary": "disk full host-2", "group_key": "disk-full",
	}, "")
	if code != 201 {
		t.Fatalf("fire grouped alert 2: %d — %v", code, body)
	}
	groupAlert2ID := body["id"].(string)

	if groupAlert1ID == groupAlert2ID {
		t.Fatal("grouped alerts should have different IDs (not dedup)")
	}

	// 8. Filter by group_key
	code, groupedAlerts := doList("GET", "/api/v1/alerts?group_key=disk-full", nil, smokeToken)
	if code != 200 {
		t.Fatalf("list alerts by group_key: want 200, got %d", code)
	}
	if len(groupedAlerts) != 2 {
		t.Fatalf("expected 2 grouped alerts, got %d", len(groupedAlerts))
	}

	t.Log("smoke test passed: routing rules and alert grouping work end-to-end")
}

// TestSmoke_AuthFlow exercises the full auth lifecycle:
// setup (first admin) → login → create user (invite) → invite accept → API token → use token.
func TestSmoke_AuthFlow(t *testing.T) {
	ctx := context.Background()

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
	authSvc := auth.NewService(s.Users(), s.DB())
	router := api.NewRouter(s, resolver, dispatcher, authSvc, "") // no legacy token

	// cookieJar stores cookies between requests to simulate a browser session.
	var cookies []*http.Cookie

	doWithCookies := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req = httptest.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Collect Set-Cookie headers
		if setCookies := rr.Result().Cookies(); len(setCookies) > 0 {
			cookies = setCookies
		}

		var result map[string]any
		_ = json.NewDecoder(rr.Body).Decode(&result)
		return rr.Code, result
	}

	doWithToken := func(method, path string, body any, token string) (int, map[string]any) {
		t.Helper()
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req = httptest.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		var result map[string]any
		_ = json.NewDecoder(rr.Body).Decode(&result)
		return rr.Code, result
	}

	// 1. Setup check — should require setup
	code, body := doWithCookies("GET", "/api/v1/auth/setup", nil)
	if code != 200 {
		t.Fatalf("setup check: want 200, got %d", code)
	}
	if body["setup_required"] != true {
		t.Fatalf("setup_required: want true, got %v", body["setup_required"])
	}

	// 2. Create first admin via setup
	code, body = doWithCookies("POST", "/api/v1/auth/setup", map[string]string{
		"name": "Admin", "email": "admin@test.com", "password": "AdminPass123!",
	})
	if code != 201 {
		t.Fatalf("setup: want 201, got %d — %v", code, body)
	}
	if body["role"] != "admin" {
		t.Fatalf("setup: want role=admin, got %v", body["role"])
	}

	// 3. Setup again should fail (already done)
	code, _ = doWithCookies("POST", "/api/v1/auth/setup", map[string]string{
		"name": "Hacker", "email": "hack@test.com", "password": "HackPass123!",
	})
	if code != 409 {
		t.Fatalf("setup again: want 409, got %d", code)
	}

	// 4. Login
	cookies = nil // clear cookies
	code, body = doWithCookies("POST", "/api/v1/auth/login", map[string]string{
		"email": "admin@test.com", "password": "AdminPass123!",
	})
	if code != 200 {
		t.Fatalf("login: want 200, got %d — %v", code, body)
	}
	if body["name"] != "Admin" {
		t.Fatalf("login: want name=Admin, got %v", body["name"])
	}

	// 5. /me should return current user
	code, body = doWithCookies("GET", "/api/v1/auth/me", nil)
	if code != 200 {
		t.Fatalf("me: want 200, got %d — %v", code, body)
	}
	if body["email"] != "admin@test.com" {
		t.Fatalf("me: want email=admin@test.com, got %v", body["email"])
	}

	// 6. Wrong password should fail
	code, _ = doWithCookies("POST", "/api/v1/auth/login", map[string]string{
		"email": "admin@test.com", "password": "WrongPassword",
	})
	if code != 401 {
		t.Fatalf("bad login: want 401, got %d", code)
	}

	// 7. Create user (generates invite URL)
	code, body = doWithCookies("POST", "/api/v1/users", map[string]string{
		"name": "Bob", "email": "bob@test.com",
	})
	if code != 201 {
		t.Fatalf("create user: want 201, got %d — %v", code, body)
	}
	inviteURL, ok := body["invite_url"].(string)
	if !ok || inviteURL == "" {
		t.Fatalf("create user: expected invite_url, got %v", body)
	}
	// Extract token from invite URL (last path segment)
	parts := strings.Split(inviteURL, "/invite/")
	if len(parts) != 2 {
		t.Fatalf("unexpected invite URL format: %s", inviteURL)
	}
	inviteToken := parts[1]

	// 8. Validate invite token
	code, body = doWithCookies("GET", "/api/v1/auth/invite/"+inviteToken, nil)
	if code != 200 {
		t.Fatalf("invite check: want 200, got %d — %v", code, body)
	}
	if body["name"] != "Bob" {
		t.Fatalf("invite check: want name=Bob, got %v", body["name"])
	}

	// 9. Accept invite (set password)
	code, body = doWithCookies("POST", "/api/v1/auth/invite/"+inviteToken, map[string]string{
		"password": "BobPass1234!",
	})
	if code != 200 {
		t.Fatalf("invite accept: want 200, got %d — %v", code, body)
	}

	// 10. Invite token should be used — can't reuse
	code, _ = doWithCookies("GET", "/api/v1/auth/invite/"+inviteToken, nil)
	if code != 410 {
		t.Fatalf("invite reuse check: want 410 (Gone), got %d", code)
	}

	// 11. Bob can now login
	cookies = nil
	code, body = doWithCookies("POST", "/api/v1/auth/login", map[string]string{
		"email": "bob@test.com", "password": "BobPass1234!",
	})
	if code != 200 {
		t.Fatalf("bob login: want 200, got %d — %v", code, body)
	}

	// 12. Generate API token
	code, body = doWithCookies("POST", "/api/v1/auth/tokens", map[string]string{
		"name": "CI Token",
	})
	if code != 201 {
		t.Fatalf("create token: want 201, got %d — %v", code, body)
	}
	apiToken, ok := body["token"].(string)
	if !ok || !strings.HasPrefix(apiToken, "pf_") {
		t.Fatalf("create token: expected pf_ prefix, got %v", body["token"])
	}
	tokenID := body["id"].(string)

	// 13. Use API token to access alerts
	code, _ = doWithToken("GET", "/api/v1/alerts", nil, apiToken)
	if code != 200 {
		t.Fatalf("alerts with API token: want 200, got %d", code)
	}

	// 14. List tokens
	code, body = doWithCookies("GET", "/api/v1/auth/tokens", nil)
	if code != 200 {
		t.Fatalf("list tokens: want 200, got %d", code)
	}

	// 15. Revoke token
	code, _ = doWithCookies("DELETE", "/api/v1/auth/tokens/"+tokenID, nil)
	if code != 200 {
		t.Fatalf("revoke token: want 200, got %d", code)
	}

	// 16. Revoked token should fail
	code, _ = doWithToken("GET", "/api/v1/alerts", nil, apiToken)
	if code != 401 {
		t.Fatalf("revoked token: want 401, got %d", code)
	}

	// 17. Logout
	code, _ = doWithCookies("POST", "/api/v1/auth/logout", nil)
	if code != 200 {
		t.Fatalf("logout: want 200, got %d", code)
	}

	t.Log("smoke test passed: full auth lifecycle (setup → login → invite → API token → revoke → logout)")
}
