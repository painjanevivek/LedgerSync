package unit_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func TestLoggerRedactsSensitiveNestedValues(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger(slog.NewJSONHandler(&output, nil))
	logger.Info("security event", slog.Group("request", "session_token", "secret", "amount_minor", 900), "correlation_id", "safe")
	logged := output.String()
	if strings.Contains(logged, "secret") || strings.Contains(logged, "900") || !strings.Contains(logged, observability.Redacted) || !strings.Contains(logged, "safe") {
		t.Fatalf("nested sensitive data was not safely redacted: %s", logged)
	}
}
