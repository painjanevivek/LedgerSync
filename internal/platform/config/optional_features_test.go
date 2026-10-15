package config

import (
	"strings"
	"testing"
)

func TestOptionalFeatureMatrixAndCronBinding(t *testing.T) {
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("LEDGERSYNC_STUDY_ENABLED", "")
	t.Setenv("LEDGERSYNC_LIVE_INVESTIGATION_ENABLED", "")
	t.Setenv("CRON_SECRET", "")
	t.Setenv("LEDGERSYNC_CRON_WORKER_DATABASE_URL", "")
	if c, err := Load(); err != nil || c.SystemsLabEnabled || c.LiveInvestigationEnabled {
		t.Fatal("core default is invalid", err)
	}
	for _, value := range []string{"1", "yes", "TRUE", "unknown"} {
		t.Setenv("LEDGERSYNC_STUDY_ENABLED", value)
		if _, err := Load(); err == nil {
			t.Fatal("unknown study flag accepted", value)
		}
	}
	t.Setenv("LEDGERSYNC_STUDY_ENABLED", "true")
	if c, err := Load(); err != nil || !c.SystemsLabEnabled {
		t.Fatal("study set rejected", err)
	}
	t.Setenv("LEDGERSYNC_REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("LEDGERSYNC_LIVE_INVESTIGATION_ENABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("unqualified combined set accepted")
	}
	t.Setenv("LEDGERSYNC_STUDY_ENABLED", "false")
	if c, err := Load(); err != nil || !c.LiveInvestigationEnabled {
		t.Fatal("collaboration set rejected", err)
	}
	if err := validateOptionalFeatures("production", false, true); err == nil || !strings.Contains(err.Error(), "optional study features") {
		t.Fatal("nonlocal optional set accepted", err)
	}
	t.Setenv("LEDGERSYNC_ENV", "development")
	t.Setenv("LEDGERSYNC_LIVE_INVESTIGATION_ENABLED", "false")
	t.Setenv("CRON_SECRET", strings.Repeat("c", 32))
	if _, err := Load(); err == nil {
		t.Fatal("cron without worker binding accepted")
	}
	t.Setenv("LEDGERSYNC_DATABASE_URL", "postgres://api-binding")
	t.Setenv("LEDGERSYNC_CRON_WORKER_DATABASE_URL", "postgres://api-binding")
	if _, err := Load(); err == nil {
		t.Fatal("shared cron/API binding accepted")
	}
	t.Setenv("LEDGERSYNC_CRON_WORKER_DATABASE_URL", "postgres://separate-worker-binding")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
