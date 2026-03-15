package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type IncidentHandler struct {
	incidents store.IncidentStore
}

func NewIncidentHandler(incidents store.IncidentStore) *IncidentHandler {
	return &IncidentHandler{incidents: incidents}
}

func (h *IncidentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Post("/{id}/updates", h.createUpdate)
	r.Get("/{id}/updates", h.listUpdates)
	r.Post("/{id}/services", h.addService)
	r.Get("/{id}/services", h.listServices)
	r.Post("/{id}/alerts", h.linkAlert)
	r.Get("/{id}/alerts", h.listAlerts)
	r.Delete("/{id}/alerts/{alertID}", h.unlinkAlert)
	return r
}

func (h *IncidentHandler) create(w http.ResponseWriter, r *http.Request) {
	var inc store.Incident
	if err := decodeJSON(w, r, &inc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if inc.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if len(inc.Title) > 500 {
		writeError(w, http.StatusBadRequest, "title must be 500 characters or fewer")
		return
	}
	if len(inc.Summary) > 10000 {
		writeError(w, http.StatusBadRequest, "summary must be 10000 characters or fewer")
		return
	}
	if inc.Status == "" {
		inc.Status = store.IncidentStatusTriggered
	}
	if inc.Severity == "" {
		inc.Severity = store.SeverityCritical
	}
	if inc.Source == "" {
		inc.Source = "manual"
	}
	if caller := UserFromContext(r.Context()); caller != nil {
		inc.CreatedBy = caller.ID
	}
	if err := h.incidents.Create(r.Context(), &inc); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inc)
}

func (h *IncidentHandler) get(w http.ResponseWriter, r *http.Request) {
	inc, err := h.incidents.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (h *IncidentHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := parseListLimit(r)
	search := r.URL.Query().Get("search")
	if len(search) > 200 {
		search = search[:200]
	}
	filter := store.IncidentFilter{
		Status: r.URL.Query().Get("status"),
		Search: search,
		Limit:  limit,
		Offset: offset,
	}

	incidents, err := h.incidents.List(r.Context(), filter)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, incidents)
}

func (h *IncidentHandler) update(w http.ResponseWriter, r *http.Request) {
	var inc store.Incident
	if err := decodeJSON(w, r, &inc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	inc.ID = chi.URLParam(r, "id")
	if err := h.incidents.Update(r.Context(), &inc); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (h *IncidentHandler) createUpdate(w http.ResponseWriter, r *http.Request) {
	var u store.IncidentUpdate
	if err := decodeJSON(w, r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u.IncidentID = chi.URLParam(r, "id")
	if u.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Auto-populate created_by from authenticated user
	if caller := UserFromContext(r.Context()); caller != nil {
		u.CreatedBy = caller.ID
	}

	if err := h.incidents.CreateUpdate(r.Context(), &u); err != nil {
		handleStoreError(w, err)
		return
	}

	// If a status was provided, sync the incident's status
	if u.Status != "" {
		inc, err := h.incidents.Get(r.Context(), u.IncidentID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		inc.Status = u.Status
		if err := h.incidents.Update(r.Context(), inc); err != nil {
			handleStoreError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusCreated, u)
}

func (h *IncidentHandler) listUpdates(w http.ResponseWriter, r *http.Request) {
	updates, err := h.incidents.ListUpdates(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updates)
}

func (h *IncidentHandler) addService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ServiceID == "" {
		writeError(w, http.StatusBadRequest, "service_id is required")
		return
	}
	if err := h.incidents.AddService(r.Context(), chi.URLParam(r, "id"), req.ServiceID); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IncidentHandler) listServices(w http.ResponseWriter, r *http.Request) {
	ids, err := h.incidents.ListServices(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ids)
}

func (h *IncidentHandler) linkAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AlertID string `json:"alert_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AlertID == "" {
		writeError(w, http.StatusBadRequest, "alert_id is required")
		return
	}
	if err := h.incidents.LinkAlert(r.Context(), chi.URLParam(r, "id"), req.AlertID); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IncidentHandler) unlinkAlert(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "id")
	alertID := chi.URLParam(r, "alertID")
	if err := h.incidents.UnlinkAlert(r.Context(), incidentID, alertID); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IncidentHandler) listAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.incidents.ListAlerts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if alerts == nil {
		alerts = []*store.Alert{}
	}
	writeJSON(w, http.StatusOK, alerts)
}
