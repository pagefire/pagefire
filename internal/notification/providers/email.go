package providers

import (
	"context"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/pagefire/pagefire/internal/notification"
)

// sanitizeHeader removes CR and LF characters to prevent email header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

type Email struct {
	host     string
	port     int
	from     string
	username string
	password string
}

func NewEmail(host string, port int, from, username, password string) *Email {
	return &Email{host: host, port: port, from: from, username: username, password: password}
}

func (e *Email) Type() string { return "email" }

func (e *Email) Send(_ context.Context, msg notification.Message) (string, error) {
	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	// Sanitize all header values to prevent header injection
	to := sanitizeHeader(msg.To)
	subject := sanitizeHeader(msg.Subject)
	from := sanitizeHeader(e.from)

	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		from, to, subject)
	body := headers + msg.Body

	var auth smtp.Auth
	if e.username != "" {
		auth = smtp.PlainAuth("", e.username, e.password, e.host)
	}

	err := smtp.SendMail(addr, auth, e.from, []string{to}, []byte(body))
	if err != nil {
		return "", err
	}

	return "email:sent", nil
}

func (e *Email) ValidateTarget(target string) error {
	_, err := mail.ParseAddress(target)
	if err != nil {
		return fmt.Errorf("invalid email address")
	}
	return nil
}
