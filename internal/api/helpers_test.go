package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

// ---------- parseListLimit ----------

func TestParseListLimit(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{
			name:       "no params defaults",
			query:      "",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "valid limit",
			query:      "limit=25",
			wantLimit:  25,
			wantOffset: 0,
		},
		{
			name:       "limit exceeds max capped at 1000",
			query:      "limit=5000",
			wantLimit:  1000,
			wantOffset: 0,
		},
		{
			name:       "limit exactly at max",
			query:      "limit=1000",
			wantLimit:  1000,
			wantOffset: 0,
		},
		{
			name:       "negative limit uses default",
			query:      "limit=-5",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "zero limit uses default",
			query:      "limit=0",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "non-numeric limit uses default",
			query:      "limit=abc",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "valid offset",
			query:      "offset=10",
			wantLimit:  50,
			wantOffset: 10,
		},
		{
			name:       "negative offset uses zero",
			query:      "offset=-1",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "non-numeric offset uses zero",
			query:      "offset=xyz",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "both valid",
			query:      "limit=10&offset=20",
			wantLimit:  10,
			wantOffset: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/"
			if tt.query != "" {
				url = "/?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			limit, offset := parseListLimit(req)

			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

// ---------- validateEmail ----------

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"name+tag@domain.org", true},
		{"user@sub.domain.com", true},
		{"a@b.co", true},
		{"", false},
		{"plaintext", false},
		{"@nodomain.com", false},
		{"missing@", false},
		{"spaces in@email.com", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.email), func(t *testing.T) {
			got := validateEmail(tt.email)
			if got != tt.want {
				t.Errorf("validateEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

// ---------- validateTimezone ----------

func TestValidateTimezone(t *testing.T) {
	tests := []struct {
		tz   string
		want bool
	}{
		{"UTC", true},
		{"America/New_York", true},
		{"Europe/London", true},
		{"Asia/Tokyo", true},
		{"Invalid/Zone", false},
		{"", true}, // empty string loads UTC in Go
		{"Not_A_Timezone", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.tz), func(t *testing.T) {
			got := validateTimezone(tt.tz)
			if got != tt.want {
				t.Errorf("validateTimezone(%q) = %v, want %v", tt.tz, got, tt.want)
			}
		})
	}
}

// ---------- validateRole ----------

func TestValidateRole(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{"admin", true},
		{"user", true},
		{"viewer", true},
		{"superadmin", false},
		{"", false},
		{"Admin", false},
		{"USER", false},
		{"owner", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.role), func(t *testing.T) {
			got := validateRole(tt.role)
			if got != tt.want {
				t.Errorf("validateRole(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

// ---------- storeErrorStatus ----------

func TestStoreErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "ErrNotFound",
			err:        store.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    "resource not found",
		},
		{
			name:       "wrapped ErrNotFound",
			err:        fmt.Errorf("get user: %w", store.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantMsg:    "resource not found",
		},
		{
			name:       "ErrDuplicateKey",
			err:        store.ErrDuplicateKey,
			wantStatus: http.StatusConflict,
			wantMsg:    "resource already exists",
		},
		{
			name:       "ErrConflict",
			err:        store.ErrConflict,
			wantStatus: http.StatusConflict,
			wantMsg:    "conflict",
		},
		{
			name:       "random error",
			err:        errors.New("something went wrong"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, msg := storeErrorStatus(tt.err)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}
