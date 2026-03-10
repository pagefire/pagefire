package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type ScheduleHandler struct {
	schedules store.ScheduleStore
}

func NewScheduleHandler(schedules store.ScheduleStore) *ScheduleHandler {
	return &ScheduleHandler{schedules: schedules}
}

func (h *ScheduleHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Get("/{id}/rotations", h.listRotations)
	r.Post("/{id}/rotations", h.createRotation)
	r.Delete("/{id}/rotations/{rotID}", h.deleteRotation)

	r.Get("/{id}/rotations/{rotID}/participants", h.listParticipants)
	r.Post("/{id}/rotations/{rotID}/participants", h.createParticipant)
	r.Delete("/{id}/rotations/{rotID}/participants/{partID}", h.deleteParticipant)

	r.Get("/{id}/overrides", h.listOverrides)
	r.Post("/{id}/overrides", h.createOverride)
	r.Delete("/{id}/overrides/{overrideID}", h.deleteOverride)

	return r
}

func (h *ScheduleHandler) create(w http.ResponseWriter, r *http.Request) {
	var sched store.Schedule
	if err := decodeJSON(w, r, &sched); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if sched.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if sched.Timezone == "" {
		sched.Timezone = "UTC"
	}
	if !validateTimezone(sched.Timezone) {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	if err := h.schedules.Create(r.Context(), &sched); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sched)
}

func (h *ScheduleHandler) get(w http.ResponseWriter, r *http.Request) {
	sched, err := h.schedules.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sched)
}

func (h *ScheduleHandler) list(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.schedules.List(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (h *ScheduleHandler) update(w http.ResponseWriter, r *http.Request) {
	var sched store.Schedule
	if err := decodeJSON(w, r, &sched); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sched.ID = chi.URLParam(r, "id")
	if sched.Timezone != "" && !validateTimezone(sched.Timezone) {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	if err := h.schedules.Update(r.Context(), &sched); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sched)
}

func (h *ScheduleHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.schedules.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) listRotations(w http.ResponseWriter, r *http.Request) {
	rotations, err := h.schedules.ListRotations(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rotations)
}

func (h *ScheduleHandler) createRotation(w http.ResponseWriter, r *http.Request) {
	var rot store.Rotation
	if err := decodeJSON(w, r, &rot); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rot.ScheduleID = chi.URLParam(r, "id")
	if rot.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if rot.Type == "" {
		rot.Type = store.RotationTypeWeekly
	}
	if rot.ShiftLength <= 0 {
		rot.ShiftLength = 1
	}
	if err := h.schedules.CreateRotation(r.Context(), &rot); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rot)
}

func (h *ScheduleHandler) deleteRotation(w http.ResponseWriter, r *http.Request) {
	if err := h.schedules.DeleteRotation(r.Context(), chi.URLParam(r, "rotID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) listParticipants(w http.ResponseWriter, r *http.Request) {
	participants, err := h.schedules.ListParticipants(r.Context(), chi.URLParam(r, "rotID"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, participants)
}

func (h *ScheduleHandler) createParticipant(w http.ResponseWriter, r *http.Request) {
	var p store.RotationParticipant
	if err := decodeJSON(w, r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.RotationID = chi.URLParam(r, "rotID")
	if p.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if err := h.schedules.CreateParticipant(r.Context(), &p); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *ScheduleHandler) deleteParticipant(w http.ResponseWriter, r *http.Request) {
	if err := h.schedules.DeleteParticipant(r.Context(), chi.URLParam(r, "partID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) listOverrides(w http.ResponseWriter, r *http.Request) {
	overrides, err := h.schedules.ListOverrides(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overrides)
}

func (h *ScheduleHandler) createOverride(w http.ResponseWriter, r *http.Request) {
	var o store.ScheduleOverride
	if err := decodeJSON(w, r, &o); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	o.ScheduleID = chi.URLParam(r, "id")
	if o.ReplaceUser == "" || o.OverrideUser == "" {
		writeError(w, http.StatusBadRequest, "replace_user and override_user are required")
		return
	}
	if !o.EndTime.After(o.StartTime) {
		writeError(w, http.StatusBadRequest, "end_time must be after start_time")
		return
	}
	if err := h.schedules.CreateOverride(r.Context(), &o); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

func (h *ScheduleHandler) deleteOverride(w http.ResponseWriter, r *http.Request) {
	if err := h.schedules.DeleteOverride(r.Context(), chi.URLParam(r, "overrideID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
