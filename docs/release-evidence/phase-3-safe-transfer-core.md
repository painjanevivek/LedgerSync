# Phase 3 — Safe transfer core evidence

**Status:** Code complete; live PostgreSQL evidence pending a configured isolated integration database.

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

## Pending live-database evidence

- `tests/integration/idempotency_test.go`, `concurrent_transfers_test.go`, and `reconciliation_test.go` require `LEDGERSYNC_TEST_DATABASE_URL` and apply the versioned migrations before seeding an isolated fixture.
- They were discovered and compiled by `go test ./...`, but skipped because `LEDGERSYNC_TEST_DATABASE_URL` is absent.
- Docker Desktop is installed but its service is disabled, so this workstation cannot currently start the isolated PostgreSQL compose profile.
- Do not treat Phase 3 as a production release gate until these tests run against an isolated PostgreSQL instance and their output is attached here.

## Required next validation command

```powershell
$env:LEDGERSYNC_TEST_DATABASE_URL = 'postgres://ledgersync:development-only-change-me@127.0.0.1:5432/ledgersync?sslmode=disable'
$env:GOCACHE = 'D:\Work\Project\Dev\LedgerSync\.cache\go-build'
& 'C:\Program Files\Go\bin\go.exe' test ./tests/integration -run 'Test(Transfer|Competing|Posted)' -count=1 -v
```
