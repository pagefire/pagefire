package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pagefire/pagefire/internal/notification"
	"github.com/pagefire/pagefire/internal/oncall"
	"github.com/pagefire/pagefire/internal/store/sqlite"
)

const testToken = "test-token-123"

func newTestRouter(t *testing.T) (http.Handler, *sqlite.SQLiteStore) {
	t.Helper()
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	resolver := oncall.NewResolver(s.Schedules(), s.Users())
	dispatcher := notification.NewDispatcher()
	router := NewRouter(s, resolver, dispatcher, testToken)
	return router, s
}

func doRequest(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

// --- Auth tests ---

func TestHealthzNoAuth(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/healthz", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz: want 200, got %d", rr.Code)
	}
	var body map[string]string
	decodeBody(t, rr, &body)
	if body["status"] != "ok" {
		t.Fatalf("GET /healthz: want status=ok, got %q", body["status"])
	}
}

func TestAuthRequired(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/users", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/users without auth: want 401, got %d", rr.Code)
	}
}

func TestAuthWrongToken(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/users", nil, "wrong-token")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/users with wrong token: want 401, got %d", rr.Code)
	}
}

func TestAuthCorrectToken(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/users", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/users with correct token: want 200, got %d", rr.Code)
	}
}

// --- User CRUD tests ---

func TestCreateUser(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/users", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/users: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var user map[string]any
	decodeBody(t, rr, &user)
	if user["id"] == nil || user["id"] == "" {
		t.Fatal("POST /api/v1/users: response missing id")
	}
	if user["name"] != "Alice" {
		t.Fatalf("POST /api/v1/users: want name=Alice, got %q", user["name"])
	}
	if user["email"] != "alice@example.com" {
		t.Fatalf("POST /api/v1/users: want email=alice@example.com, got %q", user["email"])
	}
}

func TestCreateUserMissingName(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{
		"email": "alice@example.com",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/users", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/users without name: want 400, got %d", rr.Code)
	}
}

func TestCreateUserInvalidEmail(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{
		"name":  "Alice",
		"email": "not-an-email",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/users", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/users with invalid email: want 400, got %d", rr.Code)
	}
}

func TestGetUser(t *testing.T) {
	router, _ := newTestRouter(t)

	// Create a user first.
	createBody := map[string]string{
		"name":  "Bob",
		"email": "bob@example.com",
	}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/users", createBody, testToken)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("setup: create user got %d", createRR.Code)
	}
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	// Get the user.
	rr := doRequest(t, router, http.MethodGet, "/api/v1/users/"+id, nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/users/%s: want 200, got %d", id, rr.Code)
	}
	var user map[string]any
	decodeBody(t, rr, &user)
	if user["name"] != "Bob" {
		t.Fatalf("GET user: want name=Bob, got %q", user["name"])
	}
	if user["email"] != "bob@example.com" {
		t.Fatalf("GET user: want email=bob@example.com, got %q", user["email"])
	}
	if user["role"] != "user" {
		t.Fatalf("GET user: want role=user, got %q", user["role"])
	}
}

func TestGetUserNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/users/nonexistent-id", nil, testToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/users/nonexistent: want 404, got %d", rr.Code)
	}
}

func TestUpdateUser(t *testing.T) {
	router, _ := newTestRouter(t)

	// Create a user first.
	createBody := map[string]string{
		"name":  "Charlie",
		"email": "charlie@example.com",
	}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/users", createBody, testToken)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("setup: create user got %d", createRR.Code)
	}
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	// Update the user.
	updateBody := map[string]string{
		"name": "Charlie Updated",
	}
	rr := doRequest(t, router, http.MethodPut, "/api/v1/users/"+id, updateBody, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/users/%s: want 200, got %d; body: %s", id, rr.Code, rr.Body.String())
	}
}

func TestCreateUserRoleEnforced(t *testing.T) {
	router, _ := newTestRouter(t)

	// Try to create a user with role=admin; server should force role=user.
	body := map[string]string{
		"name":  "Mallory",
		"email": "mallory@example.com",
		"role":  "admin",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/users", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/users: want 201, got %d", rr.Code)
	}
	var user map[string]any
	decodeBody(t, rr, &user)
	if user["role"] != "user" {
		t.Fatalf("POST /api/v1/users with role=admin: server should enforce role=user, got %q", user["role"])
	}
}

// --- Service tests ---

func TestCreateService(t *testing.T) {
	router, _ := newTestRouter(t)

	// Create an escalation policy first (required by service).
	epBody := map[string]any{
		"name":   "Default EP",
		"repeat": 0,
	}
	epRR := doRequest(t, router, http.MethodPost, "/api/v1/escalation-policies", epBody, testToken)
	if epRR.Code != http.StatusCreated {
		t.Fatalf("setup: create escalation policy got %d; body: %s", epRR.Code, epRR.Body.String())
	}
	var ep map[string]any
	decodeBody(t, epRR, &ep)
	epID := ep["id"].(string)

	body := map[string]string{
		"name":                 "My Service",
		"escalation_policy_id": epID,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/services: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var svc map[string]any
	decodeBody(t, rr, &svc)
	if svc["id"] == nil || svc["id"] == "" {
		t.Fatal("POST /api/v1/services: response missing id")
	}
}

func TestListServices(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/services", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/services: want 200, got %d", rr.Code)
	}
}

// --- Integration key tests ---

func createTestService(t *testing.T, router http.Handler) string {
	t.Helper()

	// Create escalation policy.
	epBody := map[string]any{"name": "Test EP", "repeat": 0}
	epRR := doRequest(t, router, http.MethodPost, "/api/v1/escalation-policies", epBody, testToken)
	if epRR.Code != http.StatusCreated {
		t.Fatalf("setup: create EP got %d; body: %s", epRR.Code, epRR.Body.String())
	}
	var ep map[string]any
	decodeBody(t, epRR, &ep)

	// Create service.
	svcBody := map[string]string{
		"name":                 "Test Service",
		"escalation_policy_id": ep["id"].(string),
	}
	svcRR := doRequest(t, router, http.MethodPost, "/api/v1/services", svcBody, testToken)
	if svcRR.Code != http.StatusCreated {
		t.Fatalf("setup: create service got %d; body: %s", svcRR.Code, svcRR.Body.String())
	}
	var svc map[string]any
	decodeBody(t, svcRR, &svc)
	return svc["id"].(string)
}

func TestCreateIntegrationKey(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)

	body := map[string]string{"name": "Grafana Webhook"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/integration-keys", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST integration-keys: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var ik map[string]any
	decodeBody(t, rr, &ik)
	secret, ok := ik["secret"].(string)
	if !ok || secret == "" {
		t.Fatal("POST integration-keys: response missing secret (one-time view)")
	}
}

func TestListIntegrationKeysSecretMasked(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)

	// Create an integration key.
	body := map[string]string{"name": "Test Key"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/integration-keys", body, testToken)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("setup: create integration key got %d", createRR.Code)
	}

	// List integration keys — secrets must be masked.
	rr := doRequest(t, router, http.MethodGet, "/api/v1/services/"+svcID+"/integration-keys", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET integration-keys: want 200, got %d", rr.Code)
	}
	var keys []map[string]any
	decodeBody(t, rr, &keys)
	if len(keys) == 0 {
		t.Fatal("GET integration-keys: expected at least one key")
	}
	prefix, _ := keys[0]["secret_prefix"].(string)
	if !strings.Contains(prefix, "****") {
		t.Fatalf("GET integration-keys: secret_prefix should be masked, got %q", prefix)
	}
	// The full secret field should not be present in list responses.
	if _, hasSecret := keys[0]["secret"]; hasSecret {
		t.Fatal("GET integration-keys: full secret should not appear in list response")
	}
}

// --- Request body limit test ---

func TestRequestBodyLimit(t *testing.T) {
	router, _ := newTestRouter(t)

	// Send a body larger than 1MB.
	bigBody := strings.NewReader(strings.Repeat("x", 2<<20))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bigBody)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should be rejected with 400 (body too large triggers decode error).
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("POST with >1MB body: want 400 or 413, got %d", rr.Code)
	}
}

// --- Security headers test ---

func TestSecurityHeadersOnRoutes(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/healthz", nil, "")

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
	}
	for name, want := range headers {
		got := rr.Header().Get(name)
		if got != want {
			t.Errorf("header %s: want %q, got %q", name, want, got)
		}
	}
}
