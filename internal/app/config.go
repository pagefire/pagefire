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
	AdminToken     string       `koanf:"admin_token"`
	Engine         EngineConfig `koanf:"engine"`
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
	err := k.Load(env.Provider("PAGEFIRE_", ".", func(s string) string {
		return strings.ReplaceAll(
			strings.ToLower(strings.TrimPrefix(s, "PAGEFIRE_")),
			"_", ".",
		)
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
