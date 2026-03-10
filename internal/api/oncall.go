package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pagefire/pagefire/internal/oncall"
)

type OnCallHandler struct {
	resolver *oncall.Resolver
}

func NewOnCallHandler(resolver *oncall.Resolver) *OnCallHandler {
	return &OnCallHandler{resolver: resolver}
}

func (h *OnCallHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{scheduleID}", h.whosOnCall)
	return r
}

func (h *OnCallHandler) whosOnCall(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")

	at := time.Now()
	if atStr := r.URL.Query().Get("at"); atStr != "" {
		parsed, err := time.Parse(time.RFC3339, atStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'at' parameter, use RFC3339 format")
			return
		}
		at = parsed
	}

	users, err := h.resolver.Resolve(r.Context(), scheduleID, at)
	if err != nil {
		handleStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, users)
}
