package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pagefire/pagefire/internal/notification"
)

type TwilioCall struct {
	accountSID string
	authToken  string
	fromNumber string
	client     *http.Client
}

func NewTwilioCall(accountSID, authToken, fromNumber string) *TwilioCall {
	return &TwilioCall{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TwilioCall) Type() string { return "phone" }

func (t *TwilioCall) Send(ctx context.Context, msg notification.Message) (string, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json", t.accountSID)

	// Use TwiML to read the alert message aloud.
	// Twilio supports inline TwiML via the Twiml parameter (no callback URL needed).
	spokenMsg := fmt.Sprintf("Page Fire alert. %s. %s. Press any key to acknowledge.", msg.Subject, msg.Body)
	if len(spokenMsg) > 4000 {
		spokenMsg = spokenMsg[:3997] + "..."
	}
	twiml := fmt.Sprintf(`<Response><Say voice="alice" loop="2">%s</Say></Response>`, xmlEscape(spokenMsg))

	form := url.Values{}
	form.Set("To", msg.To)
	form.Set("From", t.fromNumber)
	form.Set("Twiml", twiml)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.accountSID, t.authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		SID          string `json:"sid"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Twilio response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("twilio call error %d: %s", result.ErrorCode, result.ErrorMessage)
	}

	return fmt.Sprintf("call:%s", result.SID), nil
}

func (t *TwilioCall) ValidateTarget(target string) error {
	if !ValidateE164(target) {
		return fmt.Errorf("invalid phone number, use E.164 format (e.g. +12025551234)")
	}
	return nil
}

// xmlEscape escapes special characters for TwiML content.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
