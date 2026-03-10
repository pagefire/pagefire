package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/store"
)

type EscalationPolicyHandler struct {
	policies store.EscalationPolicyStore
}

func NewEscalationPolicyHandler(policies store.EscalationPolicyStore) *EscalationPolicyHandler {
	return &EscalationPolicyHandler{policies: policies}
}

func (h *EscalationPolicyHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Get("/{id}/steps", h.listSteps)
	r.Post("/{id}/steps", h.createStep)
	r.Delete("/{id}/steps/{stepID}", h.deleteStep)

	r.Get("/{id}/steps/{stepID}/targets", h.listStepTargets)
	r.Post("/{id}/steps/{stepID}/targets", h.createStepTarget)
	r.Delete("/{id}/steps/{stepID}/targets/{targetID}", h.deleteStepTarget)

	return r
}

func (h *EscalationPolicyHandler) create(w http.ResponseWriter, r *http.Request) {
	var ep store.EscalationPolicy
	if err := decodeJSON(w, r, &ep); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if ep.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if ep.Repeat < 0 || ep.Repeat > 5 {
		writeError(w, http.StatusBadRequest, "repeat must be between 0 and 5")
		return
	}
	if err := h.policies.Create(r.Context(), &ep); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ep)
}

func (h *EscalationPolicyHandler) get(w http.ResponseWriter, r *http.Request) {
	ep, err := h.policies.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *EscalationPolicyHandler) list(w http.ResponseWriter, r *http.Request) {
	policies, err := h.policies.List(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

func (h *EscalationPolicyHandler) update(w http.ResponseWriter, r *http.Request) {
	var ep store.EscalationPolicy
	if err := decodeJSON(w, r, &ep); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ep.ID = chi.URLParam(r, "id")
	if ep.Repeat < 0 || ep.Repeat > 5 {
		writeError(w, http.StatusBadRequest, "repeat must be between 0 and 5")
		return
	}
	if err := h.policies.Update(r.Context(), &ep); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (h *EscalationPolicyHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.policies.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EscalationPolicyHandler) listSteps(w http.ResponseWriter, r *http.Request) {
	steps, err := h.policies.ListSteps(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, steps)
}

func (h *EscalationPolicyHandler) createStep(w http.ResponseWriter, r *http.Request) {
	var step store.EscalationStep
	if err := decodeJSON(w, r, &step); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	step.EscalationPolicyID = chi.URLParam(r, "id")
	if step.DelayMinutes < 0 {
		writeError(w, http.StatusBadRequest, "delay_minutes must be non-negative")
		return
	}
	if err := h.policies.CreateStep(r.Context(), &step); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, step)
}

func (h *EscalationPolicyHandler) deleteStep(w http.ResponseWriter, r *http.Request) {
	if err := h.policies.DeleteStep(r.Context(), chi.URLParam(r, "stepID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EscalationPolicyHandler) listStepTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.policies.ListStepTargets(r.Context(), chi.URLParam(r, "stepID"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, targets)
}

func (h *EscalationPolicyHandler) createStepTarget(w http.ResponseWriter, r *http.Request) {
	var target store.EscalationStepTarget
	if err := decodeJSON(w, r, &target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	target.EscalationStepID = chi.URLParam(r, "stepID")
	if target.TargetType != store.TargetTypeUser && target.TargetType != store.TargetTypeSchedule {
		writeError(w, http.StatusBadRequest, "target_type must be 'user' or 'schedule'")
		return
	}
	if target.TargetID == "" {
		writeError(w, http.StatusBadRequest, "target_id is required")
		return
	}
	if err := h.policies.CreateStepTarget(r.Context(), &target); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, target)
}

func (h *EscalationPolicyHandler) deleteStepTarget(w http.ResponseWriter, r *http.Request) {
	if err := h.policies.DeleteStepTarget(r.Context(), chi.URLParam(r, "targetID")); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
