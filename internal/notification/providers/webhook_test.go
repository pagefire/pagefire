package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pagefire/pagefire/internal/notification"
)

func TestWebhook_BlocksLocalhostByDefault(t *testing.T) {
	w := NewWebhook(false)
	msg := notification.Message{
		To:      "http://localhost:9090/notify",
		Subject: "test",
		Body:    "test body",
	}
	_, err := w.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when sending to localhost with allowPrivateTargets=false")
	}
}

func TestWebhook_AllowsLocalhostWhenFlagSet(t *testing.T) {
	// Start a local test server to receive the webhook
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := NewWebhook(true)
	msg := notification.Message{
		To:       srv.URL + "/notify",
		Subject:  "test alert",
		Body:     "something broke",
		AlertID:  "alert-1",
		UserID:   "user-1",
		UserName: "Test User",
	}
	_, err := w.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("expected no error with allowPrivateTargets=true, got: %v", err)
	}

	if received["subject"] != "test alert" {
		t.Errorf("subject: got %q, want %q", received["subject"], "test alert")
	}
	if received["user_name"] != "Test User" {
		t.Errorf("user_name: got %q, want %q", received["user_name"], "Test User")
	}
	if received["user_id"] != "user-1" {
		t.Errorf("user_id: got %q, want %q", received["user_id"], "user-1")
	}
}

func TestWebhook_BlocksPrivateIPs(t *testing.T) {
	w := NewWebhook(false)
	targets := []string{
		"http://127.0.0.1:8080/hook",
		"http://10.0.0.1:8080/hook",
		"http://192.168.1.1:8080/hook",
	}
	for _, target := range targets {
		msg := notification.Message{To: target, Subject: "test"}
		_, err := w.Send(context.Background(), msg)
		if err == nil {
			t.Errorf("expected error for private target %s", target)
		}
	}
}

func TestWebhook_ValidateTarget(t *testing.T) {
	w := NewWebhook(false)

	// Invalid schemes
	if err := w.ValidateTarget("ftp://example.com"); err == nil {
		t.Error("expected error for ftp scheme")
	}

	// Missing hostname
	if err := w.ValidateTarget("http:///path"); err == nil {
		t.Error("expected error for missing hostname")
	}
}
