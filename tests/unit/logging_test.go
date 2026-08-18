package unit_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func TestLoggerRedactsSensitiveValues(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(slog.NewJSONHandler(&output, nil))
	logger.Info("request", "authorization", "Bearer secret", "balance_minor", 900, "safe", "ok")
	log := output.String()
	if strings.Contains(log, "Bearer secret") || strings.Contains(log, "900") {
		t.Fatalf("sensitive value was logged: %s", log)
	}
	if !strings.Contains(log, observability.Redacted) || !strings.Contains(log, "ok") {
		t.Fatalf("expected redaction and safe value: %s", log)
	}
}
