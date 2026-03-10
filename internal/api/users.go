package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type UserHandler struct {
	users store.UserStore
}

func NewUserHandler(users store.UserStore) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Get("/{id}/contact-methods", h.listContactMethods)
	r.Post("/{id}/contact-methods", h.createContactMethod)
	r.Delete("/{id}/contact-methods/{cmID}", h.deleteContactMethod)

	r.Get("/{id}/notification-rules", h.listNotificationRules)
	r.Post("/{id}/notification-rules", h.createNotificationRule)
	r.Delete("/{id}/notification-rules/{ruleID}", h.deleteNotificationRule)

	return r
}

func (h *UserHandler) create(w http.ResponseWriter, r *http.Request) {
	var u store.User
	if err := decodeJSON(w, r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if u.Name == "" || u.Email == "" {
		writeError(w, http.StatusBadRequest, "name and email are required")
		return
	}
	if !validateEmail(u.Email) {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	// Server enforces role — ignore client input
	u.Role = "user"
	if u.Timezone == "" {
		u.Timezone = "UTC"
	}
	if !validateTimezone(u.Timezone) {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	if err := h.users.Create(r.Context(), &u); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *UserHandler) get(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *UserHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request) {
	var u store.User
	if err := decodeJSON(w, r, &u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u.ID = chi.URLParam(r, "id")
	if u.Email != "" && !validateEmail(u.Email) {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	if u.Timezone != "" && !validateTimezone(u.Timezone) {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	// Don't allow role changes via update — admin-only endpoint needed
	u.Role = ""
	if err := h.users.Update(r.Context(), &u); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *UserHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.users.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) listContactMethods(w http.ResponseWriter, r *http.Request) {
	methods, err := h.users.ListContactMethods(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, methods)
}

func (h *UserHandler) createContactMethod(w http.ResponseWriter, r *http.Request) {
	var cm store.ContactMethod
	if err := decodeJSON(w, r, &cm); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cm.UserID = chi.URLParam(r, "id")
	if cm.Type == "" || cm.Value == "" {
		writeError(w, http.StatusBadRequest, "type and value are required")
		return
	}
	// Validate contact method value by type
	switch cm.Type {
	case "email":
		if !validateEmail(cm.Value) {
			writeError(w, http.StatusBadRequest, "invalid email address")
			return
		}
	case "webhook":
		if err := validateWebhookURL(cm.Value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "slack_dm":
		if cm.Value == "" {
			writeError(w, http.StatusBadRequest, "slack user ID required")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported contact method type")
		return
	}
	if err := h.users.CreateContactMethod(r.Context(), &cm); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cm)
}

func (h *UserHandler) deleteContactMethod(w http.ResponseWriter, r *http.Request) {
	if err := h.users.DeleteContactMethod(r.Context(), chi.URLParam(r, "cmID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) listNotificationRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.users.ListNotificationRules(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *UserHandler) createNotificationRule(w http.ResponseWriter, r *http.Request) {
	var nr store.NotificationRule
	if err := decodeJSON(w, r, &nr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	nr.UserID = chi.URLParam(r, "id")
	if nr.ContactMethodID == "" {
		writeError(w, http.StatusBadRequest, "contact_method_id is required")
		return
	}
	if nr.DelayMinutes < 0 {
		writeError(w, http.StatusBadRequest, "delay_minutes must be non-negative")
		return
	}
	if err := h.users.CreateNotificationRule(r.Context(), &nr); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, nr)
}

func (h *UserHandler) deleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	if err := h.users.DeleteNotificationRule(r.Context(), chi.URLParam(r, "ruleID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
