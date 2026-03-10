package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type TeamHandler struct {
	teams store.TeamStore
}

func NewTeamHandler(teams store.TeamStore) *TeamHandler {
	return &TeamHandler{teams: teams}
}

func (h *TeamHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Post("/{id}/members", h.addMember)
	r.Get("/{id}/members", h.listMembers)
	r.Delete("/{id}/members/{userID}", h.removeMember)

	return r
}

func (h *TeamHandler) create(w http.ResponseWriter, r *http.Request) {
	var t store.Team
	if err := decodeJSON(w, r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if t.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.teams.Create(r.Context(), &t); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *TeamHandler) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.teams.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TeamHandler) list(w http.ResponseWriter, r *http.Request) {
	teams, err := h.teams.List(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

func (h *TeamHandler) update(w http.ResponseWriter, r *http.Request) {
	var t store.Team
	if err := decodeJSON(w, r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t.ID = chi.URLParam(r, "id")
	if err := h.teams.Update(r.Context(), &t); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TeamHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.teams.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (h *TeamHandler) addMember(w http.ResponseWriter, r *http.Request) {
	var req addMemberRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	if req.Role != "admin" && req.Role != "member" {
		writeError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}
	teamID := chi.URLParam(r, "id")
	if err := h.teams.AddMember(r.Context(), teamID, req.UserID, req.Role); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, store.TeamMember{
		TeamID: teamID,
		UserID: req.UserID,
		Role:   req.Role,
	})
}

func (h *TeamHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.teams.ListMembers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *TeamHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	if err := h.teams.RemoveMember(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
