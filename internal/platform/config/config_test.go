package config

import (
	"reflect"
	"testing"
)

func TestLoadProvidesBoundedHTTPAndPilotDefaults(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("PORT", "")
	t.Setenv("LEDGERSYNC_DEVELOPMENT_SUBJECT_ID", "")
	t.Setenv("LEDGERSYNC_DEVELOPMENT_TENANT_ID", "")
	configuration, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.HTTPReadHeaderTimeout <= 0 || configuration.HTTPReadTimeout <= 0 || configuration.HTTPWriteTimeout <= 0 || configuration.HTTPIdleTimeout <= 0 || configuration.HTTPMaxHeaderBytes < 1024 {
		t.Fatalf("HTTP bounds are incomplete: %#v", configuration)
	}
	if configuration.StartupTimeout <= 0 || configuration.StartupInitialBackoff <= 0 || configuration.StartupMaxBackoff < configuration.StartupInitialBackoff || configuration.StartupTimeout < configuration.StartupMaxBackoff {
		t.Fatalf("startup retry bounds are incomplete: %#v", configuration)
	}
	if configuration.PilotCurrency != "INR" || configuration.ReadRateLimitPerMinute != 6_000 || configuration.WriteRateLimitPerMinute != 1_800 || configuration.WriteCapacityPerSecond != 30 {
		t.Fatalf("pilot controls are incomplete: %#v", configuration)
	}
}

func TestLoadUsesVercelPortWhenProvided(t *testing.T) {
	t.Setenv("PORT", "3000")
	t.Setenv("LEDGERSYNC_HTTP_ADDR", ":8080")
	configuration, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.HTTPAddress != ":3000" {
		t.Fatalf("expected Vercel PORT to win, got %q", configuration.HTTPAddress)
	}
}

func TestLoadRejectsInvalidVercelPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")
	if _, err := Load(); err == nil {
		t.Fatal("expected an invalid Vercel PORT to fail")
	}
}

func TestLoadRejectsWeakCronSecret(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("CRON_SECRET", "weak")
	if _, err := Load(); err == nil {
		t.Fatal("expected a weak cron credential to fail")
	}
}

func TestLoadRequiresGeneratedCredentialForDevelopmentIdentity(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("LEDGERSYNC_DEVELOPMENT_SUBJECT_ID", "demo")
	t.Setenv("LEDGERSYNC_DEVELOPMENT_TENANT_ID", "tenant")
	t.Setenv("LEDGERSYNC_DEVELOPMENT_API_TOKEN", "fixed")
	if _, err := Load(); err == nil {
		t.Fatal("development identity accepted a weak workload credential")
	}
}

func TestLoadRejectsInvalidStartupRetryOrder(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("LEDGERSYNC_STARTUP_TIMEOUT", "1s")
	t.Setenv("LEDGERSYNC_STARTUP_INITIAL_BACKOFF", "2s")
	t.Setenv("LEDGERSYNC_STARTUP_MAX_BACKOFF", "3s")
	if _, err := Load(); err == nil {
		t.Fatal("invalid startup retry order was accepted")
	}
}

func TestLoadRejectsUnsafeWriteCapacity(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("LEDGERSYNC_WRITE_CAPACITY_PER_SECOND", "10001")
	if _, err := Load(); err == nil {
		t.Fatal("unbounded write capacity was accepted")
	}
}

func TestParseOIDCClientTenantMap(t *testing.T) {
	want := map[string]string{"partner-client": "00000000-0000-4000-8000-000000000001"}
	got, err := parseOIDCClientTenantMap(`{"partner-client":"00000000-0000-4000-8000-000000000001"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected mapping: %#v", got)
	}
	for name, raw := range map[string]string{
		"empty":        `{}`,
		"blank client": `{"":"tenant"}`,
		"blank tenant": `{"client":""}`,
		"array":        `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseOIDCClientTenantMap(raw); err == nil {
				t.Fatalf("invalid mapping %q was accepted", raw)
			}
		})
	}
}

func TestLoadRejectsInvalidPilotCurrency(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("LEDGERSYNC_PILOT_CURRENCY", "US")
	if _, err := Load(); err == nil {
		t.Fatal("invalid pilot currency was accepted")
	}
}

func TestLoadRequiresExplicitPilotCurrencyOutsideDevelopment(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "production")
	t.Setenv("LEDGERSYNC_PILOT_CURRENCY", "")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted an implicit pilot currency")
	}
}

func TestLoadRequiresAbsoluteFixedRecoveryEvidenceRoot(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("PORT", "")
	t.Setenv("LEDGERSYNC_RECOVERY_EVIDENCE_ROOT", "data/local-backups")
	if _, err := Load(); err == nil {
		t.Fatal("relative recovery evidence root was accepted")
	}
	t.Setenv("LEDGERSYNC_RECOVERY_EVIDENCE_ROOT", "/run/ledgersync/recovery")
	configuration, err := Load()
	if err != nil || configuration.RecoveryEvidenceRoot != "/run/ledgersync/recovery" {
		t.Fatalf("absolute recovery root=%q error=%v", configuration.RecoveryEvidenceRoot, err)
	}
}
