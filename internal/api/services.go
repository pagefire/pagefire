package api

import (
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type ServiceHandler struct {
	services store.ServiceStore
}

func NewServiceHandler(services store.ServiceStore) *ServiceHandler {
	return &ServiceHandler{services: services}
}

func (h *ServiceHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Get("/{id}/integration-keys", h.listIntegrationKeys)
	r.Post("/{id}/integration-keys", h.createIntegrationKey)
	r.Delete("/{id}/integration-keys/{keyID}", h.deleteIntegrationKey)

	r.Get("/{id}/routing-rules", h.listRoutingRules)
	r.Post("/{id}/routing-rules", h.createRoutingRule)
	r.Delete("/{id}/routing-rules/{ruleID}", h.deleteRoutingRule)

	return r
}

func (h *ServiceHandler) create(w http.ResponseWriter, r *http.Request) {
	var svc store.Service
	if err := decodeJSON(w, r, &svc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if svc.Name == "" || svc.EscalationPolicyID == "" {
		writeError(w, http.StatusBadRequest, "name and escalation_policy_id are required")
		return
	}
	if err := h.services.Create(r.Context(), &svc); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) get(w http.ResponseWriter, r *http.Request) {
	svc, err := h.services.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) list(w http.ResponseWriter, r *http.Request) {
	var services []store.Service
	var err error
	if teamID := r.URL.Query().Get("team_id"); teamID != "" {
		services, err = h.services.ListByTeam(r.Context(), teamID)
	} else {
		services, err = h.services.List(r.Context())
	}
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services)
}

func (h *ServiceHandler) update(w http.ResponseWriter, r *http.Request) {
	var svc store.Service
	if err := decodeJSON(w, r, &svc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	svc.ID = chi.URLParam(r, "id")
	if err := h.services.Update(r.Context(), &svc); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.services.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// integrationKeyResponse omits the secret field for list responses.
type integrationKeyResponse struct {
	ID           string `json:"id"`
	ServiceID    string `json:"service_id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	SecretPrefix string `json:"secret_prefix"`
	CreatedAt    string `json:"created_at"`
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return "****" + secret[len(secret)-4:]
}

func (h *ServiceHandler) listIntegrationKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.services.ListIntegrationKeys(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	// Never return full secrets in list responses
	resp := make([]integrationKeyResponse, len(keys))
	for i, k := range keys {
		resp[i] = integrationKeyResponse{
			ID:           k.ID,
			ServiceID:    k.ServiceID,
			Name:         k.Name,
			Type:         k.Type,
			SecretPrefix: maskSecret(k.Secret),
			CreatedAt:    k.CreatedAt.String(),
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ServiceHandler) createIntegrationKey(w http.ResponseWriter, r *http.Request) {
	var ik store.IntegrationKey
	if err := decodeJSON(w, r, &ik); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ik.ServiceID = chi.URLParam(r, "id")
	if ik.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if ik.Type == "" {
		ik.Type = "generic"
	}
	// Always generate server-side secret, ignore client input
	ik.Secret = ""
	if err := h.services.CreateIntegrationKey(r.Context(), &ik); err != nil {
		handleStoreError(w, err)
		return
	}
	// Return full secret ONLY on creation (one-time view)
	writeJSON(w, http.StatusCreated, ik)
}

func (h *ServiceHandler) deleteIntegrationKey(w http.ResponseWriter, r *http.Request) {
	if err := h.services.DeleteIntegrationKey(r.Context(), chi.URLParam(r, "keyID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Routing rules ---

var validConditionFields = map[string]bool{"summary": true, "details": true, "source": true}
var validConditionMatchTypes = map[string]bool{"contains": true, "regex": true}

func (h *ServiceHandler) listRoutingRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.services.ListRoutingRules(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *ServiceHandler) createRoutingRule(w http.ResponseWriter, r *http.Request) {
	var rule store.RoutingRule
	if err := decodeJSON(w, r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule.ServiceID = chi.URLParam(r, "id")
	if !validConditionFields[rule.ConditionField] {
		writeError(w, http.StatusBadRequest, "condition_field must be summary, details, or source")
		return
	}
	if !validConditionMatchTypes[rule.ConditionMatchType] {
		writeError(w, http.StatusBadRequest, "condition_match_type must be contains or regex")
		return
	}
	if rule.ConditionValue == "" {
		writeError(w, http.StatusBadRequest, "condition_value is required")
		return
	}
	if len(rule.ConditionValue) > 1024 {
		writeError(w, http.StatusBadRequest, "condition_value must be 1024 characters or fewer")
		return
	}
	if rule.ConditionMatchType == "regex" {
		if _, err := regexp.Compile(rule.ConditionValue); err != nil {
			writeError(w, http.StatusBadRequest, "condition_value is not a valid regex pattern")
			return
		}
	}
	if rule.EscalationPolicyID == "" {
		writeError(w, http.StatusBadRequest, "escalation_policy_id is required")
		return
	}
	if err := h.services.CreateRoutingRule(r.Context(), &rule); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *ServiceHandler) deleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	if err := h.services.DeleteRoutingRule(r.Context(), chi.URLParam(r, "ruleID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

