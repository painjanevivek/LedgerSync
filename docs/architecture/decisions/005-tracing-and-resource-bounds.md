# ADR-005: Extend existing telemetry and make resource bounds explicit

Date: 14 October 2026. Status: design accepted; end-to-end implementation pending R06.

Reuse `internal/platform/observability` and `deploy/observability`. A new telemetry framework is not required. Request references, business IDs, trace IDs, event IDs, attempts, and Lamport clocks retain different meanings.

Propagate trusted diagnostic context through Next.js and Go. Diagnostic headers grant no authority. Use documented parent or span-link relationships for asynchronous work. Keep immutable event identity across retries and create separate attempt spans.

Allowlist exported fields. Exclude secrets, raw financial values, personal data, and request payloads. Keep collectors private. Bound export queues, retry delay, and shutdown. An export failure must not indefinitely block a financial command. Trace links require current record authority and viewer access.

## Existing bounds and next owner

| Boundary | Current source | Ownership and follow-up |
|---|---|---|
| API header/read/write/idle timeouts | `internal/platform/config/config.go` | Platform; current defaults are 3s/10s/15s/60s. Preserve validated configuration. |
| Database pool and connection lifetime | `internal/platform/db/postgres.go` | Platform; fallback maximum open 4, idle 2, lifetime 30 minutes. R09 must budget the sum across replicas. |
| Startup retry | `internal/platform/config/config.go` | Bootstrap; bounded startup defaults to 90 seconds. |
| HTTP body and field limits | HTTP handlers and middleware | API owner; R02/R03 packet inventory must record each changed operation's actual limits. Do not invent one global cap. |
| Result pages and export limits | Application query contracts and handlers | Feature owner; retain server pagination and bounded exports. |
| Queue claim expiry/renewal | Database adapters and application lease package | Worker owner; use database time and fencing generations. |
| Webhook request timeout and destination checks | Webhook dispatcher and verification modules | Delivery owner; retain SSRF controls and bounded retry. |
| Telemetry queue and sampling limits | Collector and telemetry configuration | Observability owner; qualify overhead and export failure in R06. |

Values above were read from source, not live container secrets. R00 does not dump runtime environment values. The complete operation-level bounds inventory remains a packet deliverable when the affected contract is changed.

## R06 implementation

R06 adds opaque server diagnostic context, scoped immutable event origins, new linked delivery/projection attempts, privacy-safe failure status, and explicit export bounds. Event details reauthorize trace access. An absent viewer or historical trace is explicit. See [tracing controls and measurements](../../portfolio/tracing.md). No collector or hosted viewer is deployed in the preserved runtime. The final package validates collector compatibility before enablement.
