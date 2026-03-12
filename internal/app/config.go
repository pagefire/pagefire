package app

import (
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Config holds all application configuration.
type Config struct {
	Port           int          `koanf:"port"`
	DatabaseURL    string       `koanf:"database_url"`
	DatabaseDriver string       `koanf:"database_driver"` // "sqlite" or "postgres"
	DataDir        string       `koanf:"data_dir"`
	LogLevel       string       `koanf:"log_level"`
	AllowPrivateWebhooks bool         `koanf:"allow_private_webhooks"`
	Engine               EngineConfig `koanf:"engine"`
	SMTP           SMTPConfig   `koanf:"smtp"`
	Slack          SlackConfig  `koanf:"slack"`
}

type EngineConfig struct {
	IntervalSeconds int `koanf:"interval_seconds"`
}

type SMTPConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	From     string `koanf:"from"`
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

type SlackConfig struct {
	BotToken string `koanf:"bot_token"`
}

// LoadConfig loads configuration from environment variables with PAGEFIRE_ prefix.
// Precedence: env vars > defaults.
func LoadConfig() (*Config, error) {
	k := koanf.New(".")

	// Defaults
	k.Set("port", 3000)
	k.Set("database_driver", "sqlite")
	k.Set("data_dir", ".")
	k.Set("log_level", "info")
	k.Set("engine.interval_seconds", 5)
	k.Set("smtp.port", 587)

	// Environment variables: PAGEFIRE_PORT, PAGEFIRE_DATABASE_URL, etc.
	// Known prefixes are mapped to nested struct fields; single underscores
	// within field names are preserved (e.g. database_url, allow_private_webhooks).
	nestedPrefixes := []string{"smtp_", "slack_", "engine_"}

	err := k.Load(env.Provider("PAGEFIRE_", ".", func(s string) string {
		key := strings.ToLower(strings.TrimPrefix(s, "PAGEFIRE_"))
		for _, prefix := range nestedPrefixes {
			if strings.HasPrefix(key, prefix) {
				// Replace only the first underscore to create nesting dot
				// e.g. smtp_host → smtp.host, slack_bot_token → slack.bot_token
				return strings.Replace(key, "_", ".", 1)
			}
		}
		return key
	}), nil)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}

	// Default SQLite path if no DATABASE_URL set
	if cfg.DatabaseURL == "" && cfg.DatabaseDriver == "sqlite" {
		cfg.DatabaseURL = cfg.DataDir + "/pagefire.db"
	}

	return &cfg, nil
}
