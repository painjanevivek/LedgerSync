package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceHeadersAreDiagnosticOnlyAndDoNotExportBaggage(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	telemetry := &Telemetry{tracer: provider.Tracer("test")}
	request := httptest.NewRequest("GET", "/api/accounts/private-account?token=private-token", nil)
	request.Header.Set("traceparent", "00-12345678901234567890123456789012-1234567890123456-01")
	request.Header.Set("baggage", "amount=900,password=secret")
	request.Header.Set("tracestate", "customer=private")
	telemetry.HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := trace.SpanContextFromContext(r.Context()).TraceID().String(); got != "12345678901234567890123456789012" {
			t.Fatalf("parent lost: %s", got)
		}
		w.WriteHeader(http.StatusForbidden)
	})).ServeHTTP(httptest.NewRecorder(), request)
	spans := recorder.Ended()
	if len(spans) != 1 || len(spans[0].Attributes()) != 0 || spans[0].SpanContext().TraceState().Len() != 0 {
		t.Fatal("private fields entered span")
	}
}

func TestBoundaryFailureNeverExportsRawError(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	telemetry := &Telemetry{tracer: provider.Tracer("test")}
	ctx, span := telemetry.Start(context.Background(), "db.transfer.submit")
	telemetry.ObserveBoundary(ctx, "postgres", "submit", time.Now(), errors.New("customer@example.test amount=999 token=secret"))
	span.End()
	got := recorder.Ended()[0]
	if got.Status().Code != codes.Error || got.Status().Description != "operation_failed" || len(got.Events()) != 0 {
		t.Fatalf("unsafe error export: %+v", got.Status())
	}
}

func TestDisabledTelemetryDoesNotEndCallerSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	ctx, parent := provider.Tracer("test").Start(context.Background(), "caller")
	var telemetry *Telemetry
	_, child := telemetry.Start(ctx, "disabled-child")
	child.End()
	if len(recorder.Ended()) != 0 {
		t.Fatal("disabled child ended caller span")
	}
	parent.End()
	if len(recorder.Ended()) != 1 {
		t.Fatal("caller span missing")
	}
}

func TestLinkedAttemptsKeepOneOriginAndDifferentAttemptSpans(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	telemetry := &Telemetry{tracer: provider.Tracer("test")}
	origin := "00-12345678901234567890123456789012-1234567890123456-01"
	for range 2 {
		_, span := telemetry.StartLinked(context.Background(), "event.redis.publish", origin)
		span.End()
	}
	got := recorder.Ended()
	if len(got) != 2 || got[0].SpanContext().SpanID() == got[1].SpanContext().SpanID() || len(got[0].Links()) != 1 || got[0].Links()[0].SpanContext.TraceID() != got[1].Links()[0].SpanContext.TraceID() {
		t.Fatal("attempt identity or origin link lost")
	}
	for _, bad := range []string{"", strings.Repeat("x", 256), "00-00000000000000000000000000000000-1234567890123456-01", "00-12345678901234567890123456789012-0000000000000000-01"} {
		if TraceContextFromHeader(bad).IsValid() {
			t.Fatal("invalid diagnostic accepted")
		}
	}
}

func TestUnavailableExporterDoesNotBlockRequestOrShutdown(t *testing.T) {
	telemetry, err := NewTelemetry(context.Background(), TelemetryConfig{Enabled: true, ServiceName: "local-test", Endpoint: "127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	handler := telemetry.HTTP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }))
	for range 2000 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/transfers", nil))
		if response.Code != http.StatusCreated {
			t.Fatal("export failure changed command response")
		}
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("asynchronous exporter blocked commands")
	}
	started = time.Now()
	_ = telemetry.Shutdown(context.Background())
	if time.Since(started) > 4*time.Second {
		t.Fatal("shutdown exceeded bound")
	}
}

func BenchmarkTelemetryRequest(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled_collector_down"
		}
		b.Run(name, func(b *testing.B) {
			telemetry, err := NewTelemetry(context.Background(), TelemetryConfig{Enabled: enabled, ServiceName: "local-benchmark", Endpoint: "127.0.0.1:1"})
			if err != nil {
				b.Fatal(err)
			}
			handler := telemetry.HTTP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			request := httptest.NewRequest("GET", "/api/health", nil)
			b.ResetTimer()
			for range b.N {
				handler.ServeHTTP(httptest.NewRecorder(), request)
			}
			b.StopTimer()
			_ = telemetry.Shutdown(context.Background())
		})
	}
}
