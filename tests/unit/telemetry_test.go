package unit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
)

func TestTelemetryIsNoopLocallyAndRejectsIncompleteSharedConfiguration(t *testing.T) {
	telemetry, err := observability.NewTelemetry(context.Background(), observability.TelemetryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer telemetry.Shutdown(context.Background())
	handler := telemetry.HTTP(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusNoContent)
	}
	telemetry.ObserveBoundary(context.Background(), "cache", "get", time.Now(), nil)
	if _, err := observability.NewTelemetry(context.Background(), observability.TelemetryConfig{Enabled: true, ServiceName: "ledgersync-api"}); err == nil {
		t.Fatal("enabled telemetry without a private endpoint was accepted")
	}
}
