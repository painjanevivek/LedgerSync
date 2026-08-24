# Local MVP Phase 3 — transfer-safety evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows/Docker Desktop workstation

**Evidence binding:** the Git commit containing this document

## What was proved

The supported browser/BFF/API/PostgreSQL path preserves LedgerSync's exact-money, authorization, idempotency, atomic double-entry, immediate-balance, and reconciliation guarantees. Redis remained disposable: delayed, empty, or unavailable cache state could not make an older balance appear current.

## Exact-money boundary

- A scan of `cmd`, `internal`, and `web/src` found no floating-point parsing, formatting, or arithmetic in a financial path.
- The only Go `float64` uses record elapsed telemetry milliseconds. The only TypeScript rounding uses authentication expiry or rate-limit seconds.
- INR `92233720368547758.07` round-trips as the exact signed-64-bit maximum minor-unit string `9223372036854775807`.
- INR `92233720368547758.08`, excessive precision, exponent notation, internal whitespace, signs, negative values, unsupported currency, and malformed decimals fail closed.
- Outer boundary whitespace is normalized deliberately; zero can exist as a domain value but is rejected before the transfer repository.
- Browser formatting does not use `Number`, scientific notation, locale grouping, or rounding and refuses out-of-range response values.
- The exact-money fuzz target ran for 10 seconds, executed 83,783 cases after its seed corpus, and passed.

## Atomic financial result

One accepted live-database transfer was verified to commit all of the following together:

| Evidence | Required count/result | Observed |
|---|---:|---:|
| Posted transfer | 1 | 1 |
| Journal transaction | 1 | 1 |
| Debit posting | 1 | 1 |
| Credit posting | 1 | 1 |
| Source/destination balance version increments | 2 | 2 |
| Successful posting audit | 1 | 1 |
| Completed idempotency outcome | 1 | 1 |
| Balance outbox events | 2 | 2 |
| Velocity event | 1 | 1 |

An injected PostgreSQL trigger failed the credit posting after the transfer, journal, and debit insert steps had begun. The transaction returned an error; zero transfers, journals, postings, audits, idempotency rows, outbox events, or velocity events survived, and both balances and versions remained unchanged.

Final transfers, journals, postings, audit events, and completed idempotency evidence rejected update/delete attempts. Provisioned support and break-glass roles retained no forbidden standing write authority.

## Idempotency and uncertain outcomes

- Sequential same-key requests returned one transfer ID, one journal, and two postings.
- Twelve concurrent identical same-key requests produced exactly one original outcome and eleven replays of the same transfer ID.
- Changed amount, source, destination, or currency with the same key returned an idempotency conflict and did not alter financial state.
- A first result was intentionally ignored, the transfer service/repository was reconstructed, and the same key replayed the stored result without a second movement.
- Through the real local BFF, a fresh request posted once; after the API container was restarted, two same-key attempts both returned `Idempotent-Replay: true`, the same transfer ID, and no balance change.
- The Phase 2 browser test continues to prove that an unknown response retains the exact original payload and key and exposes only **Retry same transfer**.

## Authorization, concurrency, and capacity

- Wrong tenant, actor, source account, destination permission, and missing `transfers:write` scope were denied before financial mutation or object disclosure.
- The wrong-tenant test exposed and fixed a boundary defect: a fabricated tenant previously surfaced an internal foreign-key failure. The repository now verifies the tenant boundary inside the transaction and returns the same non-disclosing account denial.
- Competing debits never crossed the account floor and produced one valid balanced movement when funds allowed only one.
- Concurrent transfers could not bypass rolling actor limits.
- Fifty hot-tenant submissions completed without an exhausted serialization conflict; every accepted movement retained two postings and one velocity event.
- A serialization exhaustion is mapped to a truthful retryable outcome, never a successful post.

## Read-your-writes, cache faults, and reconciliation

- A required balance version waits briefly for the cache, then falls back to PostgreSQL if the projection is delayed.
- Redis loss used PostgreSQL primary and rebuilt disposable cache state without changing ledger records.
- Worker lease expiry was recovered; Redis publication loss rescheduled delivery without altering financial postings.
- Duplicate projection delivery never regressed a balance version.
- A deliberately missing/stale projection cannot produce a false reconciliation pass.
- After the real API-restart replay journey, authoritative reconciliation run `27d7e3e2-a345-4064-8a38-5d04af76d0c6` completed as `matched` across 6 accounts with exactly 0 mismatches. The local outbox then reported 0 pending and 0 dead events.

## Reproducible checks

| Check | Result |
|---|---|
| Go formatting and deterministic full repository suite | Passed |
| Go unit and contract suites | Passed |
| Exact-money fuzz target, 10 seconds | Passed |
| Disposable PostgreSQL integration suite, run serially | Passed |
| Disposable PostgreSQL/Redis fault suite, run after integration | Passed |
| Web lint and 23 unit tests | Passed |
| Real BFF first request and sequential replay | Passed |
| Actual API-container restart and post-restart same-key replay | Passed |
| Fresh authoritative reconciliation and local health | Passed; 0 mismatches, 0 pending/dead outbox |
| Linux race, quality, live-dependency, browser, and real-stack jobs | Enforced by the pushed commit's `Quality gates` workflow |

The live dependency packages must run serially because each intentionally owns and truncates its disposable database. A deliberately parallel local invocation was rejected as invalid harness use after fixture collisions; rerunning in the repository's CI-defined serial order passed both suites.

The Windows host has `CGO_ENABLED=0`, so the Go race detector is not locally available. The immutable pushed commit remains blocked on the Linux race job rather than treating this as a local waiver.

## Boundary

This proves internal, same-currency INR ledger movement for the loopback-only local demo. It does not claim bank settlement, custody, card rails, FX, managed identity, or production deployment.
