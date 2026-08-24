# Phase 3 — Safe transfer core evidence

**Status:** Validated against an isolated PostgreSQL 16 instance on 2026-08-18.

## Delivered

- Exact-money transfer command with mandatory idempotency key and request fingerprint.
- PostgreSQL serializable transaction that reserves/replays the idempotency outcome, locks source/destination projections in a stable ID order, prevents overdraft, creates a balanced debit/credit journal, increments both balance versions, writes audit evidence, and enqueues one balance event per account.
- Rejected insufficient-funds outcome that creates no postings, balance movement, or outbox event.
- Private transfer HTTP handler with strict JSON/body limits, bearer authentication, safe public errors, correlation ID forwarding, and idempotent replay header.
- Same-origin BFF route that validates session/CSRF, preserves the idempotency key, uses exact minor-unit string handling, and never invents an outcome when the private API is unavailable.
- Contract, idempotency, concurrency/no-overdraft, reconciliation, and transaction-retry classification tests.

## Commands executed successfully

- `go test ./...` using a workspace-local `GOCACHE`.
- `go vet ./...` using a workspace-local `GOCACHE`.
- `npm run lint` in `web/`.
- `npm run build` in `web/`.

## Live PostgreSQL validation

- A disposable PostgreSQL 16 container was started on `127.0.0.1:5432`; it was separate from application environments and used only for this evidence run.
- The integration harness applied the repository migrations, seeded the isolated tenant/account fixture, and exercised the actual PostgreSQL repository implementation. No test was skipped.
- Command executed:

```powershell
$env:LEDGERSYNC_TEST_DATABASE_URL = 'postgres://ledgersync:<generated-local-password>@127.0.0.1:5432/ledgersync?sslmode=disable'
$env:GOCACHE = 'D:\Work\Project\Dev\LedgerSync\.cache\go-build'
& 'C:\Program Files\Go\bin\go.exe' test ./tests/integration -run 'Test(Transfer|Competing|Posted)' -count=1 -v
```

- Result: `PASS` in `2.613s`.
- Proven invariants:
  - Competing transfers cannot overdraw the source account.
  - Retrying a completed request with the same idempotency key replays the original outcome without a second movement.
  - Reusing an idempotency key for a different financial intent is rejected.
  - Posted account-balance projections reconcile exactly to the immutable ledger.

The temporary container is removed after this evidence run; no application or customer database was modified.
