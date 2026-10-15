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
	"go.opentelemetry.io/otel/trace/noop"
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

	requests                     metric.Int64Counter
	boundaryCalls                metric.Int64Counter
	boundaryDuration             metric.Float64Histogram
	httpRouteDuration            metric.Float64Histogram
	transferOutcomes             metric.Int64Counter
	outboxAge                    metric.Float64Histogram
	ryewViolations               metric.Int64Counter
	reconciliationRun            metric.Int64Counter
	recoveryOperations           metric.Int64Counter
	deadWork                     metric.Int64Counter
	retentionRuns                metric.Int64Counter
	retentionDeleted             metric.Int64Counter
	redisStreamDepth             metric.Int64Gauge
	redisConsumerLag             metric.Int64Gauge
	webhookKeyResolves           metric.Int64Counter
	webhookKeyDuration           metric.Float64Histogram
	webhookKeyLockWait           metric.Float64Histogram
	webhookKeyEntries            metric.Int64Gauge
	webhookKeyEvictions          metric.Int64Counter
	workerHeartbeat              metric.Int64Gauge
	workerProgressAge            metric.Float64Gauge
	workerActive                 metric.Int64Gauge
	workerLastStarted            metric.Int64Gauge
	workerLastCompleted          metric.Int64Gauge
	workerLastFailure            metric.Int64Gauge
	committedMetadataUnavailable metric.Int64Counter
	committedWriteFailures       metric.Int64Counter
	shutdown                     func(context.Context) error
}

func NewTelemetry(ctx context.Context, cfg TelemetryConfig) (*Telemetry, error) {
	telemetry := &Telemetry{tracer: noop.NewTracerProvider().Tracer(instrumentationName), shutdown: func(context.Context) error { return nil }}
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
	httpRouteDuration, err := meter.Float64Histogram(
		"ledgersync.http.route.duration",
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 10, 25, 50, 100, 200, 500, 750, 1000, 2000, 5000),
	)
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
	recoveryOperations, err := meter.Int64Counter("ledgersync.recovery.operations")
	if err != nil {
		return nil, err
	}
	deadWork, err := meter.Int64Counter("ledgersync.recovery.dead_work")
	if err != nil {
		return nil, err
	}
	retentionRuns, err := meter.Int64Counter("ledgersync.retention.runs")
	if err != nil {
		return nil, err
	}
	retentionDeleted, err := meter.Int64Counter("ledgersync.retention.deleted_rows")
	if err != nil {
		return nil, err
	}
	redisStreamDepth, err := meter.Int64Gauge("ledgersync.redis.stream_depth")
	if err != nil {
		return nil, err
	}
	redisConsumerLag, err := meter.Int64Gauge("ledgersync.redis.consumer_lag")
	if err != nil {
		return nil, err
	}
	webhookKeyResolves, err := meter.Int64Counter("ledgersync.webhook_key_cache.resolutions")
	if err != nil {
		return nil, err
	}
	webhookKeyDuration, err := meter.Float64Histogram("ledgersync.webhook_key_cache.resolve_duration", metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}
	webhookKeyLockWait, err := meter.Float64Histogram("ledgersync.webhook_key_cache.lock_wait", metric.WithUnit("ms"))
	if err != nil {
		return nil, err
	}
	webhookKeyEntries, err := meter.Int64Gauge("ledgersync.webhook_key_cache.entries")
	if err != nil {
		return nil, err
	}
	webhookKeyEvictions, err := meter.Int64Counter("ledgersync.webhook_key_cache.evictions")
	if err != nil {
		return nil, err
	}
	workerHeartbeat, err := meter.Int64Gauge("ledgersync.worker.heartbeat_unix", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	workerProgressAge, err := meter.Float64Gauge("ledgersync.worker.progress_age", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	workerActive, err := meter.Int64Gauge("ledgersync.worker.active")
	if err != nil {
		return nil, err
	}
	workerLastStarted, err := meter.Int64Gauge("ledgersync.worker.last_started_unix", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	workerLastCompleted, err := meter.Int64Gauge("ledgersync.worker.last_completed_unix", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	workerLastFailure, err := meter.Int64Gauge("ledgersync.worker.last_failure_unix", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	committedMetadataUnavailable, err := meter.Int64Counter("ledgersync.committed_response.metadata_unavailable")
	if err != nil {
		return nil, err
	}
	committedWriteFailures, err := meter.Int64Counter("ledgersync.committed_response.write_failures")
	if err != nil {
		return nil, err
	}
	telemetry.enabled, telemetry.tracer = true, tracerProvider.Tracer(instrumentationName)
	telemetry.requests, telemetry.boundaryCalls, telemetry.boundaryDuration, telemetry.httpRouteDuration = requests, boundaryCalls, boundaryDuration, httpRouteDuration
	telemetry.transferOutcomes, telemetry.outboxAge = transferOutcomes, outboxAge
	telemetry.ryewViolations, telemetry.reconciliationRun = ryewViolations, reconciliationRuns
	telemetry.recoveryOperations, telemetry.deadWork, telemetry.retentionRuns, telemetry.retentionDeleted = recoveryOperations, deadWork, retentionRuns, retentionDeleted
	telemetry.redisStreamDepth, telemetry.redisConsumerLag = redisStreamDepth, redisConsumerLag
	telemetry.webhookKeyResolves, telemetry.webhookKeyDuration, telemetry.webhookKeyLockWait = webhookKeyResolves, webhookKeyDuration, webhookKeyLockWait
	telemetry.webhookKeyEntries, telemetry.webhookKeyEvictions = webhookKeyEntries, webhookKeyEvictions
	telemetry.workerHeartbeat, telemetry.workerProgressAge, telemetry.workerActive = workerHeartbeat, workerProgressAge, workerActive
	telemetry.workerLastStarted, telemetry.workerLastCompleted, telemetry.workerLastFailure = workerLastStarted, workerLastCompleted, workerLastFailure
	telemetry.committedMetadataUnavailable, telemetry.committedWriteFailures = committedMetadataUnavailable, committedWriteFailures
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

func (t *Telemetry) ObserveRecovery(ctx context.Context, kind, action string, err error) {
	if t == nil || !t.enabled {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	t.recoveryOperations.Add(ctx, 1, metric.WithAttributes(attribute.String("kind", kind), attribute.String("action", action), attribute.String("outcome", outcome)))
}

func (t *Telemetry) ObserveDeadWork(ctx context.Context, kind string) {
	if t != nil && t.enabled {
		t.deadWork.Add(ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
	}
}

func (t *Telemetry) ObserveRetention(ctx context.Context, mode string, deleted int64) {
	if t != nil && t.enabled {
		attributes := metric.WithAttributes(attribute.String("mode", mode))
		t.retentionRuns.Add(ctx, 1, attributes)
		t.retentionDeleted.Add(ctx, deleted, attributes)
	}
}

func (t *Telemetry) ObserveRedisStream(ctx context.Context, depth, lag int64) {
	if t != nil && t.enabled {
		t.redisStreamDepth.Record(ctx, depth)
		t.redisConsumerLag.Record(ctx, lag)
	}
}

func (t *Telemetry) ObserveWebhookKeyCacheResolve(ctx context.Context, outcome string, duration, lockWait time.Duration) {
	if t == nil || !t.enabled {
		return
	}
	switch outcome {
	case "hit", "miss", "concurrent_hit", "upstream_error", "cancelled":
	default:
		outcome = "unknown"
	}
	attributes := metric.WithAttributes(attribute.String("outcome", outcome))
	t.webhookKeyResolves.Add(ctx, 1, attributes)
	t.webhookKeyDuration.Record(ctx, float64(duration)/float64(time.Millisecond), attributes)
	t.webhookKeyLockWait.Record(ctx, float64(lockWait)/float64(time.Millisecond), attributes)
}

func (t *Telemetry) ObserveWebhookKeyCacheEntries(ctx context.Context, count int) {
	if t != nil && t.enabled {
		t.webhookKeyEntries.Record(ctx, int64(count))
	}
}

func (t *Telemetry) ObserveWebhookKeyCacheEviction(ctx context.Context, reason string) {
	if t == nil || !t.enabled {
		return
	}
	if reason != "expired" && reason != "capacity" {
		reason = "unknown"
	}
	t.webhookKeyEvictions.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

func (t *Telemetry) ObserveWorkerProgress(ctx context.Context, report WorkerProgressReport) {
	if t == nil || !t.enabled || !isWorkerQueue(report.Queue) {
		return
	}
	attributes := metric.WithAttributes(attribute.String("queue", report.Queue))
	t.workerHeartbeat.Record(ctx, report.HeartbeatAt.Unix(), attributes)
	t.workerProgressAge.Record(ctx, report.ProgressAge.Seconds(), attributes)
	active := int64(0)
	if report.Active {
		active = 1
	}
	t.workerActive.Record(ctx, active, attributes)
	t.workerLastStarted.Record(ctx, unixOrZero(report.LastStartedAt), attributes)
	t.workerLastCompleted.Record(ctx, unixOrZero(report.LastCompletedAt), attributes)
	lastFailure := int64(0)
	if report.FailureAge != nil {
		lastFailure = report.HeartbeatAt.Add(-*report.FailureAge).Unix()
	}
	t.workerLastFailure.Record(ctx, lastFailure, attributes)
}

func (t *Telemetry) ObserveCommittedResponseMetadataUnavailable(ctx context.Context, commandKind string) {
	if t != nil && t.enabled {
		t.committedMetadataUnavailable.Add(ctx, 1, metric.WithAttributes(attribute.String("command_kind", committedCommandKind(commandKind))))
	}
}

func (t *Telemetry) ObserveCommittedResponseWriteFailure(ctx context.Context, commandKind string) {
	if t != nil && t.enabled {
		t.committedWriteFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("command_kind", committedCommandKind(commandKind))))
	}
}

func committedCommandKind(commandKind string) string {
	switch commandKind {
	case "transfer", "funding", "correction":
		return commandKind
	default:
		return "unknown"
	}
}

func isWorkerQueue(queue string) bool {
	for _, allowed := range workerQueues {
		if queue == allowed {
			return true
		}
	}
	return false
}

func unixOrZero(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
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
		routedRequest := request.WithContext(ctx)
		next.ServeHTTP(recorder, routedRequest)
		if t.enabled {
			durationMilliseconds := float64(time.Since(started)) / float64(time.Millisecond)
			outcome := "ok"
			if recorder.status >= 500 {
				outcome = "error"
			}
			attrs := metric.WithAttributes(attribute.String("method", request.Method), attribute.Int("status_code", recorder.status), attribute.String("outcome", outcome))
			t.requests.Add(ctx, 1, attrs)
			t.boundaryDuration.Record(ctx, durationMilliseconds, metric.WithAttributes(attribute.String("boundary", "http"), attribute.String("operation", "request"), attribute.String("outcome", outcome)))
			if operation := performanceRouteOperation(routedRequest.Pattern); operation != "" {
				t.httpRouteDuration.Record(ctx, durationMilliseconds, metric.WithAttributes(attribute.String("operation", operation), attribute.String("outcome", outcome)))
			}
		}
	})
}

// performanceRouteOperation is deliberately a closed set. ServeMux patterns
// are code-defined and contain no object identifiers, but the allowlist also
// prevents future wildcard or raw-path changes from creating sensitive or
// unbounded metric labels.
func performanceRouteOperation(pattern string) string {
	switch pattern {
	case "POST /api/transfers":
		return "transfer_command"
	case "GET /api/accounts/{accountID}/balance":
		return "balance_read"
	case "GET /api/local/diagnostics":
		return "diagnostics_read"
	case "GET /api/events":
		return "events_list"
	case "GET /api/events/{eventID}":
		return "events_detail"
	case "POST /api/accounts":
		return "account_create"
	case "PATCH /api/accounts/{accountID}":
		return "account_patch"
	default:
		return ""
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
