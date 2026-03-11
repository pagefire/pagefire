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
	"github.com/pagefire/pagefire/internal/store"
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

// --- Integration dedup test ---

func TestIntegrationDedupReturns200(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	// Create escalation policy, service, integration key via store directly
	ep := &store.EscalationPolicy{Name: "Test EP", Repeat: 0}
	if err := s.EscalationPolicies().Create(ctx, ep); err != nil {
		t.Fatal(err)
	}
	svc := &store.Service{Name: "Test Svc", EscalationPolicyID: ep.ID}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}
	ik := &store.IntegrationKey{ServiceID: svc.ID, Name: "test-key", Type: "generic"}
	if err := s.Services().CreateIntegrationKey(ctx, ik); err != nil {
		t.Fatal(err)
	}

	body := map[string]string{
		"summary":   "disk full",
		"details":   "root at 99%",
		"dedup_key": "disk-full",
	}

	// First alert: should be 201 Created
	rr1 := doRequest(t, router, http.MethodPost, "/api/v1/integrations/"+ik.Secret+"/alerts", body, "")
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first alert: want 201, got %d; body: %s", rr1.Code, rr1.Body.String())
	}
	var alert1 map[string]any
	decodeBody(t, rr1, &alert1)

	// Second alert with same dedup_key: should be 200 OK with same ID
	rr2 := doRequest(t, router, http.MethodPost, "/api/v1/integrations/"+ik.Secret+"/alerts", body, "")
	if rr2.Code != http.StatusOK {
		t.Fatalf("dedup alert: want 200, got %d; body: %s", rr2.Code, rr2.Body.String())
	}
	var alert2 map[string]any
	decodeBody(t, rr2, &alert2)

	if alert2["id"] != alert1["id"] {
		t.Errorf("dedup should return same alert ID: got %v, want %v", alert2["id"], alert1["id"])
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

// --- Team CRUD tests ---

func TestCreateTeam(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{
		"name":        "Platform",
		"description": "Platform team",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/teams: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var team map[string]any
	decodeBody(t, rr, &team)
	if team["id"] == nil || team["id"] == "" {
		t.Fatal("POST /api/v1/teams: response missing id")
	}
	if team["name"] != "Platform" {
		t.Fatalf("name = %q, want %q", team["name"], "Platform")
	}
}

func TestCreateTeamMissingName(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{"description": "no name"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/teams without name: want 400, got %d", rr.Code)
	}
}

func TestGetTeam(t *testing.T) {
	router, _ := newTestRouter(t)

	// Create a team.
	createBody := map[string]string{"name": "SRE"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/teams", createBody, testToken)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("setup: create team got %d", createRR.Code)
	}
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	// Get the team.
	rr := doRequest(t, router, http.MethodGet, "/api/v1/teams/"+id, nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/teams/%s: want 200, got %d", id, rr.Code)
	}
	var team map[string]any
	decodeBody(t, rr, &team)
	if team["name"] != "SRE" {
		t.Fatalf("name = %q, want %q", team["name"], "SRE")
	}
}

func TestGetTeamNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/teams/nonexistent", nil, testToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/teams/nonexistent: want 404, got %d", rr.Code)
	}
}

func TestListTeams(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/teams", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/teams: want 200, got %d", rr.Code)
	}
}

func TestUpdateTeam(t *testing.T) {
	router, _ := newTestRouter(t)

	// Create a team.
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/teams", map[string]string{"name": "Old"}, testToken)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("setup: got %d", createRR.Code)
	}
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	// Update.
	rr := doRequest(t, router, http.MethodPut, "/api/v1/teams/"+id, map[string]string{"name": "New"}, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/teams/%s: want 200, got %d; body: %s", id, rr.Code, rr.Body.String())
	}
}

func TestDeleteTeam(t *testing.T) {
	router, _ := newTestRouter(t)

	// Create a team.
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/teams", map[string]string{"name": "ToDelete"}, testToken)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("setup: got %d", createRR.Code)
	}
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodDelete, "/api/v1/teams/"+id, nil, testToken)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/v1/teams/%s: want 204, got %d", id, rr.Code)
	}

	// Verify it's gone.
	getRR := doRequest(t, router, http.MethodGet, "/api/v1/teams/"+id, nil, testToken)
	if getRR.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE: want 404, got %d", getRR.Code)
	}
}

// --- Team membership API tests ---

func createTestTeam(t *testing.T, router http.Handler) string {
	t.Helper()
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams", map[string]string{"name": "TestTeam-" + t.Name()}, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create team got %d; body: %s", rr.Code, rr.Body.String())
	}
	var team map[string]any
	decodeBody(t, rr, &team)
	return team["id"].(string)
}

func createTestUser(t *testing.T, router http.Handler) string {
	t.Helper()
	body := map[string]string{"name": "User-" + t.Name(), "email": t.Name() + "@example.com"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/users", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create user got %d; body: %s", rr.Code, rr.Body.String())
	}
	var user map[string]any
	decodeBody(t, rr, &user)
	return user["id"].(string)
}

func TestAddTeamMember(t *testing.T) {
	router, _ := newTestRouter(t)
	teamID := createTestTeam(t, router)
	userID := createTestUser(t, router)

	body := map[string]string{"user_id": userID, "role": "admin"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams/"+teamID+"/members", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST members: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var member map[string]any
	decodeBody(t, rr, &member)
	if member["role"] != "admin" {
		t.Errorf("role = %q, want %q", member["role"], "admin")
	}
}

func TestAddTeamMemberDefaultRole(t *testing.T) {
	router, _ := newTestRouter(t)
	teamID := createTestTeam(t, router)
	userID := createTestUser(t, router)

	// Omit role — should default to "member".
	body := map[string]string{"user_id": userID}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams/"+teamID+"/members", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST members: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var member map[string]any
	decodeBody(t, rr, &member)
	if member["role"] != "member" {
		t.Errorf("role = %q, want %q", member["role"], "member")
	}
}

func TestAddTeamMemberInvalidRole(t *testing.T) {
	router, _ := newTestRouter(t)
	teamID := createTestTeam(t, router)
	userID := createTestUser(t, router)

	body := map[string]string{"user_id": userID, "role": "superuser"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams/"+teamID+"/members", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST members with invalid role: want 400, got %d", rr.Code)
	}
}

func TestAddTeamMemberMissingUserID(t *testing.T) {
	router, _ := newTestRouter(t)
	teamID := createTestTeam(t, router)

	body := map[string]string{"role": "member"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/teams/"+teamID+"/members", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST members without user_id: want 400, got %d", rr.Code)
	}
}

func TestListTeamMembers(t *testing.T) {
	router, _ := newTestRouter(t)
	teamID := createTestTeam(t, router)
	userID := createTestUser(t, router)

	// Add member.
	addBody := map[string]string{"user_id": userID, "role": "member"}
	doRequest(t, router, http.MethodPost, "/api/v1/teams/"+teamID+"/members", addBody, testToken)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/teams/"+teamID+"/members", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET members: want 200, got %d", rr.Code)
	}
	var members []map[string]any
	decodeBody(t, rr, &members)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
}

func TestRemoveTeamMember(t *testing.T) {
	router, _ := newTestRouter(t)
	teamID := createTestTeam(t, router)
	userID := createTestUser(t, router)

	// Add then remove.
	addBody := map[string]string{"user_id": userID, "role": "member"}
	doRequest(t, router, http.MethodPost, "/api/v1/teams/"+teamID+"/members", addBody, testToken)

	rr := doRequest(t, router, http.MethodDelete, "/api/v1/teams/"+teamID+"/members/"+userID, nil, testToken)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE member: want 204, got %d", rr.Code)
	}

	// Verify empty.
	listRR := doRequest(t, router, http.MethodGet, "/api/v1/teams/"+teamID+"/members", nil, testToken)
	var members []map[string]any
	decodeBody(t, listRR, &members)
	if len(members) != 0 {
		t.Fatalf("expected 0 members after remove, got %d", len(members))
	}
}

func TestTeamsRequireAuth(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/teams", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/teams without auth: want 401, got %d", rr.Code)
	}
}

// --- Routing rule API tests ---

func createTestEP(t *testing.T, router http.Handler) string {
	t.Helper()
	body := map[string]any{"name": "EP-" + t.Name(), "repeat": 0}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/escalation-policies", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup: create EP got %d; body: %s", rr.Code, rr.Body.String())
	}
	var ep map[string]any
	decodeBody(t, rr, &ep)
	return ep["id"].(string)
}

func TestCreateRoutingRule(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "contains",
		"condition_value":      "database",
		"escalation_policy_id": epID,
		"priority":             0,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST routing-rules: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var rule map[string]any
	decodeBody(t, rr, &rule)
	if rule["id"] == nil || rule["id"] == "" {
		t.Fatal("response missing id")
	}
	if rule["condition_field"] != "summary" {
		t.Errorf("condition_field = %q, want %q", rule["condition_field"], "summary")
	}
}

func TestCreateRoutingRuleInvalidField(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	body := map[string]any{
		"condition_field":      "invalid",
		"condition_match_type": "contains",
		"condition_value":      "test",
		"escalation_policy_id": epID,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST routing-rules invalid field: want 400, got %d", rr.Code)
	}
}

func TestCreateRoutingRuleInvalidMatchType(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "exact",
		"condition_value":      "test",
		"escalation_policy_id": epID,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST routing-rules invalid match type: want 400, got %d", rr.Code)
	}
}

func TestCreateRoutingRuleMissingValue(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "contains",
		"condition_value":      "",
		"escalation_policy_id": epID,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST routing-rules missing value: want 400, got %d", rr.Code)
	}
}

func TestCreateRoutingRuleMissingEP(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)

	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "contains",
		"condition_value":      "test",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST routing-rules missing EP: want 400, got %d", rr.Code)
	}
}

func TestListRoutingRules(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	// Empty list.
	rr := doRequest(t, router, http.MethodGet, "/api/v1/services/"+svcID+"/routing-rules", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET routing-rules: want 200, got %d", rr.Code)
	}

	// Create a rule then list again.
	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "contains",
		"condition_value":      "test",
		"escalation_policy_id": epID,
	}
	doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)

	rr = doRequest(t, router, http.MethodGet, "/api/v1/services/"+svcID+"/routing-rules", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET routing-rules: want 200, got %d", rr.Code)
	}
	var rules []map[string]any
	decodeBody(t, rr, &rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestDeleteRoutingRule(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	// Create a rule.
	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "contains",
		"condition_value":      "test",
		"escalation_policy_id": epID,
	}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	var rule map[string]any
	decodeBody(t, createRR, &rule)
	ruleID := rule["id"].(string)

	// Delete it.
	rr := doRequest(t, router, http.MethodDelete, "/api/v1/services/"+svcID+"/routing-rules/"+ruleID, nil, testToken)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE routing-rule: want 204, got %d", rr.Code)
	}

	// Verify empty.
	listRR := doRequest(t, router, http.MethodGet, "/api/v1/services/"+svcID+"/routing-rules", nil, testToken)
	var rules []map[string]any
	decodeBody(t, listRR, &rules)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestCreateRoutingRuleInvalidRegex(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "regex",
		"condition_value":      "[invalid(regex",
		"escalation_policy_id": epID,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST routing-rules with invalid regex: want 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateRoutingRuleValueTooLong(t *testing.T) {
	router, _ := newTestRouter(t)
	svcID := createTestService(t, router)
	epID := createTestEP(t, router)

	body := map[string]any{
		"condition_field":      "summary",
		"condition_match_type": "contains",
		"condition_value":      strings.Repeat("x", 1025),
		"escalation_policy_id": epID,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/services/"+svcID+"/routing-rules", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST routing-rules with long value: want 400, got %d", rr.Code)
	}
}

// --- Alert API tests ---

func TestCreateAlert(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	body := map[string]string{
		"service_id": svc.ID,
		"summary":    "disk full",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/alerts: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var alert map[string]any
	decodeBody(t, rr, &alert)
	if alert["id"] == nil || alert["id"] == "" {
		t.Fatal("response missing id")
	}
	if alert["status"] != "triggered" {
		t.Errorf("status = %v, want triggered", alert["status"])
	}
}

func TestCreateAlertMissingFields(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{"summary": "no service"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST alert without service_id: want 400, got %d", rr.Code)
	}
}

func TestGetAlert(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	body := map[string]string{"service_id": svc.ID, "summary": "test"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/alerts/"+id, nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/alerts/%s: want 200, got %d", id, rr.Code)
	}
	var alert map[string]any
	decodeBody(t, rr, &alert)
	if alert["summary"] != "test" {
		t.Errorf("summary = %v, want test", alert["summary"])
	}
}

func TestGetAlertNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/alerts/nonexistent", nil, testToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/alerts/nonexistent: want 404, got %d", rr.Code)
	}
}

func TestListAlerts(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	for i := 0; i < 3; i++ {
		body := map[string]string{"service_id": svc.ID, "summary": "alert"}
		doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/v1/alerts", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/alerts: want 200, got %d", rr.Code)
	}
	var alerts []map[string]any
	decodeBody(t, rr, &alerts)
	if len(alerts) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(alerts))
	}
}

func TestListAlertsFilterByStatus(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	body := map[string]string{"service_id": svc.ID, "summary": "alert"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)

	// Resolve it
	doRequest(t, router, http.MethodPost, "/api/v1/alerts/"+created["id"].(string)+"/resolve", map[string]string{}, testToken)

	// Create another (stays triggered)
	doRequest(t, router, http.MethodPost, "/api/v1/alerts", map[string]string{"service_id": svc.ID, "summary": "alert2"}, testToken)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/alerts?status=triggered", nil, testToken)
	var alerts []map[string]any
	decodeBody(t, rr, &alerts)
	if len(alerts) != 1 {
		t.Errorf("expected 1 triggered alert, got %d", len(alerts))
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)
	u := &store.User{Name: "Acker", Email: "acker@test.com", Role: "user", Timezone: "UTC"}
	s.Users().Create(ctx, u)

	body := map[string]string{"service_id": svc.ID, "summary": "ack-me"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodPost, "/api/v1/alerts/"+id+"/acknowledge", map[string]string{"user_id": u.ID}, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST acknowledge: want 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	// Verify status
	getRR := doRequest(t, router, http.MethodGet, "/api/v1/alerts/"+id, nil, testToken)
	var alert map[string]any
	decodeBody(t, getRR, &alert)
	if alert["status"] != "acknowledged" {
		t.Errorf("status = %v, want acknowledged", alert["status"])
	}
}

func TestResolveAlert(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	body := map[string]string{"service_id": svc.ID, "summary": "resolve-me"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodPost, "/api/v1/alerts/"+id+"/resolve", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST resolve: want 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	getRR := doRequest(t, router, http.MethodGet, "/api/v1/alerts/"+id, nil, testToken)
	var alert map[string]any
	decodeBody(t, getRR, &alert)
	if alert["status"] != "resolved" {
		t.Errorf("status = %v, want resolved", alert["status"])
	}
}

func TestListAlertLogs(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	body := map[string]string{"service_id": svc.ID, "summary": "log-test"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/alerts/"+id+"/logs", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET alert logs: want 200, got %d", rr.Code)
	}
	var logs []map[string]any
	decodeBody(t, rr, &logs)
	if len(logs) < 1 {
		t.Error("expected at least 1 log entry (created)")
	}
}

func TestCreateAlertWithGroupKey(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	ep := &store.EscalationPolicy{Name: "EP"}
	s.EscalationPolicies().Create(ctx, ep)
	svc := &store.Service{Name: "Svc", EscalationPolicyID: ep.ID}
	s.Services().Create(ctx, svc)

	// First alert in group
	body := map[string]string{"service_id": svc.ID, "summary": "cpu high host-1", "group_key": "cpu-high"}
	rr1 := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body, testToken)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first alert: want 201, got %d", rr1.Code)
	}

	// Second alert in same group
	body2 := map[string]string{"service_id": svc.ID, "summary": "cpu high host-2", "group_key": "cpu-high"}
	rr2 := doRequest(t, router, http.MethodPost, "/api/v1/alerts", body2, testToken)
	if rr2.Code != http.StatusCreated {
		t.Fatalf("second alert: want 201, got %d", rr2.Code)
	}

	var a1, a2 map[string]any
	decodeBody(t, rr1, &a1)
	decodeBody(t, rr2, &a2)

	// Different IDs (not dedup)
	if a1["id"] == a2["id"] {
		t.Error("grouped alerts should have different IDs")
	}

	// Filter by group_key
	rr := doRequest(t, router, http.MethodGet, "/api/v1/alerts?group_key=cpu-high", nil, testToken)
	var alerts []map[string]any
	decodeBody(t, rr, &alerts)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts with group_key=cpu-high, got %d", len(alerts))
	}
}

// --- Incident API tests ---

func TestCreateIncident(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{"title": "Major outage", "severity": "critical"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/incidents", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/incidents: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var inc map[string]any
	decodeBody(t, rr, &inc)
	if inc["id"] == nil || inc["id"] == "" {
		t.Fatal("response missing id")
	}
	if inc["status"] != "triggered" {
		t.Errorf("status = %v, want triggered", inc["status"])
	}
}

func TestCreateIncidentMissingTitle(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{"severity": "critical"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/incidents", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST incident without title: want 400, got %d", rr.Code)
	}
}

func TestGetIncident(t *testing.T) {
	router, _ := newTestRouter(t)
	body := map[string]string{"title": "Outage"}
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/incidents", body, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/incidents/"+id, nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/incidents/%s: want 200, got %d", id, rr.Code)
	}
}

func TestGetIncidentNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	rr := doRequest(t, router, http.MethodGet, "/api/v1/incidents/nonexistent", nil, testToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/incidents/nonexistent: want 404, got %d", rr.Code)
	}
}

func TestListIncidents(t *testing.T) {
	router, _ := newTestRouter(t)
	doRequest(t, router, http.MethodPost, "/api/v1/incidents", map[string]string{"title": "Inc 1"}, testToken)
	doRequest(t, router, http.MethodPost, "/api/v1/incidents", map[string]string{"title": "Inc 2"}, testToken)

	rr := doRequest(t, router, http.MethodGet, "/api/v1/incidents", nil, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/incidents: want 200, got %d", rr.Code)
	}
	var incidents []map[string]any
	decodeBody(t, rr, &incidents)
	if len(incidents) < 2 {
		t.Errorf("expected at least 2 incidents, got %d", len(incidents))
	}
}

func TestUpdateIncident(t *testing.T) {
	router, _ := newTestRouter(t)
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/incidents", map[string]string{"title": "Outage"}, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	rr := doRequest(t, router, http.MethodPut, "/api/v1/incidents/"+id, map[string]string{
		"title": "Outage Updated", "status": "investigating", "severity": "major",
	}, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/incidents/%s: want 200, got %d; body: %s", id, rr.Code, rr.Body.String())
	}
}

func TestCreateIncidentUpdate(t *testing.T) {
	router, _ := newTestRouter(t)
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/incidents", map[string]string{"title": "Outage"}, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	body := map[string]string{"status": "investigating", "message": "Looking into it"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/incidents/"+id+"/updates", body, testToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST incident update: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}

	// List updates
	listRR := doRequest(t, router, http.MethodGet, "/api/v1/incidents/"+id+"/updates", nil, testToken)
	if listRR.Code != http.StatusOK {
		t.Fatalf("GET incident updates: want 200, got %d", listRR.Code)
	}
	var updates []map[string]any
	decodeBody(t, listRR, &updates)
	if len(updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(updates))
	}
}

func TestCreateIncidentUpdateMissingFields(t *testing.T) {
	router, _ := newTestRouter(t)
	createRR := doRequest(t, router, http.MethodPost, "/api/v1/incidents", map[string]string{"title": "Outage"}, testToken)
	var created map[string]any
	decodeBody(t, createRR, &created)
	id := created["id"].(string)

	// Missing message
	body := map[string]string{"status": "investigating"}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/incidents/"+id+"/updates", body, testToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST incident update without message: want 400, got %d", rr.Code)
	}
}

func TestRoutingIntegration(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	// Create two escalation policies.
	epDefault := &store.EscalationPolicy{Name: "Default EP"}
	if err := s.EscalationPolicies().Create(ctx, epDefault); err != nil {
		t.Fatal(err)
	}
	epDB := &store.EscalationPolicy{Name: "Database EP"}
	if err := s.EscalationPolicies().Create(ctx, epDB); err != nil {
		t.Fatal(err)
	}

	// Create service with default EP.
	svc := &store.Service{Name: "API", EscalationPolicyID: epDefault.ID}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatal(err)
	}

	// Create integration key.
	ik := &store.IntegrationKey{ServiceID: svc.ID, Name: "test", Type: "generic"}
	if err := s.Services().CreateIntegrationKey(ctx, ik); err != nil {
		t.Fatal(err)
	}

	// Add routing rule: summary contains "database" → use DB EP.
	if err := s.Services().CreateRoutingRule(ctx, &store.RoutingRule{
		ServiceID:          svc.ID,
		Priority:           0,
		ConditionField:     "summary",
		ConditionMatchType: "contains",
		ConditionValue:     "database",
		EscalationPolicyID: epDB.ID,
	}); err != nil {
		t.Fatal(err)
	}

	// Send alert matching routing rule.
	body := map[string]string{
		"summary":   "database connection timeout",
		"details":   "pg pool exhausted",
		"dedup_key": "db-timeout-1",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/v1/integrations/"+ik.Secret+"/alerts", body, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST alert: want 201, got %d; body: %s", rr.Code, rr.Body.String())
	}
	var alert map[string]any
	decodeBody(t, rr, &alert)

	// Verify the alert used the DB escalation policy (not default).
	snapshot := alert["escalation_policy_snapshot"].(string)
	if !strings.Contains(snapshot, epDB.ID) {
		t.Errorf("alert should use DB EP (%s), snapshot: %s", epDB.ID, snapshot)
	}
	if strings.Contains(snapshot, epDefault.ID) {
		t.Errorf("alert should NOT use default EP (%s), snapshot: %s", epDefault.ID, snapshot)
	}

	// Send alert NOT matching any rule — should use default EP.
	body2 := map[string]string{
		"summary":   "high CPU on web-01",
		"details":   "cpu at 95%",
		"dedup_key": "cpu-web-1",
	}
	rr2 := doRequest(t, router, http.MethodPost, "/api/v1/integrations/"+ik.Secret+"/alerts", body2, "")
	if rr2.Code != http.StatusCreated {
		t.Fatalf("POST alert 2: want 201, got %d; body: %s", rr2.Code, rr2.Body.String())
	}
	var alert2 map[string]any
	decodeBody(t, rr2, &alert2)

	snapshot2 := alert2["escalation_policy_snapshot"].(string)
	if !strings.Contains(snapshot2, epDefault.ID) {
		t.Errorf("alert2 should use default EP (%s), snapshot: %s", epDefault.ID, snapshot2)
	}
}
