package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pagefire/pagefire/internal/notification"
)

type Webhook struct {
	client *http.Client
}

func NewWebhook() *Webhook {
	return &Webhook{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *Webhook) Type() string { return "webhook" }

func (w *Webhook) Send(ctx context.Context, msg notification.Message) (string, error) {
	// SSRF protection: validate URL target before connecting
	if err := validateWebhookTarget(msg.To); err != nil {
		return "", fmt.Errorf("blocked webhook target: %w", err)
	}

	payload := map[string]string{
		"alert_id": msg.AlertID,
		"subject":  msg.Subject,
		"body":     msg.Body,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msg.To, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PageFire/1.0")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return fmt.Sprintf("webhook:%d", resp.StatusCode), nil
}

func (w *Webhook) ValidateTarget(target string) error {
	return validateWebhookTarget(target)
}

// validateWebhookTarget validates a webhook URL for SSRF protection.
func validateWebhookTarget(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Resolve hostname and block private/internal IPs
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("cannot resolve hostname")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("webhook URL must not point to private/internal addresses")
		}
	}
	return nil
}

// isBlockedIP checks if an IP is in a private/reserved range.
func isBlockedIP(ip net.IP) bool {
	blockedCIDRs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"0.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range blockedCIDRs {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
