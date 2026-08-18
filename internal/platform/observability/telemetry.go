// Package observability provides bounded, privacy-safe OpenTelemetry signals.
// It never labels telemetry with account IDs, tenant IDs, amounts, tokens, or
// other customer data. PostgreSQL remains the financial source of truth.
package observability

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "ledgersync/platform"

type TelemetryConfig struct {
	Enabled     bool
	ServiceName string
	Endpoint    string // private OTLP/HTTP host:port, for example otel-collector:4318
}

type Telemetry struct {
	enabled bool
	tracer  trace.Tracer

	requests          metric.Int64Counter
	boundaryCalls     metric.Int64Counter
	boundaryDuration  metric.Float64Histogram
	transferOutcomes  metric.Int64Counter
	outboxAge         metric.Float64Histogram
	ryewViolations    metric.Int64Counter
	reconciliationRun metric.Int64Counter
	shutdown          func(context.Context) error
}

func NewTelemetry(ctx context.Context, cfg TelemetryConfig) (*Telemetry, error) {
	telemetry := &Telemetry{tracer: trace.NewNoopTracerProvider().Tracer(instrumentationName), shutdown: func(context.Context) error { return nil }}
	if !cfg.Enabled {
		return telemetry, nil
	}
	if cfg.ServiceName == "" || cfg.Endpoint == "" {
		return nil, errors.New("enabled telemetry requires service name and private OTLP endpoint")
	}
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(cfg.Endpoint), otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(cfg.Endpoint), otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, err
	}
	resources, err := resource.Merge(resource.Default(), resource.NewWithAttributes("", attribute.String("service.name", cfg.ServiceName)))
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(resources))
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)), sdkmetric.WithResource(resources))
	meter := meterProvider.Meter(instrumentationName)
	requests, err := meter.Int64Counter("ledgersync.http.requests")
	if err != nil {
		return nil, err
	}
	boundaryCalls, err := meter.Int64Counter("ledgersync.boundary.operations")
	if err != nil {
		return nil, err
	}
	boundaryDuration, err := meter.Float64Histogram("ledgersync.boundary.duration", metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}
	transferOutcomes, err := meter.Int64Counter("ledgersync.transfer.outcomes")
	if err != nil {
		return nil, err
	}
	outboxAge, err := meter.Float64Histogram("ledgersync.outbox.event_age", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	ryewViolations, err := meter.Int64Counter("ledgersync.ryew.violations")
	if err != nil {
		return nil, err
	}
	reconciliationRuns, err := meter.Int64Counter("ledgersync.reconciliation.runs")
	if err != nil {
		return nil, err
	}
	telemetry.enabled, telemetry.tracer = true, tracerProvider.Tracer(instrumentationName)
	telemetry.requests, telemetry.boundaryCalls, telemetry.boundaryDuration = requests, boundaryCalls, boundaryDuration
	telemetry.transferOutcomes, telemetry.outboxAge = transferOutcomes, outboxAge
	telemetry.ryewViolations, telemetry.reconciliationRun = ryewViolations, reconciliationRuns
	telemetry.shutdown = func(shutdownCtx context.Context) error {
		return errors.Join(meterProvider.Shutdown(shutdownCtx), tracerProvider.Shutdown(shutdownCtx))
	}
	return telemetry, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil || t.shutdown == nil {
		return nil
	}
	return t.shutdown(ctx)
}

func (t *Telemetry) Start(ctx context.Context, name string) (context.Context, trace.Span) {
	if t == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, name)
}

func (t *Telemetry) ObserveBoundary(ctx context.Context, boundary, operation string, started time.Time, err error) {
	if t == nil || !t.enabled {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
		trace.SpanFromContext(ctx).RecordError(err)
	}
	attrs := metric.WithAttributes(attribute.String("boundary", boundary), attribute.String("operation", operation), attribute.String("outcome", outcome))
	t.boundaryCalls.Add(ctx, 1, attrs)
	t.boundaryDuration.Record(ctx, float64(time.Since(started))/float64(time.Millisecond), attrs)
}

func (t *Telemetry) ObserveRYEWViolation(ctx context.Context) {
	if t != nil && t.enabled {
		t.ryewViolations.Add(ctx, 1)
	}
}

func (t *Telemetry) ObserveTransfer(ctx context.Context, status string, replayed bool) {
	if t == nil || !t.enabled {
		return
	}
	t.transferOutcomes.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status), attribute.Bool("replayed", replayed)))
}

func (t *Telemetry) ObserveOutboxAge(ctx context.Context, age time.Duration) {
	if t != nil && t.enabled {
		t.outboxAge.Record(ctx, age.Seconds())
	}
}

func (t *Telemetry) ObserveReconciliation(ctx context.Context, status string) {
	if t != nil && t.enabled {
		t.reconciliationRun.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	}
}

func (t *Telemetry) HTTP(next http.Handler) http.Handler {
	if t == nil {
		return next
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx, span := t.Start(request.Context(), "http.server")
		defer span.End()
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request.WithContext(ctx))
		if t.enabled {
			outcome := "ok"
			if recorder.status >= 500 {
				outcome = "error"
			}
			attrs := metric.WithAttributes(attribute.String("method", request.Method), attribute.Int("status_code", recorder.status), attribute.String("outcome", outcome))
			t.requests.Add(ctx, 1, attrs)
			t.boundaryDuration.Record(ctx, float64(time.Since(started))/float64(time.Millisecond), metric.WithAttributes(attribute.String("boundary", "http"), attribute.String("operation", "request"), attribute.String("outcome", outcome)))
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
