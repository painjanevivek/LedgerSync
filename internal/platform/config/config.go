package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment                string
	HTTPAddress                string
	ShutdownTimeout            time.Duration
	HTTPReadHeaderTimeout      time.Duration
	HTTPReadTimeout            time.Duration
	HTTPWriteTimeout           time.Duration
	HTTPIdleTimeout            time.Duration
	HTTPMaxHeaderBytes         int
	PilotCurrency              string
	ReadRateLimitPerMinute     int
	WriteRateLimitPerMinute    int
	RedisStreamMaxLength       int64
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
	environment := valueOrDefault("LEDGERSYNC_ENV", "development")
	pilotCurrency := strings.ToUpper(strings.TrimSpace(os.Getenv("LEDGERSYNC_PILOT_CURRENCY")))
	if pilotCurrency == "" {
		if environment != "development" {
			return Config{}, fmt.Errorf("LEDGERSYNC_PILOT_CURRENCY must be explicitly selected outside development")
		}
		pilotCurrency = "USD"
	}
	timeout, err := time.ParseDuration(valueOrDefault("LEDGERSYNC_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("LEDGERSYNC_SHUTDOWN_TIMEOUT must be a positive duration")
	}

	readHeaderTimeout, err := positiveDuration("LEDGERSYNC_HTTP_READ_HEADER_TIMEOUT", "3s")
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := positiveDuration("LEDGERSYNC_HTTP_READ_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := positiveDuration("LEDGERSYNC_HTTP_WRITE_TIMEOUT", "15s")
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := positiveDuration("LEDGERSYNC_HTTP_IDLE_TIMEOUT", "60s")
	if err != nil {
		return Config{}, err
	}
	maxHeaderBytes, err := positiveInt("LEDGERSYNC_HTTP_MAX_HEADER_BYTES", 32*1024)
	if err != nil {
		return Config{}, err
	}
	readRate, err := positiveInt("LEDGERSYNC_READ_RATE_LIMIT_PER_MINUTE", 120)
	if err != nil {
		return Config{}, err
	}
	writeRate, err := positiveInt("LEDGERSYNC_WRITE_RATE_LIMIT_PER_MINUTE", 30)
	if err != nil {
		return Config{}, err
	}
	streamMaxLength, err := positiveInt("LEDGERSYNC_REDIS_STREAM_MAX_LENGTH", 5_000_000)
	if err != nil {
		return Config{}, err
	}
	telemetryEnabled, err := parseBool("LEDGERSYNC_TELEMETRY_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	config := Config{
		Environment:                environment,
		HTTPAddress:                valueOrDefault("LEDGERSYNC_HTTP_ADDR", ":8080"),
		ShutdownTimeout:            timeout,
		HTTPReadHeaderTimeout:      readHeaderTimeout,
		HTTPReadTimeout:            readTimeout,
		HTTPWriteTimeout:           writeTimeout,
		HTTPIdleTimeout:            idleTimeout,
		HTTPMaxHeaderBytes:         maxHeaderBytes,
		PilotCurrency:              pilotCurrency,
		ReadRateLimitPerMinute:     readRate,
		WriteRateLimitPerMinute:    writeRate,
		RedisStreamMaxLength:       int64(streamMaxLength),
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
	if len(config.PilotCurrency) != 3 || config.PilotCurrency[0] < 'A' || config.PilotCurrency[0] > 'Z' || config.PilotCurrency[1] < 'A' || config.PilotCurrency[1] > 'Z' || config.PilotCurrency[2] < 'A' || config.PilotCurrency[2] > 'Z' {
		return Config{}, fmt.Errorf("LEDGERSYNC_PILOT_CURRENCY must be an ISO-style three-letter uppercase code")
	}
	return config, nil
}

func positiveDuration(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(valueOrDefault(key, fallback))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func positiveInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value <= 0 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%s must be a positive 32-bit integer", key)
	}
	return int(value), nil
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
