# Local-product Phase 5 — diagnostics and event-delivery evidence

**Result:** `PASSED`

**Verified:** 2026-08-24T21:30:06Z

**Candidate:** Phase 5 working tree based on `2b95d8a`; the resulting Phase 5 commit binds this evidence to the implementation.

**Boundary:** the supported single-workstation Docker Compose runtime, one INR demo tenant, a server-controlled local operator, private API/PostgreSQL/Redis services, and browser access only at `http://127.0.0.1:3000`.

## Operator outcome

- `/local-status` reports overall readiness, PostgreSQL financial authority and schema, latest reconciliation, outbox state and database-derived worker progress, disposable Redis cache reachability, local environment, build facts, and safe next actions.
- `/events` provides tenant-authorized, filtered, deterministic keyset pagination over existing outbox evidence. Event detail shows a bounded, sanitized timeline and the newest 25 delivery attempts in chronological presentation order.
- Posted financial state is never derived from delivery state. Pending, retrying, dead, unknown, and published delivery evidence use separate language and visual severity.
- Related account and transfer links are returned and rendered only when the repository proves they belong to the authorized tenant.
- No Docker socket, logs, DSN, host path, container identifier, credential, token, endpoint URL, raw error, raw event payload, replay control, or mutation route crosses the operations API/BFF boundary.

## Security and correctness controls

- Private and BFF routes are GET-only and require the signed tenant session plus `local:read` or `events:read`.
- The BFF enforces the exact loopback host, a per-principal read rate limit, a five-second upstream timeout, `no-store`, strict query/DTO allowlists, semantic error-code redaction, and a streamed 256 KiB response ceiling.
- Repository authorization is repeated at the database query boundary with tenant and operator/finance role predicates and non-disclosing not-found behavior.
- Event cursors bind occurrence time, event UUID, every filter, and limit using a query fingerprint. Changing a filter invalidates the cursor.
- Browser request generations prevent a slower prior filter/detail request from overwriting newer operator context.
- Diagnostic dependency probes run concurrently, honor parent cancellation, stop at short per-dependency deadlines, and preserve already-completed partial results.
- The additive `000015_operations_read_models.up.sql` migration contains indexes only; it does not alter financial records or payloads.

## Automated evidence

| Layer | Result |
|---|---|
| Go unit, contract, fault, integration, and system suite | `go test ./... -count=1` passed |
| Go static checks | `go vet ./...` passed |
| Repeated operations application suite | 50 consecutive passes |
| Web unit/security | 60/60 passed |
| Full browser, accessibility, responsive, and visual suite | 85/85 passed with 16 workers |
| Focused Phase 5 browser states | 8/8 passed |
| Focused Phase 5 visual comparison | 3/3 passed |
| Type, lint, production build | TypeScript, ESLint, and Next.js build passed |
| Performance budget | 776,783 total JavaScript bytes; largest chunk 229,156 bytes, below 2,000,000 and 350,000-byte limits |
| Patch integrity | `git diff --check` passed |

## Disposable PostgreSQL proof

The root verification created a named disposable database inside the local PostgreSQL container, applied all migrations through `000015`, ran the three Phase 5 integration cases, then removed the test container and dropped only that validated database.

- `TestEventEvidenceAuthorizationPaginationAndFirstClaimTruth`: passed, including cursor/filter binding, correlation filtering, non-disclosing authorization, pending first claim, and retry after reschedule.
- `TestEventEvidenceRedactsHostileCodesAndNeverReturnsPayloadOrEndpoint`: passed.
- `TestDiagnosticFactsAreTenantScopedAndWorkerProgressIsDatabaseDerived`: passed.
- Result: 3/3 passed in 0.670 seconds.
- Cleanup: `PHASE5_TEST_DATABASE_CLEANUP=PASS`.

The first live attempt safely failed because the draft query referenced a nonexistent transfer correlation column. The database and container were still removed. The query was corrected to derive a transfer correlation only from the immutable `transfer.posted` audit event through a bounded lateral lookup, a correlation-filter regression assertion was added, and a fresh database passed all three tests. This failure was not hidden or converted into a conditional pass.

## Supported-stack and fault evidence

- Pre-migration backup: `data/local-backups/backup-20260824T212417Z-2b95d8a`.
- Backup source schema: `000014_operator_reconciliation_commands.up.sql`.
- Backup counts: 7 accounts, 140,590 transfers, 281,178 postings.
- Dump: 80,438,481 bytes; SHA-256 `6a36f81ba27268651024c114780939fc0d67a19ede6449209ebe4c0627e78ec4`.
- The supported start path rebuilt the candidate images, applied schema `000015_operations_read_models.up.sql`, and returned all long-running services healthy.
- Live BFF diagnostics returned HTTP 200, `overall=ready`, PostgreSQL `reachable`, schema `000015`, Redis `reachable`, and `Cache-Control: no-store`.
- Live BFF events returned HTTP 200 with bounded results, a stable next cursor, and `Cache-Control: no-store`.
- With Redis stopped, diagnostics remained HTTP 200 and truthfully returned `overall=degraded`, PostgreSQL `reachable`, Redis `unavailable`, and the `disposable_cache` label. PostgreSQL-backed account/balance smoke reads remained available.
- Redis flush/rebuild, worker restart, API/web restart, PostgreSQL unavailability, full dependency-order stop/start, and cache rebuild all passed.
- Final financial fingerprint was unchanged; outbox pending `0`, dead `0`; reconciliation matched with `0` mismatches.
- Final reconciliation/cache-rebuild run: `6c49a6b2-f813-4007-b5bb-14cbd7df17db`.

## Visual review

The three new operations baselines and the two overview baselines affected by Local tools navigation were inspected and approved. They retain the chosen navy/emerald document-console treatment, reserve green success meaning for proven healthy/published states, use amber for retrying/unknown delivery, and preserve a single semantic table with contained horizontal inspection at compact and 200%-equivalent widths.

## Local-only limitations

- Worker state is intentionally inferred from durable outbox progress and lease evidence; the API does not claim whether a Docker process exists. With no pending work it reports `idle`, not “worker running.”
- The BFF read limiter is process-local because this boundary has one web process. A distributed limiter remains outside the local-only product claim.
- Operations evidence is read-only. Existing two-person replay tools remain separate and are not exposed in the browser.

