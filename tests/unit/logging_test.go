package unit_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func TestLoggerRedactsSensitiveValues(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(slog.NewJSONHandler(&output, nil))
	logger.Info("request", "authorization", "Bearer secret", "balance_minor", 900, "csrf_value", "csrf-secret", "safe", "ok")
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode structured log: %v", err)
	}
	if record["authorization"] != observability.Redacted || record["balance_minor"] != observability.Redacted || record["csrf_value"] != observability.Redacted {
		t.Fatalf("sensitive fields were not redacted: %s", output.String())
	}
	if record["safe"] != "ok" {
		t.Fatalf("safe field was not retained: %s", output.String())
	}
}

func TestLoggerRedactsSensitiveValuesEmbeddedInErrors(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(slog.NewJSONHandler(&output, nil))
	logger.Error("dependency", "error", "connect postgres://ledger:actual-secret@postgres:5432/ledger")
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["error"] != observability.Redacted {
		t.Fatalf("database credential was emitted: %s", output.String())
	}
}
