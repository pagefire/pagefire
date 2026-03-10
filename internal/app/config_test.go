package app

import (
	"os"
	"testing"
)

func TestLoadConfig_EnvVarMapping(t *testing.T) {
	// Verify that flat env vars like PAGEFIRE_ADMIN_TOKEN map to "admin_token",
	// not "admin.token" (which would happen if all underscores were replaced with dots).
	t.Setenv("PAGEFIRE_ADMIN_TOKEN", "secret-123")
	t.Setenv("PAGEFIRE_PORT", "4000")
	t.Setenv("PAGEFIRE_DATABASE_URL", "/tmp/test.db")
	t.Setenv("PAGEFIRE_LOG_LEVEL", "debug")
	t.Setenv("PAGEFIRE_ALLOW_PRIVATE_WEBHOOKS", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.AdminToken != "secret-123" {
		t.Errorf("AdminToken: got %q, want %q", cfg.AdminToken, "secret-123")
	}
	if cfg.Port != 4000 {
		t.Errorf("Port: got %d, want 4000", cfg.Port)
	}
	if cfg.DatabaseURL != "/tmp/test.db" {
		t.Errorf("DatabaseURL: got %q, want %q", cfg.DatabaseURL, "/tmp/test.db")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "debug")
	}
	if !cfg.AllowPrivateWebhooks {
		t.Error("AllowPrivateWebhooks: got false, want true")
	}
}

func TestLoadConfig_NestedEnvVars(t *testing.T) {
	// Verify that nested prefixes (smtp_, slack_, engine_) get dot-mapped correctly.
	t.Setenv("PAGEFIRE_SMTP_HOST", "mail.example.com")
	t.Setenv("PAGEFIRE_SMTP_PORT", "465")
	t.Setenv("PAGEFIRE_SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("PAGEFIRE_ENGINE_INTERVAL_SECONDS", "10")

	// Clear vars that might interfere from other tests
	os.Unsetenv("PAGEFIRE_ADMIN_TOKEN")
	os.Unsetenv("PAGEFIRE_PORT")
	os.Unsetenv("PAGEFIRE_DATABASE_URL")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.SMTP.Host != "mail.example.com" {
		t.Errorf("SMTP.Host: got %q, want %q", cfg.SMTP.Host, "mail.example.com")
	}
	if cfg.SMTP.Port != 465 {
		t.Errorf("SMTP.Port: got %d, want 465", cfg.SMTP.Port)
	}
	if cfg.Slack.BotToken != "xoxb-test" {
		t.Errorf("Slack.BotToken: got %q, want %q", cfg.Slack.BotToken, "xoxb-test")
	}
	if cfg.Engine.IntervalSeconds != 10 {
		t.Errorf("Engine.IntervalSeconds: got %d, want 10", cfg.Engine.IntervalSeconds)
	}
}
