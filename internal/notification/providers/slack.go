package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pagefire/pagefire/internal/notification"
)

type Slack struct {
	botToken string
	client   *http.Client
}

func NewSlack(botToken string) *Slack {
	return &Slack{
		botToken: botToken,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Slack) Type() string { return "slack_dm" }

func (s *Slack) Send(ctx context.Context, msg notification.Message) (string, error) {
	payload := map[string]string{
		"channel": msg.To,
		"text":    fmt.Sprintf("*%s*\n%s", msg.Subject, msg.Body),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("slack API error: %s", result.Error)
	}

	return fmt.Sprintf("slack:%s", result.TS), nil
}

func (s *Slack) ValidateTarget(target string) error {
	if target == "" {
		return fmt.Errorf("slack channel/user ID required")
	}
	return nil
}
