package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	"github.com/pagefire/pagefire/internal/store"
)

const maxRequestBodySize = 1 << 20 // 1MB
const maxListLimit = 1000
const defaultListLimit = 50

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// storeErrorStatus maps store sentinel errors to HTTP status codes.
// Returns a generic message to avoid leaking internal details.
func storeErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound, "resource not found"
	case errors.Is(err, store.ErrDuplicateKey):
		return http.StatusConflict, "resource already exists"
	case errors.Is(err, store.ErrConflict):
		return http.StatusConflict, "conflict"
	default:
		slog.Error("internal error", "error", err)
		return http.StatusInternalServerError, "internal server error"
	}
}

// handleStoreError writes an appropriate error response for store errors.
func handleStoreError(w http.ResponseWriter, err error) {
	status, msg := storeErrorStatus(err)
	writeError(w, status, msg)
}

// parseListLimit parses and validates limit/offset query parameters.
func parseListLimit(r *http.Request) (limit, offset int) {
	limit = defaultListLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

// validateEmail checks if a string is a valid email address.
func validateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// validateE164 checks if a phone number is in E.164 format (+1234567890).
func validateE164(phone string) bool {
	if len(phone) < 2 || len(phone) > 16 || phone[0] != '+' {
		return false
	}
	for _, c := range phone[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return phone[1] >= '1' && phone[1] <= '9'
}

// validateTimezone checks if a timezone string is valid.
func validateTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}

// validRoles is the whitelist of allowed user roles.
var validRoles = map[string]bool{
	"admin":  true,
	"user":   true,
	"viewer": true,
}

// validateRole checks if a role is in the allowed set.
func validateRole(role string) bool {
	return validRoles[role]
}
