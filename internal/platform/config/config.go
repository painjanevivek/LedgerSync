package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment                string
	HTTPAddress                string
	ShutdownTimeout            time.Duration
	DatabaseURL                string
	RedisAddress               string
	ConsistencySigningKey      string
	ConsistencySigningKeyID    string
	SessionSecret              string
	OIDCIssuerURL              string
	OIDCAudience               string
	BFFAssertionSecret         string
	BFFAssertionKeyID          string
	BFFAssertionIssuer         string
	BFFAssertionAudience       string
	BFFAssertionPreviousSecret string
	BFFAssertionPreviousKeyID  string
	DevelopmentSubjectID       string
	DevelopmentTenantID        string
	TelemetryEnabled           bool
	TelemetryServiceName       string
	OTLPHTTPEndpoint           string
}

func Load() (Config, error) {
	timeout, err := time.ParseDuration(valueOrDefault("LEDGERSYNC_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("LEDGERSYNC_SHUTDOWN_TIMEOUT must be a positive duration")
	}

	telemetryEnabled, err := parseBool("LEDGERSYNC_TELEMETRY_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	config := Config{
		Environment:                valueOrDefault("LEDGERSYNC_ENV", "development"),
		HTTPAddress:                valueOrDefault("LEDGERSYNC_HTTP_ADDR", ":8080"),
		ShutdownTimeout:            timeout,
		DatabaseURL:                os.Getenv("LEDGERSYNC_DATABASE_URL"),
		RedisAddress:               os.Getenv("LEDGERSYNC_REDIS_ADDR"),
		ConsistencySigningKey:      os.Getenv("LEDGERSYNC_CONSISTENCY_SIGNING_KEY"),
		ConsistencySigningKeyID:    valueOrDefault("LEDGERSYNC_CONSISTENCY_SIGNING_KEY_ID", "current"),
		SessionSecret:              os.Getenv("LEDGERSYNC_SESSION_SECRET"),
		OIDCIssuerURL:              os.Getenv("LEDGERSYNC_OIDC_ISSUER_URL"),
		OIDCAudience:               valueOrDefault("LEDGERSYNC_OIDC_AUDIENCE", "ledgersync"),
		BFFAssertionSecret:         os.Getenv("LEDGERSYNC_BFF_ASSERTION_SECRET"),
		BFFAssertionKeyID:          valueOrDefault("LEDGERSYNC_BFF_ASSERTION_KEY_ID", "current"),
		BFFAssertionIssuer:         valueOrDefault("LEDGERSYNC_BFF_ASSERTION_ISSUER", "ledgersync-bff"),
		BFFAssertionAudience:       valueOrDefault("LEDGERSYNC_BFF_ASSERTION_AUDIENCE", "ledgersync-private-api"),
		BFFAssertionPreviousSecret: os.Getenv("LEDGERSYNC_BFF_ASSERTION_PREVIOUS_SECRET"),
		BFFAssertionPreviousKeyID:  os.Getenv("LEDGERSYNC_BFF_ASSERTION_PREVIOUS_KEY_ID"),
		DevelopmentSubjectID:       os.Getenv("LEDGERSYNC_DEVELOPMENT_SUBJECT_ID"),
		DevelopmentTenantID:        os.Getenv("LEDGERSYNC_DEVELOPMENT_TENANT_ID"),
		TelemetryEnabled:           telemetryEnabled,
		TelemetryServiceName:       valueOrDefault("LEDGERSYNC_TELEMETRY_SERVICE_NAME", "ledgersync-api"),
		OTLPHTTPEndpoint:           os.Getenv("LEDGERSYNC_OTLP_HTTP_ENDPOINT"),
	}
	if config.Environment != "development" && (config.DatabaseURL == "" || config.RedisAddress == "" || config.SessionSecret == "" || len(config.ConsistencySigningKey) < 32 || config.OIDCIssuerURL == "" || config.OIDCAudience == "" || len(config.BFFAssertionSecret) < 32) {
		return Config{}, fmt.Errorf("database URL, redis address, session secret, 32-byte consistency key, OIDC issuer/audience, and 32-byte BFF assertion secret are required outside development")
	}
	if config.Environment != "development" && (!config.TelemetryEnabled || config.OTLPHTTPEndpoint == "") {
		return Config{}, fmt.Errorf("enabled telemetry and a private OTLP HTTP endpoint are required outside development")
	}
	if (config.BFFAssertionPreviousSecret == "") != (config.BFFAssertionPreviousKeyID == "") || (config.BFFAssertionPreviousSecret != "" && len(config.BFFAssertionPreviousSecret) < 32) {
		return Config{}, fmt.Errorf("previous BFF assertion key ID and 32-byte secret must be configured together")
	}
	return config, nil
}

func parseBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}
	return false, fmt.Errorf("%s must be true or false", key)
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
