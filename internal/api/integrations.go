package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type IntegrationHandler struct {
	services   store.ServiceStore
	alerts     store.AlertStore
	escalation store.EscalationPolicyStore
}

func NewIntegrationHandler(services store.ServiceStore, alerts store.AlertStore, escalation store.EscalationPolicyStore) *IntegrationHandler {
	return &IntegrationHandler{services: services, alerts: alerts, escalation: escalation}
}

func (h *IntegrationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{key}/alerts", h.genericWebhook)
	r.Post("/{key}/grafana", h.grafanaWebhook)
	r.Post("/{key}/prometheus", h.prometheusWebhook)
	return r
}

type genericWebhookRequest struct {
	Summary          string `json:"summary"`
	Details          string `json:"details"`
	DeduplicationKey string `json:"dedup_key"`
}

func (h *IntegrationHandler) genericWebhook(w http.ResponseWriter, r *http.Request) {
	ik, err := h.services.GetIntegrationKeyBySecret(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid integration key")
		return
	}

	var req genericWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Summary == "" {
		writeError(w, http.StatusBadRequest, "summary is required")
		return
	}

	alert, err := h.createAlertFromIntegration(r, ik, req.Summary, req.Details, req.DeduplicationKey)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, alert)
}

func (h *IntegrationHandler) grafanaWebhook(w http.ResponseWriter, r *http.Request) {
	ik, err := h.services.GetIntegrationKeyBySecret(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid integration key")
		return
	}

	var payload struct {
		Title   string `json:"title"`
		Message string `json:"message"`
		RuleID  string `json:"ruleId"`
		State   string `json:"state"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if payload.State == "ok" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
		return
	}

	alert, err := h.createAlertFromIntegration(r, ik, payload.Title, payload.Message, "grafana:"+payload.RuleID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, alert)
}

func (h *IntegrationHandler) prometheusWebhook(w http.ResponseWriter, r *http.Request) {
	ik, err := h.services.GetIntegrationKeyBySecret(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid integration key")
		return
	}

	var payload struct {
		Alerts []struct {
			Status      string            `json:"status"`
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
			Fingerprint string            `json:"fingerprint"`
		} `json:"alerts"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, a := range payload.Alerts {
		if a.Status == "resolved" {
			continue
		}

		summary := a.Annotations["summary"]
		if summary == "" {
			summary = a.Labels["alertname"]
		}

		h.createAlertFromIntegration(r, ik, summary, a.Annotations["description"], "prometheus:"+a.Fingerprint)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *IntegrationHandler) createAlertFromIntegration(r *http.Request, ik *store.IntegrationKey, summary, details, dedupKey string) (*store.Alert, error) {
	svc, err := h.services.Get(r.Context(), ik.ServiceID)
	if err != nil {
		return nil, err
	}

	snapshot, err := h.escalation.GetFullPolicy(r.Context(), svc.EscalationPolicyID)
	if err != nil {
		return nil, err
	}
	snapshotJSON, _ := json.Marshal(snapshot)

	now := time.Now()
	alert := &store.Alert{
		ServiceID:                ik.ServiceID,
		Status:                   store.AlertStatusTriggered,
		Summary:                  summary,
		Details:                  details,
		Source:                   "integration",
		DeduplicationKey:         dedupKey,
		EscalationPolicySnapshot: string(snapshotJSON),
		NextEscalationAt:         &now,
	}

	if err := h.alerts.Create(r.Context(), alert); err != nil {
		return nil, err
	}

	h.alerts.CreateLog(r.Context(), &store.AlertLog{
		AlertID: alert.ID,
		Event:   "created",
		Message: "Alert created via " + ik.Type + " integration",
	})

	return alert, nil
}
