package unit_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func TestLoggerRedactsSensitiveNestedValues(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(slog.NewJSONHandler(&output, nil))
	logger.Info("security event", slog.Group("request", "session_token", "secret", "amount_minor", 900), "correlation_id", "safe")
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode structured log: %v", err)
	}
	request, ok := event["request"].(map[string]any)
	if !ok || request["session_token"] != observability.Redacted || request["amount_minor"] != observability.Redacted || event["correlation_id"] != "safe" {
		t.Fatalf("nested sensitive data was not safely redacted: %s", output.String())
	}
}
