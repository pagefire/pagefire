package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/auth"
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
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Timezone string `json:"timezone"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "name and email are required")
		return
	}
	if !validateEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	if req.Password != "" && len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	role := store.RoleUser
	if req.Role == store.RoleAdmin {
		caller := UserFromContext(r.Context())
		if caller != nil && caller.Role == store.RoleAdmin {
			role = store.RoleAdmin
		}
	}

	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if !validateTimezone(tz) {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}

	var passwordHash string
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		passwordHash = hash
	}

	u := &store.User{
		Name:         req.Name,
		Email:        req.Email,
		Role:         role,
		Timezone:     tz,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	if err := h.users.Create(r.Context(), u); err != nil {
		handleStoreError(w, err)
		return
	}

	// If no password provided, generate an invite token
	resp := map[string]any{
		"id":       u.ID,
		"name":     u.Name,
		"email":    u.Email,
		"role":     u.Role,
		"timezone": u.Timezone,
	}
	if passwordHash == "" {
		rawToken, inviteURL, err := h.generateInvite(r, u.ID)
		if err != nil {
			// User was created but invite failed — still return user with error hint
			resp["invite_error"] = "failed to generate invite link"
			writeJSON(w, http.StatusCreated, resp)
			return
		}
		_ = rawToken
		resp["invite_url"] = inviteURL
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *UserHandler) generateInvite(r *http.Request, userID string) (string, string, error) {
	// Generate 32 random bytes → hex-encoded token
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	rawToken := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	invite := &store.InviteToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour), // 7 days
	}
	if err := h.users.CreateInviteToken(r.Context(), invite); err != nil {
		return "", "", err
	}

	// Build invite URL from request host
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	inviteURL := fmt.Sprintf("%s://%s/invite/%s", scheme, r.Host, rawToken)
	return rawToken, inviteURL, nil
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
	// Preserve existing role — role changes not allowed via this endpoint
	existing, err := h.users.Get(r.Context(), u.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	u.Role = existing.Role
	if err := h.users.Update(r.Context(), &u); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *UserHandler) delete(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")

	// Prevent self-deletion
	caller := UserFromContext(r.Context())
	if caller != nil && caller.ID == targetID {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	if err := h.users.Delete(r.Context(), targetID); err != nil {
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
		// Basic URL format validation only. SSRF protection is enforced
		// at send time by the webhook provider, not at registration time.
		u, err := url.Parse(cm.Value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			writeError(w, http.StatusBadRequest, "invalid webhook URL")
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
