package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken     string        `env:"TELEGRAM_BOT_TOKEN"`
	TelegramAllowedUsers string        `env:"TELEGRAM_ALLOWED_USERS"`
	DBPath               string        `env:"DB_PATH" envDefault:"/data/gosift.db"`
	LogLevel             string        `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat            string        `env:"LOG_FORMAT" envDefault:"text"`
	ParseInterval        time.Duration `env:"PARSE_INTERVAL" envDefault:"15m"`
	ParseJitter          time.Duration `env:"PARSE_JITTER" envDefault:"3s"`
	ParseTimeout         time.Duration `env:"PARSE_TIMEOUT" envDefault:"20s"`
	ParseLimit           int           `env:"PARSE_LIMIT" envDefault:"20"`
	ParseRetries         int           `env:"PARSE_RETRIES" envDefault:"3"`
	ParseRetryBackoff    time.Duration `env:"PARSE_RETRY_BACKOFF" envDefault:"5s"`
	NotifyAlertInterval  time.Duration `env:"NOTIFY_ALERT_INTERVAL" envDefault:"1h"`
	HTTPProxy            string        `env:"HTTP_PROXY"`
	StoreBaseURL         string        `env:"STORE_KOMISSIONKI_BASE_URL" envDefault:"https://komissionki.ru"`
	StoreAPIURL          string        `env:"STORE_KOMISSIONKI_API_URL" envDefault:"https://saf.komissionki.ru"`
	StoreCDNURL          string        `env:"STORE_KOMISSIONKI_CDN_URL" envDefault:"https://c.komissionki.ru"`
	StoreCDNFallbacks    string        `env:"STORE_KOMISSIONKI_CDN_FALLBACKS" envDefault:"https://cdny.komissionki.ru"`
	StoreEnabled         bool          `env:"STORE_KOMISSIONKI_ENABLED" envDefault:"true"`
	StoreRPS             float64       `env:"STORE_KOMISSIONKI_RPS" envDefault:"0.5"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("LOG_FORMAT must be text or json")
	}
	if c.ParseLimit <= 0 {
		return fmt.Errorf("PARSE_LIMIT must be positive")
	}
	if c.StoreRPS <= 0 {
		return fmt.Errorf("STORE_KOMISSIONKI_RPS must be positive")
	}
	return nil
}
