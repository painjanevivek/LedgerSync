# Local-product Phase 4 — authoritative reconciliation evidence

**Result:** `PASSED`

**Verified:** 2026-08-24T20:56:56Z

**Candidate:** Phase 4 working tree based on `e487d35`; the resulting Phase 4 commit binds this evidence to the implementation.

**Boundary:** one local Windows workstation, the supported Docker Compose runtime, one INR tenant, a server-controlled demo operator, and a private PostgreSQL financial authority. No external deployment or managed identity claim is made.

## What passed

- One tenant can have only one active operator reconciliation command. The command repository obtains a PostgreSQL tenant advisory lock inside the authoritative repeatable-read transaction and keeps only one bounded active-command marker.
- A retry with the same idempotency key returns the original immutable run. A different key while the tenant lock is held returns `reconciliation_already_running` with the stable active run identifier.
- Request, completion, and denial audit evidence is durable. Run creation, mismatch rows, completion audit, idempotency completion, and active-command cleanup commit atomically.
- A matched result is emitted only after projected balances are compared with immutable postings. A deliberately changed projection produces a mismatch result and never all-clear wording.
- The BFF requires the signed operator session, `reconciliation:write`, fixed host and origin, CSRF, a strict empty JSON body, a bounded visible-ASCII idempotency key, a rate limit, a timeout, and `no-store`. Upstream fields are allowlisted before reaching the browser.
- The UI has explicit review, running, matched, mismatch, already-running, offline, unavailable, retained-unknown, and bounded-polling states. It preserves earlier run history and cannot mark a mismatch resolved.

## Automated evidence

| Layer | Result |
|---|---|
| Go unit, contract, and non-database integration suite | `go test ./... -count=1` passed |
| Go static checks | `go vet ./...` passed |
| PostgreSQL command integration | 3/3 passed in disposable database `ledgersync_phase4_test_20260825`: matched/mismatch/replay, stable already-running conflict, and forced-audit-failure rollback |
| Web unit and security | 54/54 passed |
| Mocked browser journeys | 74/74 passed |
| Windows visual comparison | 23/23 passed without snapshot update |
| Pinned Linux visual comparison | 19/19 passed; four Windows-only snapshot cases were intentionally skipped |
| Performance budget | 733,590 total JavaScript bytes; largest chunk 229,156 bytes, below the 2,000,000 and 350,000-byte limits |
| Type, lint, and production build | TypeScript, ESLint, and Next.js production build passed |
| Patch integrity | `git diff --check` passed |

## Real-stack financial proof

The isolated supported-stack journey created an account, replayed its create request, funded it through balanced double entry, denied a non-zero close, froze and reactivated it, returned it to exact zero, closed it, and then ran authoritative reconciliation through the browser/BFF.

- Isolated project: `ledgersync-acceptance-20260824205537-edd63d9b`
- Schema: `000014_operator_reconciliation_commands.up.sql`
- Account: `e566ef07-df96-4780-86ec-dc1d6cb017d6`
- Funding transfer: `d0e57082-3e88-4434-961f-469d1dc1e9ae`
- Return transfer: `790e8c2e-068d-44b2-8669-3b97a4b0e037`
- Reconciliation run: `b28cb2a5-ad35-414f-8ace-6817ade1501f`
- Browser result: matched, zero mismatches; exact-key replay returned HTTP 201 with `Idempotent-Replay: true` and the same run ID.
- Direct PostgreSQL proof: one matched run, one completed idempotency record, one request audit, one completion audit, and zero active reconciliation command rows.
- Final isolated controls: pending outbox `0`, dead outbox `0`, reconciliation `matched`, mismatches `0`.
- Cleanup: disposable containers, networks, and volumes removed; the normal `compose` project was restored and healthy.

## Active-lock and retry proof on the preserved normal stack

Holding the tenant advisory lock forced the first key to return `request_in_progress`. A different key returned `reconciliation_already_running` and active run `87360188-dc53-467e-808d-468c41bb3f99`. After releasing the lock, retrying the exact first key completed that same preallocated run. Durable inspection found one request audit, one completion audit, one denial audit, zero mismatches, and zero remaining active-command markers.

An independent normal BFF request also created matched run `bf9e73c4-2cec-4bac-b9f7-5b1b58687b6b`; its exact retry returned the same run with the replay header.

## Failure and rollback evidence

- A seeded one-minor-unit projection difference produced a durable mismatch run in PostgreSQL integration testing.
- A trigger-forced request-audit failure rolled back the run, idempotency state, and command reservation instead of exposing partial evidence.
- Lock-held request interruption retained a stable preallocated run and exact retry key; release plus same-key retry completed once.
- A committed request replay returned the already-created run, proving lost-response retry does not duplicate financial-control evidence.
- Commit ambiguity is mapped to `response_unknown`; the browser retains the exact retry key and does not infer failure or success.

## Reviewed limitations

- The local Compose runtime intentionally connects as its local database owner and does not provision the optional `ledgersync_api` least-privilege role. Migration grants are conditional and the role-contract test runs when those roles exist. This is a local-only runtime limitation, not a production-role claim.
- The browser/BFF rate limiter is process-local, appropriate for the single local web instance. Multi-instance shared throttling remains outside this local-product boundary.
- A physical PostgreSQL TCP disconnect at the exact commit acknowledgement boundary was not induced. The ambiguity mapping is unit-tested, while real-stack same-key replay after commit and interruption while the authoritative lock is held prove the two safe operator recovery paths.

