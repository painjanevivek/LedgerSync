package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment             string
	HTTPAddress             string
	ShutdownTimeout         time.Duration
	DatabaseURL             string
	RedisAddress            string
	ConsistencySigningKey   string
	ConsistencySigningKeyID string
	SessionSecret           string
	OIDCIssuerURL           string
	OIDCAudience            string
	BFFAssertionSecret      string
	DevelopmentSubjectID    string
	DevelopmentTenantID     string
}

func Load() (Config, error) {
	timeout, err := time.ParseDuration(valueOrDefault("LEDGERSYNC_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("LEDGERSYNC_SHUTDOWN_TIMEOUT must be a positive duration")
	}

	config := Config{
		Environment:             valueOrDefault("LEDGERSYNC_ENV", "development"),
		HTTPAddress:             valueOrDefault("LEDGERSYNC_HTTP_ADDR", ":8080"),
		ShutdownTimeout:         timeout,
		DatabaseURL:             os.Getenv("LEDGERSYNC_DATABASE_URL"),
		RedisAddress:            os.Getenv("LEDGERSYNC_REDIS_ADDR"),
		ConsistencySigningKey:   os.Getenv("LEDGERSYNC_CONSISTENCY_SIGNING_KEY"),
		ConsistencySigningKeyID: valueOrDefault("LEDGERSYNC_CONSISTENCY_SIGNING_KEY_ID", "current"),
		SessionSecret:           os.Getenv("LEDGERSYNC_SESSION_SECRET"),
		OIDCIssuerURL:           os.Getenv("LEDGERSYNC_OIDC_ISSUER_URL"),
		OIDCAudience:            valueOrDefault("LEDGERSYNC_OIDC_AUDIENCE", "ledgersync"),
		BFFAssertionSecret:      os.Getenv("LEDGERSYNC_BFF_ASSERTION_SECRET"),
		DevelopmentSubjectID:    os.Getenv("LEDGERSYNC_DEVELOPMENT_SUBJECT_ID"),
		DevelopmentTenantID:     os.Getenv("LEDGERSYNC_DEVELOPMENT_TENANT_ID"),
	}
	if config.Environment != "development" && (config.DatabaseURL == "" || config.RedisAddress == "" || config.SessionSecret == "" || len(config.ConsistencySigningKey) < 32 || config.OIDCIssuerURL == "" || config.OIDCAudience == "" || len(config.BFFAssertionSecret) < 32) {
		return Config{}, fmt.Errorf("database URL, redis address, session secret, 32-byte consistency key, OIDC issuer/audience, and 32-byte BFF assertion secret are required outside development")
	}
	return config, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
