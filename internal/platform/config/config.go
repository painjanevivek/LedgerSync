package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddress     string
	ShutdownTimeout time.Duration
	DatabaseURL     string
	RedisAddress    string
	SessionSecret   string
	OIDCIssuerURL   string
	OIDCAudience    string
}

func Load() (Config, error) {
	timeout, err := time.ParseDuration(valueOrDefault("LEDGERSYNC_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("LEDGERSYNC_SHUTDOWN_TIMEOUT must be a positive duration")
	}

	config := Config{
		Environment:     valueOrDefault("LEDGERSYNC_ENV", "development"),
		HTTPAddress:     valueOrDefault("LEDGERSYNC_HTTP_ADDR", ":8080"),
		ShutdownTimeout: timeout,
		DatabaseURL:     os.Getenv("LEDGERSYNC_DATABASE_URL"),
		RedisAddress:    os.Getenv("LEDGERSYNC_REDIS_ADDR"),
		SessionSecret:   os.Getenv("LEDGERSYNC_SESSION_SECRET"),
		OIDCIssuerURL:   os.Getenv("LEDGERSYNC_OIDC_ISSUER_URL"),
		OIDCAudience:    valueOrDefault("LEDGERSYNC_OIDC_AUDIENCE", "ledgersync"),
	}
	if config.Environment != "development" && (config.DatabaseURL == "" || config.SessionSecret == "") {
		return Config{}, fmt.Errorf("database URL and session secret are required outside development")
	}
	return config, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
