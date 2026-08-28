# Local-product Phase 1 — account domain and data assurance

**Result:** `PASSED`

**Gate:** [LPC-010](../pilot/local-product-completion-gates.md)

**Starting point:** Phase 0 remediation commit `d674eaf`, with migrations `000001`–`000012` frozen

This is the independent verification record for the additive account-command implementation. Primary implementation review, independent security/API integration, the primary integration rerun, normal-stack migration, protected backup, reconciliation, and security smoke all passed. The verified working tree is based on `0a730a9`; the resulting integration commit will be assigned after the coordinated Phase 1 changes are committed.

## Implementation boundary reviewed

The review covers the emerging account aggregate and command service, shared command idempotency, additive migration `000013`, account command repository, existing transfer locking, generic audit records, the transfer-coupled outbox being generalized for account aggregates, Redis publication/projection consumers, provisioning compatibility, and database roles.

Phase 1 is data/domain work only. Public account mutation routes, BFF/CSRF behavior, and operator UI remain Phase 2 and Phase 3 gates.

## Verified risk checklist

### Concurrency and atomicity

- [x] One create transaction atomically commits exactly one `accounts` row, one debit owner, one credit permission, one zero balance projection, one zero opening baseline, one completed idempotency outcome, one success audit event, and one account outbox event.
- [x] Failure of any owner, projection, opening-baseline, audit, outbox, or idempotency-finalization insert rolls the entire create back; no partially visible account or orphan evidence remains.
- [x] Concurrent identical requests using the same actor/operation/key return one stable account ID and one persisted result, with no duplicate account, owner, audit, or outbox rows.
- [x] Concurrent changed intent using the same actor/operation/key returns an idempotency conflict and cannot change the original account or evidence.
- [x] Concurrent different keys using the same normalized tenant/external reference produce one deterministic success and one safe reference conflict, not duplicate open accounts or a raw uniqueness error.
- [x] Metadata and status commands use optimistic account-version comparison so concurrent writers cannot both succeed; a stale expected version has no side effects except the required bounded denial audit.
- [x] Freeze/close and transfer posting share compatible account/projection row locks. In both forced interleavings, either the lifecycle transition commits before the transfer is rejected or the transfer commits before closure is rejected; they cannot both commit an invalid outcome and must not deadlock beyond bounded serialization retry.
- [x] Closing an account while another transaction changes its projection cannot pass a stale zero check. The locked opening baseline plus immutable postings must agree with the locked projection in the same transaction.

### Migration compatibility

- [x] Frozen migrations `000001`–`000012` retain their Phase 0 SHA-256 values; the first schema change is additive `000013`.
- [x] `000013` applies from a clean database and a populated `000012` database without changing existing tenant/account IDs, currency, status, balances, balance versions, transfers, postings, idempotency results, audit rows, or outbox payloads.
- [x] Migration testing covers legacy metadata that the old schema allowed: null/blank names and references, mixed case, invalid characters, overlength values, and duplicate open references. The migration transforms these deterministically without silently merging accounts.
- [x] Existing transfer outbox rows are backfilled as `account_balance` with their original transfer/account relationship and remain claimable, publishable, replayable, and deduplicated.
- [x] Nullable `outbox_events.transfer_id` is accepted only for the new account aggregate shape. Existing balance-event constraints remain strict, and no fake transfer is manufactured for an account lifecycle event.
- [x] Existing worker, replay, retention, backup/restore, reconciliation, provisioning, and read repositories continue to operate after `000013`.
- [x] Database-role evidence grants the API only the new account-command operations it requires and preserves append-only/immutable protections. Migration, API, worker, reconciliation, provisioning, and support role tests pass against the expanded schema.

### Authorization and no disclosure

- [x] Create rejects a missing tenant or actor outside the tenant without creating an idempotency reservation, account, ownership, audit target, or outbox event that discloses tenant state.
- [x] The owner and tenant stored for a created account come from the trusted command envelope; metadata cannot nominate another subject, tenant, currency, starting balance, status, version, or account ID.
- [x] Update/freeze/reactivate/close requires a tenant-scoped debit owner at the repository boundary. Missing, cross-tenant, and unauthorized accounts return the same non-disclosing result.
- [x] Creator authority is bound to the server-controlled tenant `operator`/`finance` role policy; merely owning an account with read permission does not grant account-creation authority.
- [x] Authorization is checked before idempotency replay disclosure, so a subject whose access was removed cannot retrieve the stored account result solely by retaining a key.

Phase 2 must additionally enforce `accounts:write` at the HTTP/BFF boundary. That future route control is intentionally not claimed as Phase 1 evidence.

### Idempotency fingerprint and result stability

- [x] Fingerprints use length-delimited canonical fields and are namespaced by tenant, actor, and operation.
- [x] Create fingerprints include normalized display name, normalized external reference, normalized category, and currency. Metadata/status fingerprints additionally include account ID, action kind, expected version, and every requested value.
- [x] Idempotency key, correlation ID, request time, generated account ID, and transport formatting are excluded from semantic fingerprints.
- [x] Equivalent normalized input replays; any semantic field change conflicts. Metadata and status commands cannot collide even though they share the `accounts.update.v1` operation namespace.
- [x] A replay returns the exact persisted first result and original timestamps even after later account changes; it does not synthesize current state or append another audit/outbox event.
- [x] Completed account-command outcomes remain protected by the existing immutable idempotency trigger and are not deleted merely because their retry expiry passes.
- [x] Account version and every unsafe counter are stored and serialized as canonical decimal strings before any Phase 2 JavaScript boundary.

### Audit and outbox evidence

- [x] Success audit, account mutation, outbox row, and idempotency outcome commit in the same PostgreSQL transaction. An audit or outbox persistence failure prevents the business mutation from committing.
- [x] Exact replay produces no new success audit or outbox row. Safe denials record a bounded sanitized reason without money, credentials, raw payload, or untrusted free-form metadata.
- [x] Account events use an explicit account aggregate ID/version and an empty transfer relationship. Existing balance events retain a non-empty transfer relationship and their original account/balance aggregate version.
- [x] Account-created, metadata-updated, and status-changed events are unique per tenant/aggregate/event/version, contain exact string versions and timestamps, and cannot collide with balance-change events.
- [x] The outbox repository can scan null transfer IDs, the Redis stream can encode/decode both old and new envelopes, and the balance projector acknowledges but never applies an account lifecycle event as a balance update.
- [x] At-least-once publish, retry, dead-letter inspection, and replay work for account events without requiring a transfer ID or corrupting the balance cache.
- [x] Role grants allow the API transaction to insert account audit/outbox evidence while worker and support permissions remain least privilege.

### Zero-balance and financial invariants

- [x] New accounts always start `active`, INR, account version `1`, balance version `0`, available minor `0`, ledger minor `0`, and opening ledger minor `0`.
- [x] Create accepts no opening amount and creates no transfer, journal, or posting. Funding is possible only through the existing exact balanced-transfer path.
- [x] Reconciliation includes zero-balance account state and reports `matched` with zero mismatches; omitting the opening baseline or projection produces a mismatch, never a pass.
- [x] Metadata, freeze, reactivate, close, replay, audit, and outbox operations never update `account_balance_projections` or create financial postings.
- [x] Close requires both available and ledger projections to be zero and the projection to equal opening baseline plus immutable postings. Non-zero, missing, overflowed, or inconsistent financial state fails closed.
- [x] Closed is terminal at both domain and database boundaries. A closed account cannot be reactivated, renamed, debited, credited, or have its currency/tenant/ID/history rewritten.
- [x] Existing account, transfer, journal, posting, projection, and opening-balance fingerprints are unchanged after migration and after create/lifecycle tests, except for the specifically created Phase 1 fixtures.

## Evidence reviewed

| Evidence | Result |
|---|---|
| Independent security/API suite using disposable PostgreSQL 16 and Redis 7.4 | `PASS` |
| Primary `go test ./tests/integration -count=1` rerun | `PASS` |
| Go account/idempotency/domain/outbox unit tests, existing internal and contract suites, and `go vet` | `PASS` |
| Metadata and fingerprint fuzzing | `PASS` across millions of generated inputs |
| Populated `000012` → `000013` migration with dirty, invalid, and duplicate legacy metadata | `PASS`; financial identity/state preserved and references deterministically normalized |
| Existing-role migration and API-role account creation | `PASS` |
| Live close-versus-transfer concurrency race | `PASS`; exactly one valid outcome committed in each observed interleaving |
| Stable denial original/replay/changed-intent behavior | `PASS`; exactly one durable denial audit and idempotency outcome |
| Injected unknown dependency failure, rollback, and same-key retry after recovery | `PASS` |
| Normal protected backup | `backup-20260824T183310Z-0a730a9` — `PASS`; 6 accounts, 140,590 transfers, 281,178 postings |
| Normal supported stack | Healthy on schema `000013` with 13 migrations applied |
| Explicit reconciliation | Run `d2340016-b310-43d4-963a-012aee5e4f6b` — `matched`, 6 accounts checked, 0 mismatches |
| Outbox after drain | 0 pending, 0 dead |
| Local security smoke | `PASS` |

## Known verification limitation

The Go race detector could not execute in the current Windows environment because CGO is disabled. This does not fail LPC-010: the database-backed close-versus-transfer concurrency test passed against live PostgreSQL, the primary integration suite passed, and the implementation uses PostgreSQL serialization plus row locking as the authoritative concurrency boundary. A CGO-enabled Linux race run remains required in the later CI/convergence gate and must not be represented as completed here.

## Phase decision

The reviewed evidence closes the Phase 1 account domain/data risks without changing the frozen financial model. `LPC-010` is `PASSED`. Public mutation-route authorization, BFF/CSRF behavior, and browser contracts remain unimplemented claims governed by `LPC-020`, which is now `READY`.
