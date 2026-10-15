# Transaction, failure, and resource ownership

R01 design review, 14 October 2026. These walks check responsibility against existing source. They are not new end-to-end execution results.

## One transfer

1. The console keeps exact amount text and a stable idempotency key/request reference. The Next.js BFF derives authority from the session. The user cannot choose authority through an arbitrary header.
2. The Go HTTP handler validates the request and maps typed errors. `application/transfers` normalises financial intent and its fingerprint. It depends on a repository interface.
3. `platform/db/transfer_repository.go` uses `WithSerializableSequence` with five attempts. It calls the existing `controlled_submit_transfer_v1` and attaches the request reference inside that transaction. Keep this implementation; do not build a duplicate controlled transfer function in R03.
4. The database command must keep accepted intent, stored replay response, transfer, journal, balanced postings, affected projections/versions, required audit, and required outbox obligations in one commit. Verify the existing SQL implementation and all callers in R03. Tenant context and role hardening are incomplete today.
5. The handler returns a stored posted/rejected result only when authoritative evidence permits it. A lost response is unknown to the client. The client uses the same intent and key to resolve it. A delivery failure does not undo a committed transfer.

Policy-denial audit currently has a separate write after the failed transaction in `recordDeniedAudit`. This is not a posted financial result and must not be described as part of the accepted transaction. R03 must preserve denial evidence, control its writer, and test audit failure. No network delivery belongs inside the financial commit.

## One correction

1. A requester selects an existing posted transfer and gives a permitted reason. Corrections handler/application validate identity and intent.
2. `transfer_correction_repository.go` uses the existing serializable sequence. It checks requester authority, original transfer state, policy version, recent step-up where required, expiry, and idempotency.
3. A separate permitted approver decides when policy requires it. A repeated decision cannot create a second compensation. Concurrent decisions must resolve to one authorised outcome.
4. Compensation creates a new linked transfer and journal. It does not edit the original posting. Financial records, decision state, stored outcome, required audit, and event obligations commit together on the final financial path.
5. Expired, rejected, unauthorised, conflicting, and uncertain outcomes remain distinct. R03 moves current direct SQL into controlled commands only after matching tests exist. A UI route returning 200 does not verify this transaction.

## One balance/history read

1. Session and server identity establish tenant/actor. The account/history handler passes a bounded query through the application interface.
2. The repository applies object access and tenant predicates today. R02 adds fail-closed database tenant context on the same transaction/connection. Test joins, direct IDs, cursor changes, exports, and pooled-connection reuse.
3. PostgreSQL is authoritative. Redis observations can satisfy a documented version rule, but cannot invent current truth. Preserve minimum-version consistency requirements.
4. Success with zero authorised rows can be empty. Timeout/unavailable is not empty. A denied row must not leak through counts, cursors, exports, or diagnostic links. Read cancellation releases resources.

## One worker retry

1. Outbox rows exist only after commit. `application/outbox` claims a bounded batch through its Store interface.
2. The database repository owns lease expiry and claim generation. `application/lease` renews the claim. The event ID and aggregate version remain stable across attempts.
3. The publisher can succeed before the worker records acknowledgement. A crash in this interval causes a possible duplicate delivery. The receiver must deduplicate and apply versions monotonically.
4. Failed delivery becomes retry/dead state under the existing bound. A stale worker must not acknowledge or reschedule a newer claim. Lost lease is an explicit failure.
5. R03 controls operational writers. R07/R08 prove crash, duplicate, cancellation, and stale-worker cases in an isolated stack. No result is labelled exactly-once delivery.

## One evidence export

1. The authorised export handler calls `application/exports` with tenant/actor and a bounded filter. The SQL adapter checks the same object authority as the screen.
2. The exporter uses keyset pages and streams exact values through the existing quoted CSV writer. It checks cancellation between pages and propagates write/read errors.
3. A partial or truncated export must state its limit. A failed download is not a complete file. R02 tests isolation; R13 binds candidate exports to their schema/version and evidence time.
4. Recovery/demonstration evidence is historical. The new demo JSON validator validates structure only. R07 must verify referenced file hashes, target identity, and actual execution before accepting a run.

## Current resource-bound register

Values are source defaults, not a dump of live environment values. `source-inventory.json` records exact file/line declarations for follow-up. The named owner must recheck a value when its operation changes.

| Boundary | Source and current bound | Owner and failure behaviour |
|---|---|---|
| Go HTTP lifecycle | `internal/platform/config/config.go`: header 3s, read 10s, write 15s, idle 60s; `LEDGERSYNC_HTTP_*_TIMEOUT` | Platform; validate positive durations. Client uncertainty after a write timeout remains unknown. |
| Startup/shutdown | Same config: startup 90s, shutdown 10s; validated startup retry/backoff settings | Bootstrap; bounded startup retry and cancellation. |
| Database pool | Config `LEDGERSYNC_DATABASE_MAX_OPEN_CONNS`, default 4, maximum 64; `db/postgres.go` fallback idle 2 and lifetime 30m | Database; R09 budgets all process pools together. |
| Financial transaction retry | `db/postgres.go`; transfer/correction callers request 5 attempts | Database; only recognised serialization/deadlock failures retry, with bounded jitter and context cancellation. Never retry a different financial intent. |
| Go command bodies | Handler constants: accounts 4 KiB; transfer/funding/correction 64 KiB; developer credentials 16 KiB; reconciliation 1 KiB | API feature owner; reject oversize/malformed input before application calls. |
| Preferences/saved views | Handler constants: 4 KiB / 8 KiB | Investigation/guidance owner; bounded stored preferences. |
| BFF JSON input | `web/src/lib/security.ts`: default 16,384 bytes; callers can supply their contract limit | BFF owner; validate advertised and actual byte count. |
| BFF upstream read/write | `web/src/lib/upstream-outcome.ts`: 5,000 / 8,000 ms | BFF owner; distinguish unavailable read from unknown financial write. |
| Browser API calls | `web/src/lib/api/client.ts`: 8,000 ms | UI owner; retain safe retry identity for uncertain writes. |
| Rate-limit/credential fetch | `web/src/lib/rate-limit.ts`: 2,000 ms; `workload-credential.ts`: 5,000 ms | Security boundary; fail according to existing endpoint contract. |
| History page | Application transactions and HTTP transactions handler: 1–100 | Query owner; validate cursor and access on every page. |
| Evidence export | `application/exports/service.go`: default 10,000 rows, 250/page; configurable maximum 100,000 rows, 1,000/page | Export owner; streaming, cancellation, explicit truncation. |
| Outbox claim/retry | `application/outbox/worker.go`: default batch 10, maximum 100; lease default 30s; attempts default 12 | Worker; lease loss and retry/dead states are explicit. |
| Webhook claim/retry | Delivery/verification worker configuration: default attempts 8; repository claim limit 1–100 and lease up to 10m | Delivery owner; preserve fencing and safe replay. |
| Webhook HTTP | `application/webhookdelivery/dispatcher.go`: total 10s, TLS/header 5s | Delivery adapter; retain destination validation and SSRF protection. R04 owns extraction. |
| Optional study observations | `platform/routing/study_proxy.go`: HTTP client 1s; distributed evidence 750ms | Systems Lab; unavailable observation cannot change ledger truth. |
| Demo manifest | `scripts/portfolio/demo-contract.mjs`: input 1 MiB; schema bounds arrays/strings, scenario duration up to 3,600s | Evidence owner; reject invalid metadata. These are contract caps, not a claim that a runner enforces them yet. |

Rate-limit policy, feature-specific page sizes, telemetry queue bounds, and cancellation paths remain owned by their current configuration and callers. The lexical register is the baseline. Each changed R02/R03 operation requires executable limit tests; R06 verifies telemetry bounds and R09 measures combined resource use.
