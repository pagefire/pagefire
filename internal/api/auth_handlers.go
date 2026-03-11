package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/store"
)

// AuthHandler handles authentication endpoints (login, logout, me, setup).
type AuthHandler struct {
	authSvc *auth.Service
	users   store.UserStore
}

func NewAuthHandler(authSvc *auth.Service, users store.UserStore) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, users: users}
}

// Routes returns all auth routes. Public routes (login, setup) have no auth.
// Protected routes (me, logout, tokens) require SessionOrTokenAuth.
func (h *AuthHandler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// Public (no auth)
	loginLimiter := NewRateLimiter(10, time.Minute)
	r.With(RateLimitMiddleware(loginLimiter)).Post("/login", h.login)
	r.Get("/setup", h.setupCheck)
	r.Post("/setup", h.setup)
	r.Get("/invite/{token}", h.inviteCheck)
	r.Post("/invite/{token}", h.inviteAccept)

	// Protected (requires auth)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/me", h.me)
		r.Post("/logout", h.logout)
		r.Put("/password", h.changePassword)
		r.Post("/tokens", h.createToken)
		r.Get("/tokens", h.listTokens)
		r.Delete("/tokens/{id}", h.revokeToken)
	})

	return r
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.authSvc.Logout(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "logout failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// setupCheck returns whether initial setup is needed (no users exist yet).
func (h *AuthHandler) setupCheck(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setup_required": count == 0})
}

// setup creates the first admin user. Only works when no users exist.
func (h *AuthHandler) setup(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "name, email, and password required")
		return
	}
	if !validateEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user := &store.User{
		Name:         req.Name,
		Email:        req.Email,
		Role:         store.RoleAdmin,
		Timezone:     "UTC",
		PasswordHash: hash,
		IsActive:     true,
	}
	if err := h.users.Create(r.Context(), user); err != nil {
		handleStoreError(w, err)
		return
	}

	// Auto-login the newly created admin
	loggedIn, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		// User was created but login failed — still return the user
		writeJSON(w, http.StatusCreated, user)
		return
	}

	writeJSON(w, http.StatusCreated, loggedIn)
}

func (h *AuthHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password required")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	// Verify current password
	fullUser, err := h.users.Get(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	match, err := auth.CheckPassword(req.CurrentPassword, fullUser.PasswordHash)
	if err != nil || !match {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := h.users.SetPassword(r.Context(), user.ID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (h *AuthHandler) createToken(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	rawToken, token, err := h.authSvc.GenerateAPIToken(r.Context(), user.ID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token": rawToken, // shown once
		"id":    token.ID,
		"name":  token.Name,
	})
}

func (h *AuthHandler) listTokens(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	tokens, err := h.users.ListAPITokens(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	if tokens == nil {
		tokens = []store.APIToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) revokeToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.users.RevokeAPIToken(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// inviteCheck validates an invite token and returns the user info (no auth required).
func (h *AuthHandler) inviteCheck(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	invite, err := h.users.GetInviteTokenByHash(r.Context(), tokenHash)
	if err != nil {
		writeError(w, http.StatusNotFound, "invalid or expired invite link")
		return
	}
	if invite.UsedAt != nil {
		writeError(w, http.StatusGone, "this invite has already been used")
		return
	}
	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusGone, "this invite has expired")
		return
	}

	user, err := h.users.Get(r.Context(), invite.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"name":  user.Name,
		"email": user.Email,
	})
}

// inviteAccept sets the user's password and marks the invite as used.
func (h *AuthHandler) inviteAccept(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	invite, err := h.users.GetInviteTokenByHash(r.Context(), tokenHash)
	if err != nil {
		writeError(w, http.StatusNotFound, "invalid or expired invite link")
		return
	}
	if invite.UsedAt != nil {
		writeError(w, http.StatusGone, "this invite has already been used")
		return
	}
	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusGone, "this invite has expired")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.users.SetPassword(r.Context(), invite.UserID, passwordHash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set password")
		return
	}
	if err := h.users.UseInviteToken(r.Context(), invite.ID); err != nil {
		// Password was set but token mark failed — not critical
		_ = err
	}

	// Auto-login
	user, err := h.users.Get(r.Context(), invite.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	loggedIn, err := h.authSvc.Login(r.Context(), user.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "password set, please log in"})
		return
	}
	writeJSON(w, http.StatusOK, loggedIn)
}
