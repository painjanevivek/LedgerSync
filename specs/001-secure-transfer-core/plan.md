# LedgerSync Secure Transfer Core — Detailed Implementation Plan

**Specification**: [spec.md](spec.md)  
**Date**: 2026-08-18  
**Status**: Ready for task breakdown

## Executive decision

LedgerSync must move from a non-runnable learning prototype to a deliberately small, production-grade financial core. The optimal starting shape is a modular Go application for all financial writes, a separate outbox worker for post-commit work, a Next.js dashboard/BFF as the only public boundary, PostgreSQL as the sole financial authority, and Redis as a versioned disposable read cache.

```text
Browser --HTTPS--> Next.js dashboard + BFF --private request--> Go Transfer Core
                                                               |       |
                                                               |       +--> PostgreSQL primary
                                                               |              ledger, transfers, projections,
                                                               |              idempotency, outbox, audit records
                                                               v
                                                        Go outbox worker --> Redis Streams/cache
```

In plain language: the database is the permanent book of record; ledger entries explain every change; Redis makes reads fast but is never trusted with the final answer. If cache data is too old after a transfer, the system safely gets the authoritative answer instead.

Do **not** scale the current four demo services. Their separation creates operational cost without real business boundaries. Retain the Python simulator only as a development/admin fault-injection tool, disabled in production.

## Current-state corrections

| Area | Present issue | Correction before feature work |
|---|---|---|
| Build | Invalid Go comments, missing imports/variables, incompatible module imports, no generated protocol types. | Replace with one root Go module and a compile-tested `cmd/api` plus `cmd/outbox-worker` layout. |
| Containers | Missing `go.sum`, Redis config, dashboard Dockerfile and dependent health checks. | Add reproducible Dockerfiles, lockfiles, health/readiness endpoints and verified Compose profiles. |
| Browser API | UI calls HTTP-like URLs on a raw gRPC port; schemas do not match. | Publish a versioned same-origin BFF JSON contract. |
| Money | Float/double values appear in Go, protocol, JavaScript and Python paths. | Store integer minor units with currency; parse decimal strings once at input. |
| Accounting | Mutable account balance only; no transfer/ledger history. | Add immutable double-entry postings, transfer records, balance projections and reconciliation. |
| Reliability | Direct post-commit Redis publish can be lost; cache lacks version/TTL; RYEW is a random token. | Use transactional outbox, at-least-once idempotent consumers, versioned cache, signed minimum-version requirement. |
| Security | Hard-coded secrets, public Postgres/Redis/internal ports, demo auth, public simulation, raw token display. | OIDC, BFF session, server-side authorization, private networking, managed secrets, redacted logs and admin-only diagnostics. |
| Operations | Runtime DDL/seeding, cosmetic replica, placeholder tests. | Versioned migrations, tested backups/restores, real test pyramid and observability. |

## Architecture decisions

| Decision | Choice | Reason | Defer |
|---|---|---|---|
| Service shape | Modular monolith + outbox worker | One financial transaction and lower operational risk. | Account/auth/cache/event microservices. |
| Public boundary | Next.js BFF, HTTPS, same-origin | No browser-to-gRPC/CORS problem; sessions and CSRF at one boundary. | Direct browser access to internal services. |
| Money | `BIGINT` minor units + ISO currency | Exact and simple. `double`/float are inexact. | FX/cross-currency transfers. |
| Accounting | Immutable double-entry ledger + rebuildable balance projection | Auditability and reconciliation without slow ledger scans on each read. | Editable history. |
| Duplicate safety | Idempotency key + validated-request fingerprint + stored outcome | Makes retry after a lost response safe. | Client-only duplicate prevention. |
| RYEW | Account balance versions and short-lived signed requirements | O(1) proof a cache is sufficiently fresh, with bounded primary fallback. | Stream scans / unbounded waiting. |
| Events | Transactional outbox then Redis Streams | Transfer does not lose an event between commit and publish. | Kafka/NATS until scale/fan-out requires them. |
| Identity | External OIDC provider, BFF secure session | Avoid custom password/token lifecycle. | Demo auth service. |
| Production platform | Managed Postgres/PITR, secrets manager, private network | Highest financial-safety return. | Kubernetes, mesh, multi-region writes. |

## Recommended source layout

```text
cmd/api/                    # Transfer Core executable
cmd/outbox-worker/          # Durable event relay/cache projector
internal/domain/{money,account,transfer,ledger}/
internal/application/{transfers,accounts,reconciliation}/
internal/platform/{config,db,cache,events,identity,observability,security}/
internal/transport/http/{handlers,middleware}/
migrations/                 # Only controlled schema changes
contracts/                  # OpenAPI and versioned event schemas
web/                        # Next.js UI + BFF
tests/{unit,integration,contract,e2e,performance,fault}/
docs/{adr,runbooks,threat-model}/
deploy/{docker,compose,observability}/
```

Domain code knows financial rules, not HTTP/Redis/UI. Application code orchestrates use cases. Platform adapters own external integrations. This makes changes modular while keeping the financial transaction in one reliable boundary.

## Phased plan

### Phase 0 — Governance and runnable baseline

**What it means**: make every developer and CI run the same trustworthy skeleton before it handles money.

1. Mark the present runtime as an archived demo reference, not the production base.
2. Create a root Go module and generated-type workflow; eliminate incompatible nested modules and invalid entry points.
3. Create `cmd/api` and `cmd/outbox-worker`; do not migrate unused demo services into production.
4. Add `.gitignore`, `.env.example`, configuration validation, pinned tools, Make/Task commands and onboarding documentation.
5. Add non-root, minimal images for API, worker and web; pin release image digests.
6. Rebuild Compose with normal and diagnostic profiles. Publish only web/API in development; keep Postgres, Redis, worker and diagnostic ports private.
7. Implement `/healthz` and `/readyz`; use health checks only where supported.
8. Create a real project constitution and ADRs for money, ledger, outbox, RYEW, API boundary and identity.
9. Add baseline CI: format, compile, lint, unit test and container build.

**Exit evidence**: clean checkout starts consistently; no secrets live in source; every image builds; internal stores are not exposed by default.

### Phase 1 — Financial data foundation

**What it means**: every balance change becomes a permanent and explainable accounting record.

1. Define exact `Money`: positive integer `amount_minor` plus currency. Browser input remains decimal text and is validated against a currency-exponent policy; JavaScript never handles money as `Number`.
2. Establish v1 scope: internal, same-currency transfers. Explicitly reject zero/negative, excessive precision, unsupported currency and cross-currency movement.
3. Add migration tool and ordered history. Application startup must not create schema or seed production data.
4. Create accounts, account ownership, journal transactions, ledger postings, transfer request/outcome, balance projections, idempotency requests, outbox events and audit events.
5. Require a balanced debit and credit set per journal transaction. Postings are append-only; corrections are new linked compensating entries.
6. Add database constraints, foreign keys, unique keys and value/status checks. Give the service database role no permission to update/delete posted ledger entries.
7. Update balance projection and increment `balance_version` inside the same transaction as postings.
8. Implement reconciliation to compare ledger totals to projections and preserve result evidence.
9. Put sample records in development fixtures only.

**Exit evidence**: migration/rollback compatibility is tested; no float crosses a financial path; every fixture reconciles exactly.

### Phase 2 — Identity, authorization and private boundary

**What it means**: define who can see or move which money, and remove access to internal systems.

1. Select an OIDC provider. Use authorization-code flow with PKCE; production uses managed identity, local work uses a narrow development adapter.
2. Create BFF sessions with `HttpOnly`, `Secure`, appropriate `SameSite` cookies. Browser JavaScript never stores refresh tokens or RYEW requirements.
3. Implement only same-origin browser routes: owned accounts, balance, transaction history and transfer submission.
4. Pass a verified principal, role/scope, correlation ID and trace context to the private API.
5. Enforce object-level authorization in every read/write: UI visibility is never authorization.
6. Add account-holder and operator roles; simulation routes require operator role plus non-production configuration.
7. Add CSRF protection for session writes, request-size/schema limits, rate limits by identity/IP, timeouts and safe error envelopes.
8. Audit transfer attempts/outcomes, denials, operator actions, key changes, restores and deployments. Redact tokens, passwords, PII and raw balances.

**Exit evidence**: modified account IDs cannot bypass server-side ownership checks; normal users cannot access diagnostics; no browser calls internal ports.

### Phase 3 — Correct transfer command

**What it means**: transfers stay correct when users retry, double-click or act concurrently.

1. Freeze the transfer HTTP contract before handlers; require `Idempotency-Key` for each mutation.
2. Validate identity, ownership, distinct accounts, active status, currency, exact positive amount, limits and canonical request fingerprint.
3. Persist idempotency by tenant/actor/operation/key. Matching retry replays the saved response; same key with a different request is rejected; in-progress duplicate behavior is bounded and documented.
4. Start one short transaction and lock source/destination projections in ascending immutable ID order.
5. Check funds only after locks. Do not call Redis, HTTP or a broker within the transaction.
6. Create the transfer/journal, balanced postings, projection balances/versions, stable idempotency outcome and one outbox event per affected account, then commit together.
7. Return transfer ID only after commit. A lost response is resolved by retrying the same idempotency key.
8. Retry only known serialization/deadlock failures with bounded backoff; never replay ambiguous writes blindly.

**Exit evidence**: concurrency tests never overdraw; retries never double debit; every completed transfer is balanced and reconciled.

### Phase 4 — Outbox, cache and real read-your-writes

**What it means**: a customer sees the result of their completed transfer even when the fast display cache is behind.

1. Outbox event fields: immutable event/transfer/account IDs, currency, resulting minor balance, resulting version and occurrence time.
2. Worker claims small batches safely, publishes only committed events, marks success after publication, retries with backoff/jitter and exposes old/dead work.
3. Use Redis Streams consumer groups with acknowledgement/pending recovery. Delivery is at-least-once, so event ID/version makes consumers idempotent.
4. Cache `{balance_minor,currency,balance_version,cached_at,expiry}`. Apply incoming event only if it is not older than the cached version.
5. After commit issue a short-lived signed consistency requirement containing only user, permitted account(s), minimum version(s), expiry, audience and key metadata.
6. Keep it server-side in BFF session; never display, log or put it in local storage. It is not authorization; every read still checks ownership.
7. For a requirement-bearing read, use cache only when `cached_version >= required_version`; otherwise wait 100–250 ms initially, then read PostgreSQL primary.
8. If primary is unavailable, return a truthful temporary error. If Redis fails, use primary and rebuild later.
9. Add replay/rebuild tools and metrics: outbox age, worker error, consumer lag, cache version miss, primary fallback, RYEW violations (must be zero).

**Exit evidence**: delay, replay, worker restart, Redis flush and response-loss fault tests always return the transfer-produced version or a clear availability error.

### Phase 5 — Customer UX and accessibility

**What it means**: customers see a simple, safe transfer journey rather than internal engineering details.

1. Build sign-in, owned accounts, balance, transfer form, confirmation, transaction history and sign-out.
2. Parse/format exact money with canonical helpers; never use `parseFloat`.
3. Retain a generated client idempotency key until a final transfer result exists; prevent accidental duplicate submission while allowing safe retry.
4. Show transfer ID, amount, accounts and final result. Never show raw tokens, cache states, event IDs or replication jargon.
5. Show validating, processing, completed, invalid, insufficient-funds, safely-retried, refreshing and temporary-unavailable states in plain language.
6. Place simulation only on an authorized developer/admin page.
7. Verify keyboard flow, focus return, labels, screen-reader status, contrast, mobile layout and recoverable errors.

**Exit evidence**: browser E2E tests cover success/error/retry; accessibility tests pass; 95% of representative users can complete a normal transfer on first attempt.

### Phase 6 — Security and network hardening

**What it means**: protect money and information from abuse, exposure and unsafe operations.

1. Public ingress is HTTPS gateway/dashboard only; API, worker, Postgres, Redis and diagnostics remain private.
2. Add TLS, HSTS/security headers, strict CORS only if approved, private internal network controls and platform-appropriate service encryption.
3. Use `.env` only locally. Adopt secrets manager/KMS before shared environments; define rotation, access audit and emergency revocation.
4. Run containers non-root with dropped privileges/read-only filesystems where practical; pin images.
5. Add WAF/edge throttling and egress restrictions before public launch.
6. Enforce logging redaction through tests; perform threat model and security review.

**Exit evidence**: authorization, secret, dependency, image, IaC and log-redaction checks pass; security review confirms exposure is private by default.

### Phase 7 — Observability, backup and recovery

**What it means**: operators can detect problems quickly and restore evidence-backed financial data after failures.

1. Instrument BFF, API, worker, database/cache calls and events with OpenTelemetry traces/metrics and correlation IDs.
2. Monitor transfer outcomes/p50-p95-p99, lock waits, idempotency replay/conflict, authorization denials, pool pressure, cache/version fallbacks, outbox age, stream pending work and worker retries.
3. Alert on RYEW violations, reconciliation mismatch, aged outbox, backup age, restore failure and customer-impacting SLO burn. Every alert has an action-oriented runbook.
4. Create runbooks for database/Redis outage, outbox backlog, duplicate delivery, reconciliation mismatch, idempotency dispute, secret compromise and restore failure.
5. Enable managed Postgres backups and continuous recovery logs before shared production. Encrypt/isolate backups.
6. Propose initial objectives for approval: RPO ≤ 5 minutes, RTO ≤ 60 minutes. Treat them as targets until product/operations formally agree.
7. Perform scheduled isolated restore drills and ledger-to-projection reconciliation. Redis loss is recovered from PostgreSQL/outbox, never from Redis money state.

**Exit evidence**: restore evidence meets approved objectives with no unexplained reconciliation difference; dependency exercises follow documented runbooks.

### Phase 8 — Quality, capacity and release gates

**What it means**: prove the product handles failures, not only a happy-path demo.

1. Unit test money, ledger, state, authorization, idempotency and RYEW validation.
2. Integration test migrations, constraints, locks, rollback, outbox, and cache ordering against isolated Postgres/Redis containers.
3. Contract test BFF/API/client agreement; E2E test authorized transfer journey and accessibility.
4. Add property/fuzz tests for money/invariants and concurrency race tests.
5. Fault test delayed/replayed events, relay termination, cache loss, database outage, lost post-commit response and stale cache.
6. Load test agreed capacity; tune only after query plan/pool/worker/cache measurement.
7. CI requires formatting, static analysis, tests, secret/SCA/container/IaC scans, SBOM, provenance/signing and migration compatibility.
8. Production release requires review, required checks, reconciliation, RYEW fault, cross-account authorization and restore-drill evidence.

**Exit evidence**: every feature requirement has automated evidence; artifacts are traceable; performance target is met without bypassing a required gate.

### Phase 9 — Controlled rollout and justified scaling

1. Roll out to internal users then a controlled cohort; observe reconciliation, fallback rate, outbox age and errors.
2. Feature flags may control non-financial rollout behavior only; they never disable ledger integrity, authorization, idempotency or primary fallback.
3. Application rollback remains migration compatible; financial correction uses compensating entries, not rewritten history.
4. Add a read replica only for measured read pressure; RYEW still falls back to primary.
5. Add NATS/RabbitMQ/Kafka only for proven independent durable consumers, retention or throughput needs.
6. Treat multi-region writes, FX, holds, scheduled transfers, payment rails and Kubernetes as independently specified future programs.

## Non-negotiable rules

1. No floating-point money.
2. No editing/deleting posted ledger history.
3. Same idempotency key plus same request never creates a second movement.
4. Transfer, postings, versions, response and outbox commit together or not at all.
5. Events are at-least-once; business effects are idempotent. Never claim distributed exactly once.
6. Redis/replicas are never financial authority.
7. Server-side object authorization is mandatory.
8. Raw consistency requirements are never logged, shown, or stored in browser local storage.
9. A lost response is resolved by idempotent replay.
10. No shared production release without reconciliation, RYEW fault, authorization and restore proof.

## High-value add-ons

**Add now**: Goose/Atlas/golang-migrate; `pgx` + `sqlc`; Testcontainers; OpenTelemetry Collector + Prometheus/Grafana/Tempo; Playwright + axe; Gitleaks, Trivy, Syft, `govulncheck`; Toxiproxy; managed OIDC, Postgres PITR and secret manager before shared production.

**Add when measured**: WAF before public launch; read replica for real load; NATS/RabbitMQ/Kafka for durable fan-out; SPIFFE/SPIRE/mTLS for a multi-service estate; Sentry only after telemetry redaction is proven.

**Do not add now**: Kubernetes, service mesh, active-active multi-region writes, custom auth, event sourcing as the only financial truth, or foreign exchange.

## Dependency order

Phase 0 enables all work. Phase 1 enables correct writes, events and recovery. Phase 2 is required before any user-facing transfer. Phase 3 must pass before Phase 4 cache/RYEW work. Phases 6–8 are mandatory release gates before Phase 9 shared production rollout.

## Design artifacts

- [Research and decisions](research.md)
- [Data model](data-model.md)
- [Browser/API and event contract](contracts/http-api.md)
- [Validation quickstart](quickstart.md)

## Next action

Run `$speckit-tasks` to produce dependency-ordered, file-level implementation tasks. Do not begin transfer implementation until the Phase 0–2 exit evidence is complete.
