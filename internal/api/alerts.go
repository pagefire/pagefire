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
	incidents  store.IncidentStore
}

func NewAlertHandler(alerts store.AlertStore, services store.ServiceStore, escalation store.EscalationPolicyStore, incidents ...store.IncidentStore) *AlertHandler {
	h := &AlertHandler{alerts: alerts, services: services, escalation: escalation}
	if len(incidents) > 0 {
		h.incidents = incidents[0]
	}
	return h
}

func (h *AlertHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/acknowledge", h.acknowledge)
	r.Post("/{id}/resolve", h.resolve)
	r.Get("/{id}/logs", h.listLogs)
	r.Get("/{id}/incident", h.getIncidentForAlert)
	return r
}

type createAlertRequest struct {
	ServiceID        string `json:"service_id"`
	Summary          string `json:"summary"`
	Details          string `json:"details"`
	DeduplicationKey string `json:"dedup_key"`
	GroupKey         string `json:"group_key"`
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
	if len(req.Summary) > 500 {
		writeError(w, http.StatusBadRequest, "summary must be 500 characters or fewer")
		return
	}
	if len(req.Details) > 10000 {
		writeError(w, http.StatusBadRequest, "details must be 10000 characters or fewer")
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
		Status:                   store.AlertStatusTriggered,
		Summary:                  req.Summary,
		Details:                  req.Details,
		Source:                   "api",
		DeduplicationKey:         req.DeduplicationKey,
		GroupKey:                 req.GroupKey,
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
	search := r.URL.Query().Get("search")
	if len(search) > 200 {
		search = search[:200]
	}
	filter := store.AlertFilter{
		Status:    r.URL.Query().Get("status"),
		ServiceID: r.URL.Query().Get("service_id"),
		GroupKey:  r.URL.Query().Get("group_key"),
		Source:    r.URL.Query().Get("source"),
		Search:    search,
		Limit:     limit,
		Offset:    offset,
	}

	if v := r.URL.Query().Get("created_after"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.CreatedAfter = &t
		} else {
			writeError(w, http.StatusBadRequest, "created_after must be RFC3339 format")
			return
		}
	}
	if v := r.URL.Query().Get("created_before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.CreatedBefore = &t
		} else {
			writeError(w, http.StatusBadRequest, "created_before must be RFC3339 format")
			return
		}
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
	// Body is optional — fall back to session user
	_ = decodeJSON(w, r, &req)

	userID := req.UserID
	if userID == "" {
		if caller := UserFromContext(r.Context()); caller != nil {
			userID = caller.ID
		}
	}
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.alerts.Acknowledge(r.Context(), id, userID); err != nil {
		handleStoreError(w, err)
		return
	}

	h.alerts.CreateLog(r.Context(), &store.AlertLog{
		AlertID: id,
		Event:   "acknowledged",
		Message: "Alert acknowledged",
		UserID:  &userID,
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

func (h *AlertHandler) getIncidentForAlert(w http.ResponseWriter, r *http.Request) {
	if h.incidents == nil {
		writeError(w, http.StatusNotImplemented, "incident linking not available")
		return
	}
	inc, err := h.incidents.GetIncidentForAlert(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}
