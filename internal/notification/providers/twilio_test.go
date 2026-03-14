package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pagefire/pagefire/internal/notification"
)

func TestTwilioSMS_Type(t *testing.T) {
	sms := NewTwilioSMS("AC123", "token", "+15551234567")
	if sms.Type() != "sms" {
		t.Errorf("Type() = %q, want %q", sms.Type(), "sms")
	}
}

func TestTwilioSMS_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "AC123" || pass != "secret" {
			t.Errorf("bad auth: user=%q pass=%q ok=%v", user, pass, ok)
		}

		// Verify content type
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want form-urlencoded", ct)
		}

		// Verify form fields
		r.ParseForm()
		if to := r.FormValue("To"); to != "+12025551234" {
			t.Errorf("To = %q, want +12025551234", to)
		}
		if from := r.FormValue("From"); from != "+15551234567" {
			t.Errorf("From = %q, want +15551234567", from)
		}
		body := r.FormValue("Body")
		if body == "" {
			t.Error("Body is empty")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"sid": "SM123abc"})
	}))
	defer srv.Close()

	sms := NewTwilioSMS("AC123", "secret", "+15551234567")
	// Override the API URL by replacing the client with one that routes to our test server
	sms.client = srv.Client()
	// We need to override the URL. The simplest way is to use a custom transport.
	originalURL := srv.URL
	sms.client.Transport = rewriteTransport{url: originalURL}

	ref, err := sms.Send(context.Background(), notification.Message{
		To:      "+12025551234",
		Subject: "Server Down",
		Body:    "web-1 is unreachable",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if ref != "sms:SM123abc" {
		t.Errorf("ref = %q, want %q", ref, "sms:SM123abc")
	}
}

func TestTwilioSMS_SendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error_code":    21211,
			"error_message": "Invalid 'To' Phone Number",
		})
	}))
	defer srv.Close()

	sms := NewTwilioSMS("AC123", "secret", "+15551234567")
	sms.client = srv.Client()
	sms.client.Transport = rewriteTransport{url: srv.URL}

	_, err := sms.Send(context.Background(), notification.Message{
		To:      "+10000000000",
		Subject: "test",
		Body:    "test",
	})
	if err == nil {
		t.Fatal("expected error on 400 response")
	}
}

func TestTwilioSMS_ValidateTarget(t *testing.T) {
	sms := NewTwilioSMS("AC123", "token", "+15551234567")

	valid := []string{"+12025551234", "+442071234567", "+8618612345678"}
	for _, v := range valid {
		if err := sms.ValidateTarget(v); err != nil {
			t.Errorf("ValidateTarget(%q) = %v, want nil", v, err)
		}
	}

	invalid := []string{"", "12025551234", "+0999", "abc", "+1 202 555 1234"}
	for _, v := range invalid {
		if err := sms.ValidateTarget(v); err == nil {
			t.Errorf("ValidateTarget(%q) = nil, want error", v)
		}
	}
}

func TestTwilioCall_Type(t *testing.T) {
	call := NewTwilioCall("AC123", "token", "+15551234567")
	if call.Type() != "phone" {
		t.Errorf("Type() = %q, want %q", call.Type(), "phone")
	}
}

func TestTwilioCall_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "AC123" || pass != "secret" {
			t.Errorf("bad auth: user=%q pass=%q ok=%v", user, pass, ok)
		}

		r.ParseForm()
		if to := r.FormValue("To"); to != "+12025551234" {
			t.Errorf("To = %q, want +12025551234", to)
		}
		twiml := r.FormValue("Twiml")
		if twiml == "" {
			t.Error("Twiml is empty")
		}
		// Should contain Response and Say tags
		if !contains(twiml, "<Response>") || !contains(twiml, "<Say") {
			t.Errorf("Twiml missing expected tags: %s", twiml)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"sid": "CA456def"})
	}))
	defer srv.Close()

	call := NewTwilioCall("AC123", "secret", "+15551234567")
	call.client = srv.Client()
	call.client.Transport = rewriteTransport{url: srv.URL}

	ref, err := call.Send(context.Background(), notification.Message{
		To:      "+12025551234",
		Subject: "Server Down",
		Body:    "web-1 is unreachable",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if ref != "call:CA456def" {
		t.Errorf("ref = %q, want %q", ref, "call:CA456def")
	}
}

func TestTwilioCall_ValidateTarget(t *testing.T) {
	call := NewTwilioCall("AC123", "token", "+15551234567")
	if err := call.ValidateTarget("+12025551234"); err != nil {
		t.Errorf("ValidateTarget valid number: %v", err)
	}
	if err := call.ValidateTarget("not-a-number"); err == nil {
		t.Error("ValidateTarget invalid number: want error")
	}
}

func TestValidateE164(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"+12025551234", true},
		{"+442071234567", true},
		{"+8618612345678", true},
		{"+12", true}, // minimal valid (country code + 1 digit)
		{"", false},
		{"12025551234", false},   // no +
		{"+0999", false},         // leading 0
		{"+1 202", false},        // spaces
		{"+1234567890123456", false}, // too long (16 digits + plus = 17 chars)
		{"abc", false},
	}
	for _, tc := range cases {
		if got := ValidateE164(tc.input); got != tc.want {
			t.Errorf("ValidateE164(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestXmlEscape(t *testing.T) {
	input := `Alert: "server" <web-1> & status`
	got := xmlEscape(input)
	want := `Alert: &quot;server&quot; &lt;web-1&gt; &amp; status`
	if got != want {
		t.Errorf("xmlEscape() = %q, want %q", got, want)
	}
}

// contains is a simple helper for substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// rewriteTransport rewrites all requests to point at the test server URL.
type rewriteTransport struct {
	url string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = removeScheme(t.url)
	return http.DefaultTransport.RoundTrip(req)
}

func removeScheme(u string) string {
	if len(u) > 7 && u[:7] == "http://" {
		return u[7:]
	}
	if len(u) > 8 && u[:8] == "https://" {
		return u[8:]
	}
	return u
}
