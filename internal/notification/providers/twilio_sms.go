package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pagefire/pagefire/internal/notification"
)

var e164Regex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// ValidateE164 checks if a phone number is in E.164 format (+1234567890).
func ValidateE164(phone string) bool {
	return e164Regex.MatchString(phone)
}

type TwilioSMS struct {
	accountSID string
	authToken  string
	fromNumber string
	client     *http.Client
}

func NewTwilioSMS(accountSID, authToken, fromNumber string) *TwilioSMS {
	return &TwilioSMS{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TwilioSMS) Type() string { return "sms" }

func (t *TwilioSMS) Send(ctx context.Context, msg notification.Message) (string, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)

	body := fmt.Sprintf("[PageFire] %s\n\n%s", msg.Subject, msg.Body)
	if len(body) > 1600 {
		body = body[:1597] + "..."
	}

	form := url.Values{}
	form.Set("To", msg.To)
	form.Set("From", t.fromNumber)
	form.Set("Body", body)

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
		return "", fmt.Errorf("twilio SMS error %d: %s", result.ErrorCode, result.ErrorMessage)
	}

	return fmt.Sprintf("sms:%s", result.SID), nil
}

func (t *TwilioSMS) ValidateTarget(target string) error {
	if !ValidateE164(target) {
		return fmt.Errorf("invalid phone number, use E.164 format (e.g. +12025551234)")
	}
	return nil
}
