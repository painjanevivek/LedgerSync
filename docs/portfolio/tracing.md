# Diagnostic tracing

Current repair status: [audit repair register](audit-remediation.md). The following R06 measurements retain their original source. The new pool/lock and receiver changes need an affected runtime repeat before a new integrated trace claim.

R06 extends the existing OpenTelemetry adapter. PostgreSQL remains the financial authority. A trace is a diagnostic record. It grants no tenant, role, approval, replay, or money authority.

## Request and delayed work

The BFF generates a fresh opaque W3C version-zero context for a private API call. Its exported `bff.private_context` span measures credential and actor-assertion preparation. The upstream API span measures the command. The preparation span ends before the private fetch starts; it does not claim the duration of the complete browser request. The BFF does not accept browser baggage or use a tenant, account, request key, amount or session ID as a trace ID. Strict account, reconciliation and replay command header builders preserve only a valid diagnostic header.

The existing Next.js instrumentation register initializes the Node provider only when telemetry is enabled. The narrow processor exports only the named manual BFF span. It removes attributes, events, links, parent metadata and framework auto-spans, and fixes the resource to the service name. The already locked OpenTelemetry packages are explicit runtime dependencies. Their versions were preserved. The final dependency review remains part of R13.

The Go HTTP boundary reads only `traceparent`. It discards baggage and trace state. The local sampler selects ten percent by trace ID; a remote sample flag cannot force full sampling. Existing database, cache and Redis spans inherit request context. A disabled adapter never ends its caller's span.

The tenant transaction sets and clears diagnostic context with tenant and actor context. Migration 000061 stores an optional immutable origin beside an outbox event. It does not change the event payload or financial evidence. Old rows remain NULL. A worker reads the origin only inside its existing authorized scope. Each Redis publish and webhook dispatch has a separate attempt span with an origin link. Event IDs and lease generations keep their existing meanings. Redis carries a private diagnostic header separately from its public payload. Projection attempts link to the publication attempt. Legacy or invalid context produces an unlinked span; it does not reject valid financial evidence.

The event detail query first verifies current record access. It returns only the opaque origin trace ID. The console explains missing historical context, sampling, and retention. It shows a viewer link only when the BFF has a valid `LEDGERSYNC_TRACE_VIEWER_BASE_URL`. The same-origin link checks current event authority again. The configured viewer must enforce its own access policy. Configuration accepts HTTPS or loopback HTTP, without credentials, query strings or fragments. The expected viewer path is `<base>/trace/<trace-id>`. No viewer is configured or deployed in the preserved local runtime.

## Privacy and bounds

Exported resource data contains only the configured service name. Arbitrary environment resource attributes and host or user metadata are excluded. Span names and metric labels come from code-defined operations. Raw SQL, statement parameters, business IDs, raw paths, request bodies, errors, personal data, tokens and amounts are not attributes. Boundary failures set `operation_failed`; they do not export raw exception text. Never add a raw `RecordError` call at a financial boundary.

| Control | Local setting |
|---|---|
| Trace sampling | Ten percent by trace ID |
| SDK pending trace queue | 512 spans; non-blocking drop at capacity |
| SDK export batch | At most 64 spans |
| Trace batch wait and export timeout | One second each |
| Metrics interval and export timeout | 15 seconds and one second |
| Go export retries | Disabled; later batches can recover |
| Telemetry shutdown | At most three seconds, or the caller's earlier deadline |
| Node BFF trace queue and batch | 512 spans and 64 spans |
| Node BFF batch wait and export deadline | One second each; exporter concurrency one |
| Node BFF transport retry | Existing SDK retry logic stays inside its one-second deadline |
| Node BFF shutdown wait | At most three seconds; loss remains possible on forced exit |
| Collector batch | Target 64; maximum 128 |
| Collector trace export | One-second timeout, queue 512, retry disabled |

The collector stays on the private observability network. Its existing memory limiter remains. A failed collector can lose diagnostics. It cannot decide a financial result. Collector configuration is parsed locally; running collector compatibility belongs to the final package check.

## Verification and measured limit

Tests cover diagnostic-only headers, forbidden metadata, raw-error exclusion, independent attempt links, malformed context, parent-span lifetime, down-collector requests, and bounded shutdown. Isolated database tests cover commit, retries, immutable origin and connection reuse. Existing tenant, worker crash, webhook and Lamport tests remain part of qualification.

Three 10,000-operation local microbenchmarks on Windows amd64, Intel i9-13980HX, measured 298–381 ns per disabled request and 1,460–1,647 ns with telemetry enabled and the collector down. The enabled path allocated approximately 1.7 KiB and 17 objects per request. This is a synthetic handler benchmark, run beside other local tests. It does not measure database latency, production capacity, or exporter delivery success. Raw results stay in the private R06 evidence directory. R09 measures whole-workload capacity.

## Audit additions — 17 October 2026

The API, worker and optional cron worker register their own in-process database pool counters. Labels are the fixed pool role and connection state. Wait count and wait duration are cumulative counters; duration uses seconds. These callbacks use `database/sql` statistics and make no extra query. They unregister before pool closure. They export no DSN, tenant or account.

The existing tenant-policy advisory-lock call records its duration and a fixed success/error label. This includes a database round trip. It is not a measurement of every PostgreSQL row lock. The local capacity harness still samples database lock waiters separately. No lock order, policy lock, retry, financial writer or transaction rule was removed.

Webhook dispatch now sends only an opaque `traceparent` for receiver diagnostics. It remains outside the financial event envelope and signature input. It does not grant receiver authority. The example receiver can save a bounded private sibling `.trace.jsonl` file when `receiver.tracePath` is explicitly configured. It saves only span name, opaque trace/span ancestry and result class. Diagnostic loss cannot change signature verification or the receiver's durable effect.

Use a verified native promtool 3.15.0 executable for the actual rule evaluator:

```powershell
node scripts/portfolio/verify-alerts.mjs
```

First set `LEDGERSYNC_PROMTOOL_PATH` to the checksum-verified local executable. The command starts no Prometheus server and sends no notification. Tests cover reconciliation mismatch, a delayed stalled-worker alert, recovery and negative conditions. All 16 alert runbook paths must exist. The [official rule-test format](https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/) defines the evaluator input. Later operating alert destinations remain separate.

The [capture checker](../../scripts/portfolio/trace-evidence.mjs) requires the BFF, API, database, publisher, failed/successful delivery attempts and receiver. It requires separate referenced financial evidence. Its unit fixtures are not runtime captures. Full command capture remains open while PostgreSQL/Redis and Docker are stopped. No trace alone proves a financial result.
