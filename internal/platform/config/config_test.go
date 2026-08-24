package config

import (
	"reflect"
	"testing"
)

func TestLoadProvidesBoundedHTTPAndPilotDefaults(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	configuration, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.HTTPReadHeaderTimeout <= 0 || configuration.HTTPReadTimeout <= 0 || configuration.HTTPWriteTimeout <= 0 || configuration.HTTPIdleTimeout <= 0 || configuration.HTTPMaxHeaderBytes < 1024 {
		t.Fatalf("HTTP bounds are incomplete: %#v", configuration)
	}
	if configuration.PilotCurrency != "INR" || configuration.ReadRateLimitPerMinute <= 0 || configuration.WriteRateLimitPerMinute <= 0 {
		t.Fatalf("pilot controls are incomplete: %#v", configuration)
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
