package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type AlertHandler struct {
	alerts     store.AlertStore
	services   store.ServiceStore
	escalation store.EscalationPolicyStore
}

func NewAlertHandler(alerts store.AlertStore, services store.ServiceStore, escalation store.EscalationPolicyStore) *AlertHandler {
	return &AlertHandler{alerts: alerts, services: services, escalation: escalation}
}

func (h *AlertHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/acknowledge", h.acknowledge)
	r.Post("/{id}/resolve", h.resolve)
	r.Get("/{id}/logs", h.listLogs)
	return r
}

type createAlertRequest struct {
	ServiceID        string `json:"service_id"`
	Summary          string `json:"summary"`
	Details          string `json:"details"`
	DeduplicationKey string `json:"dedup_key"`
}

func (h *AlertHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createAlertRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ServiceID == "" || req.Summary == "" {
		writeError(w, http.StatusBadRequest, "service_id and summary are required")
		return
	}

	svc, err := h.services.Get(r.Context(), req.ServiceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	snapshot, err := h.escalation.GetFullPolicy(r.Context(), svc.EscalationPolicyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	now := time.Now()
	alert := &store.Alert{
		ServiceID:                req.ServiceID,
		Summary:                  req.Summary,
		Details:                  req.Details,
		Source:                   "api",
		DeduplicationKey:         req.DeduplicationKey,
		EscalationPolicySnapshot: string(snapshotJSON),
		NextEscalationAt:         &now,
	}

	if err := h.alerts.Create(r.Context(), alert); err != nil {
		handleStoreError(w, err)
		return
	}

	h.alerts.CreateLog(r.Context(), &store.AlertLog{
		AlertID: alert.ID,
		Event:   "created",
		Message: "Alert created via API",
	})

	writeJSON(w, http.StatusCreated, alert)
}

func (h *AlertHandler) get(w http.ResponseWriter, r *http.Request) {
	alert, err := h.alerts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alert)
}

func (h *AlertHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseListLimit(r)
	filter := store.AlertFilter{
		Status:    r.URL.Query().Get("status"),
		ServiceID: r.URL.Query().Get("service_id"),
		Limit:     limit,
		Offset:    offset,
	}

	alerts, err := h.alerts.List(r.Context(), filter)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (h *AlertHandler) acknowledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.alerts.Acknowledge(r.Context(), id, req.UserID); err != nil {
		handleStoreError(w, err)
		return
	}

	h.alerts.CreateLog(r.Context(), &store.AlertLog{
		AlertID: id,
		Event:   "acknowledged",
		Message: "Alert acknowledged",
		UserID:  req.UserID,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func (h *AlertHandler) resolve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.alerts.Resolve(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}

	h.alerts.CreateLog(r.Context(), &store.AlertLog{
		AlertID: id,
		Event:   "resolved",
		Message: "Alert resolved",
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (h *AlertHandler) listLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.alerts.ListLogs(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
