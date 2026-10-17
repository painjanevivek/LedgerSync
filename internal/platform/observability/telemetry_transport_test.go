package observability

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

func TestTelemetryTransportRejectsAuthorityConfusion(t *testing.T) {
	for _, value := range []string{"https://user:password@collector:4318", "https://collector:4318/path", "https://collector:4318?token=x", "https://collector:4318#fragment", "ftp://collector:4318", "collector", " collector:4318"} {
		if _, _, err := telemetryTransport(value, ""); err == nil {
			t.Fatalf("unsafe endpoint accepted %q", value)
		}
	}
	for _, value := range []string{"collector:4318", "http://127.0.0.1:4318", "https://collector:4318", "https://[::1]:4318"} {
		target, tlsConfig, err := telemetryTransport(value, "")
		if err != nil || target == "" {
			t.Fatalf("valid endpoint %q: %v", value, err)
		}
		if len(value) > 5 && value[:5] == "https" && (tlsConfig == nil || tlsConfig.InsecureSkipVerify) {
			t.Fatal("HTTPS verification disabled")
		}
	}
	if _, _, err := telemetryTransport("http://127.0.0.1:4318", "missing-ca"); err == nil {
		t.Fatal("CA silently used with plaintext")
	}
	if _, _, err := telemetryTransport("https://collector:4318", "missing-ca"); err == nil {
		t.Fatal("missing trust file ignored")
	}
}

func TestTelemetryExportsTracesAndMetricsWithVerifiedLocalTLS(t *testing.T) {
	var lock sync.Mutex
	paths := make(map[string]int)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("telemetry arrived without TLS")
		}
		lock.Lock()
		paths[r.URL.Path]++
		lock.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	certificate := filepath.Join(t.TempDir(), "local-ca.pem")
	if err := os.WriteFile(certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	telemetry, err := NewTelemetry(ctx, TelemetryConfig{Enabled: true, ServiceName: "ledgersync-local-tls-test", Endpoint: server.URL, CAFile: certificate})
	if err != nil {
		t.Fatal(err)
	}
	// A low fixed trace ID makes ratio sampling deterministic in this test.
	parent := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{15: 1}, SpanID: trace.SpanID{7: 1}, TraceFlags: trace.FlagsSampled})
	spanCtx, span := telemetry.Start(trace.ContextWithSpanContext(ctx, parent), "synthetic.tls.probe")
	telemetry.ObserveBoundary(spanCtx, "database", "read", time.Now(), nil)
	span.End()
	if err := telemetry.Shutdown(ctx); err != nil {
		t.Fatal("verified TLS export failed", err)
	}
	lock.Lock()
	defer lock.Unlock()
	if paths["/v1/traces"] != 1 || paths["/v1/metrics"] != 1 {
		t.Fatalf("expected both actual OTLP exporters over TLS: %v", paths)
	}
}
