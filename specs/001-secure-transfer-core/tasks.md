# Tasks: Secure Transfer Core

**Input**: Design documents in `specs/001-secure-transfer-core/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/http-api.md`, `quickstart.md`  
**Tests**: Required by FR-030/FR-031 and must be written before their implementation tasks.

## Phase 1: Setup — runnable, governed baseline

- [X] T001 Create the sole production root Go module using the repository module path and pin Go tools in `go.mod`, `go.work`, and `.tool-versions`
- [X] T002 [P] Create the target application directories, including `web/`, in `cmd/`, `internal/`, `migrations/`, `contracts/`, `deploy/`, `docs/`, and `tests/`
- [X] T003 [P] Create local configuration examples and ignore rules in `.env.example` and `.gitignore`
- [X] T004 [P] Replace the placeholder constitution with financial-correctness governance in `.specify/memory/constitution.md`
- [X] T005 [P] Record money, ledger, outbox, RYEW, API-boundary, and identity decisions in `docs/adr/`
- [X] T006 Create reproducible non-root API, worker, and web Dockerfiles plus committed web package lockfile in `deploy/docker/api.Dockerfile`, `deploy/docker/outbox-worker.Dockerfile`, `deploy/docker/web.Dockerfile`, `web/package.json`, and `web/package-lock.json`
- [X] T007 Create a production-safe local Compose topology that explicitly labels/excludes the legacy demo and gates diagnostics behind a non-default profile in `deploy/compose/docker-compose.yml`
- [X] T008 Add task-runner commands for format, lint, test, build, run, migrate, and reconcile in `Makefile`
- [X] T009 Implement configuration loading and startup validation in `internal/platform/config/config.go`
- [X] T010 Implement liveness/readiness endpoints and API bootstrap in `cmd/api/main.go` and `internal/transport/http/health.go`
- [X] T011 Implement graceful worker bootstrap/shutdown in `cmd/outbox-worker/main.go`
- [X] T012 Add baseline Go, web, and container CI scoped only to new production paths in `.github/workflows/ci.yml`

**Checkpoint**: a clean checkout builds API/worker images, starts private dependencies, and exposes health checks.

## Phase 2: Foundational — shared financial and security primitives

- [X] T013 Create migration runner configuration in `internal/platform/db/migrate.go` and `migrations/000001_financial_schema.up.sql`
- [X] T014 [P] Define exact money parsing, formatting, currency precision, and unit tests in `internal/domain/money/money.go` and `tests/unit/money_test.go`
- [X] T015 [P] Define account status, ownership permissions, and tests in `internal/domain/account/account.go` and `tests/unit/account_test.go`
- [X] T016 [P] Define transfer, ledger posting, and balancing invariants in `internal/domain/transfer/transfer.go`, `internal/domain/ledger/posting.go`, and `tests/unit/ledger_test.go`
- [X] T017 Add accounts, owners, transfer, journal, posting, projection, idempotency, outbox, and audit migrations in `migrations/000002_transfer_ledger.up.sql`
- [X] T018 Add immutable-ledger permissions and transaction-balance constraint migration in `migrations/000003_ledger_integrity.up.sql`
- [X] T019 Create typed PostgreSQL query definitions and generated query workflow in `internal/platform/db/queries.sql` and `sqlc.yaml`
- [X] T020 Implement database connection pool, transaction helper, and deadlock classification in `internal/platform/db/postgres.go`
- [X] T021 Implement validated public error envelope and correlation-ID middleware in `internal/transport/http/errors.go` and `internal/transport/http/middleware/request.go`
- [X] T022 Implement structured redacted logging and redaction tests in `internal/platform/observability/logging.go` and `tests/unit/logging_test.go`
- [X] T023 Implement OIDC principal interface and development identity adapter in `internal/platform/identity/identity.go` and `internal/platform/identity/development.go`
- [X] T024 Create BFF session, CSRF, security-header, request-limit, and rate-limit foundations in `web/src/lib/session.ts`, `web/src/lib/security.ts`, and `web/src/middleware.ts`
- [X] T025 Add versioned OpenAPI source and schema validation workflow in `contracts/openapi.yaml` and `.github/workflows/contract.yml`
- [X] T026 Add shared Postgres/Redis integration harness and fixtures in `tests/integration/harness_test.go` and `tests/integration/fixtures/`

**Checkpoint**: exact money, schema migration, identity abstraction, safe errors/logging, BFF session foundations, and integration harness are ready. No user story starts before this checkpoint.

## Phase 3: User Story 1 — Complete a safe money transfer (Priority: P1) 🎯 MVP

**Goal**: an authorized account holder transfers exact money once, receives a transfer ID, and can never overdraw through races or retries.

**Independent Test**: submit a transfer, repeat it with the same idempotency key, run competing debits, and reconcile the affected accounts.

- [X] T027 [P] [US1] Write transfer-contract success/validation tests in `tests/contract/transfers_contract_test.go`
- [X] T028 [P] [US1] Write duplicate-key, mismatch-key, and lost-response integration tests in `tests/integration/idempotency_test.go`
- [X] T029 [P] [US1] Write concurrent-debit/no-overdraft integration tests in `tests/integration/concurrent_transfers_test.go`
- [X] T030 [P] [US1] Write ledger/projection reconciliation tests in `tests/integration/reconciliation_test.go`
- [X] T031 [US1] Implement account ownership lookup and debit authorization in `internal/application/transfers/authorize.go`
- [X] T032 [US1] Implement idempotency reservation, fingerprint comparison, and response replay in `internal/application/transfers/idempotency.go`
- [X] T033 [US1] Implement deterministic account-projection locking and funds checks in `internal/platform/db/transfer_repository.go`
- [X] T034 [US1] Implement atomic journal, posting, balance-version, transfer-outcome, and outbox persistence in `internal/application/transfers/service.go`
- [X] T035 [US1] Implement the `POST /api/transfers` private API handler in `internal/transport/http/handlers/transfers.go`
- [X] T036 [US1] Implement the BFF transfer route and idempotency-header forwarding in `web/src/app/api/transfers/route.ts`
- [X] T037 [US1] Implement transfer request/result client types in `web/src/lib/api/transfers.ts`
- [X] T038 [US1] Add safe transfer audit records and metrics in `internal/application/transfers/audit.go` and `internal/platform/observability/transfers.go`
- [X] T039 [US1] Add transaction compatibility and retry tests for deadlock/serialization cases in `tests/unit/transaction_retry_test.go`

**Checkpoint**: US1 is demoable independently: no duplicate movement, no overdraft, and every posted transfer reconciles.

## Phase 4: User Story 2 — See the balance produced by a completed transfer (Priority: P1)

**Goal**: the transfer initiator never receives a balance version older than their completed transfer.

**Independent Test**: delay projection, complete a transfer, read immediately, then repeat with duplicate events, stopped worker, and Redis loss.

- [X] T040 [P] [US2] Write RYEW delay/primary-fallback fault tests in `tests/fault/ryew_delay_test.go`
- [X] T041 [P] [US2] Write duplicate-event/version-order tests in `tests/integration/cache_projection_test.go`
- [X] T042 [P] [US2] Write Redis-loss cache-rebuild tests in `tests/fault/cache_recovery_test.go`
- [X] T043 [US2] Implement outbox claiming, publish acknowledgement, retry backoff, and dead-event metrics in `internal/application/outbox/worker.go`
- [X] T044 [US2] Implement Redis Streams publishing and consumer-group recovery in `internal/platform/events/redis_streams.go`
- [X] T045 [US2] Implement version-aware cache projection and expiry in `internal/platform/cache/balance_cache.go`
- [X] T046 [US2] Implement signed consistency-requirement issue/verify/rotation interface in `internal/application/consistency/token.go`
- [X] T047 [US2] Implement balance read use case with cache-version check, bounded wait, and primary fallback in `internal/application/accounts/balance.go`
- [X] T048 [US2] Implement authorized balance API and BFF session requirement storage in `internal/transport/http/handlers/balances.go` and `web/src/app/api/accounts/[accountId]/balance/route.ts`
- [X] T049 [US2] Add outbox/cache/RYEW metrics and replay/rebuild command in `internal/platform/observability/ryew.go` and `cmd/reconcile/main.go`

**Checkpoint**: US2 fault tests prove a requirement-bearing read returns the required version or a truthful availability error, never stale data as current.

## Phase 5: User Story 3 — Protect accounts and administration (Priority: P2)

**Goal**: users access only their accounts; operational controls are privileged, safe, and auditable.

**Independent Test**: modify account IDs, call admin routes as a normal user, and inspect logs/telemetry for sensitive-data leakage.

- [X] T050 [P] [US3] Write object-authorization negative tests in `tests/integration/account_authorization_test.go`
- [X] T051 [P] [US3] Write BFF CSRF, session, and security-header tests in `web/tests/security/session.test.ts`
- [X] T052 [P] [US3] Write administrative-route and log-redaction tests in `tests/integration/admin_authorization_test.go` and `tests/unit/redaction_test.go`
- [X] T053 [US3] Implement OIDC token validation and role/scope mapping in `internal/platform/identity/oidc.go`
- [X] T054 [US3] Implement account read/history authorization use cases in `internal/application/accounts/service.go` and `internal/application/transactions/history.go`
- [X] T055 [US3] Implement owned-account and history handlers in `internal/transport/http/handlers/accounts.go` and `internal/transport/http/handlers/transactions.go`
- [X] T056 [US3] Implement BFF account/history routes and deny-by-default admin routing in `web/src/app/api/me/accounts/route.ts`, `web/src/app/api/accounts/[accountId]/transactions/route.ts`, and `web/src/app/admin/`
- [X] T057 [US3] Implement audit-event persistence and sensitive-action audit policy in `internal/platform/db/audit_repository.go` and `docs/runbooks/audit-events.md`
- [X] T058 [US3] Move local secrets to configuration and document managed-secret/rotation requirements in `.env.example` and `docs/runbooks/secrets-rotation.md`
- [X] T059 [US3] Restrict Compose networking, container privileges, and diagnostic profile exposure in `deploy/compose/docker-compose.yml` and `deploy/docker/`

**Checkpoint**: US3 denies cross-account and unauthorized admin access without disclosure, while operators retain sanitized audit evidence.

## Phase 6: User Story 4 — Recover and operate with confidence (Priority: P2)

**Goal**: operators detect degradation, recover safely, and prove financial records remain correct.

**Independent Test**: stop Redis/worker, age an outbox event, simulate a restore, and verify dashboards/runbooks/reconciliation evidence.

- [X] T060 [P] [US4] Write dependency-loss, outbox-recovery, and reconciliation-alert fault tests in `tests/fault/dependency_recovery_test.go`
- [X] T061 [P] [US4] Write migration-forward/backward compatibility tests in `tests/integration/migration_compatibility_test.go`
- [X] T062 [US4] Implement OpenTelemetry traces/metrics for API, worker, DB, cache, and event boundaries in `internal/platform/observability/telemetry.go`
- [X] T063 [US4] Define Prometheus alerts and Grafana dashboards in `deploy/observability/alerts.yml` and `deploy/observability/dashboards/`
- [X] T064 [US4] Implement reconciliation command/result persistence in `cmd/reconcile/main.go` and `internal/application/reconciliation/service.go`
- [X] T065 [US4] Create encrypted-backup/PITR configuration and isolated restore procedure in `deploy/backup/` and `docs/runbooks/restore.md`
- [X] T066 [US4] Create database, Redis, outbox, consistency, idempotency, secret-compromise, and restore runbooks in `docs/runbooks/`
- [X] T067 [US4] Add backup-age, restore-result, outbox-age, and zero-RYEW-violation release checks in `.github/workflows/release-evidence.yml`

**Checkpoint**: US4 proves dependency behavior, runbook-guided recovery, reconciliation, and restore evidence before shared deployment.

## Phase 7: Polish and cross-cutting launch readiness

- [X] T068 [P] Build dashboard sign-in, owned-account, balance, transaction-history, and sign-out screens in `web/src/app/` and `web/src/features/accounts/`
- [X] T069 [P] Build accessible transfer form, exact-money input, stable idempotency-key lifecycle, confirmation, and error states in `web/src/features/transfers/`
- [X] T070 Implement plain-language RYEW refresh and temporary-unavailability UX in `web/src/features/accounts/BalanceStatus.tsx`
- [X] T071 [P] Add Playwright authorized transfer/retry/error journey tests in `web/tests/e2e/transfer.spec.ts`
- [X] T072 [P] Add axe accessibility checks in `web/tests/e2e/accessibility.spec.ts`
- [X] T073 Add Go formatting, vet, lint, race, fuzz/property, Testcontainers, and coverage gates in `.github/workflows/quality.yml`
- [X] T074 [P] Add secret, dependency, container, IaC, and SBOM/provenance gates in `.github/workflows/security.yml`
- [X] T075 [P] Add reproducible fault-injection profile and Toxiproxy scenarios in `deploy/compose/docker-compose.fault.yml` and `tests/fault/`
- [X] T076 Add load-test scenarios, initial SLO targets, and capacity report template in `tests/performance/` and `docs/performance-baseline.md`
- [X] T077 Add public architecture, API, operations, and developer onboarding documentation in `README.md` and `docs/`
- [X] T078 Run every scenario in `quickstart.md` and attach release evidence in `docs/release-evidence/secure-transfer-core.md`

## Dependencies and execution order

- **Phase 1** has no dependencies; T002–T005 can run in parallel, then T006–T012.
- **Phase 2** depends on Phase 1; T014–T016 and T021–T024 can run in parallel once the relevant directories exist. T017/T018 depend on T013.
- **US1** depends on all Phase 2 tasks. T027–T030 are parallel test-first tasks; T031–T038 then proceed in the listed order.
- **US2** depends on US1 because it requires a real transfer version and outbox row. T040–T042 are parallel before T043–T049.
- **US3** requires Phase 2; it can be developed in parallel with US2 after US1's transfer contract and identity foundations are stable.
- **US4** requires Phases 1–4 because it measures/recover their behavior; T060/T061 can start before observability implementation.
- **Phase 7** dashboard work depends on US1–US3 contracts; final evidence T078 depends on all desired stories.

## Parallel subagent assignments

Only start tasks whose prerequisites are complete. Avoid concurrent edits to the same files.

| Subagent role | Assigned work | Start condition | Deliverable |
|---|---|---|---|
| Foundation engineer | T001–T012 | Immediate | Runnable root skeleton, Compose, configuration, health, CI. |
| Financial-core engineer | T013–T039 | After Phase 1 | Migrations, exact money, ledger, authorization, idempotent transfer and proof tests. |
| Consistency/reliability engineer | T040–T049, T060–T067 | US1 completed; US4 after US2 | Outbox, versioned cache, RYEW, telemetry, recovery and runbooks. |
| Web/security engineer | T050–T059, T068–T072 | Phase 2; UI after contracts stabilize | OIDC/BFF, authorization, admin isolation, accessible dashboard. |
| Release-quality engineer | T073–T078 | Core contracts and tests exist | Supply-chain gates, fault/load tests, documentation and release evidence. |

## MVP strategy

The smallest credible MVP is **Phase 1 + Phase 2 + User Story 1**. Stop and prove exact-money, authorization, idempotency, concurrent-funds protection, ledger balancing, and reconciliation before adding caching or dashboard polish. User Story 2 is the next required differentiator; US3/US4 are shared-production release gates.

All tasks use the required checkbox, sequential ID, optional parallel marker, user-story label, and exact-path format.
