# Phase 0D lifecycle, recovery, and real-runtime evidence

Phase 0D closes the operational gap between a correct ledger transaction and a platform that can be run safely when dependencies fail, records accumulate, or a design partner must be provisioned. This document records reproducible engineering evidence; it is not a substitute for the external pilot approvals listed below.

## Implemented controls

- Redis stream publication is approximately bounded by `LEDGERSYNC_REDIS_STREAM_MAX_LENGTH` (default 5,000,000 entries). Stream depth and consumer lag are measured and alert before the bound is approached.
- PostgreSQL cleanup is explicitly invoked, defaults to dry-run, processes at most 10,000 rows per batch, and writes an immutable retention run plus an audit event. Final idempotency records, ledger records, transfers, reconciliation evidence, dead/unresolved work, delivery evidence, audit evidence, and provisioning evidence are retained.
- Dead outbox and delivery work supports inspect, approve, and replay commands. Approval and execution are separately attributed; one person cannot both approve and execute. Identical approval retries and execution retries cannot duplicate work.
- Internal partner provisioning validates one pilot currency, exact string money, supported roles/scopes/categories, credential references, and expiration before applying all records atomically. Rollback closes accounts and appends credential revocation evidence; it does not delete financial or identity history.
- Account history loading is independent from balance loading, so an unavailable history cannot be rendered as a false empty history. Responsive, keyboard, forced-colors, zoom, reflow, and authored-color contrast checks run in Playwright.
- The system test crosses the actual Next.js BFF, Go API, PostgreSQL, Redis, and outbox worker. It posts and safely retries one transfer with the same idempotency key.

## Reproducible commands

Run these from the repository root after starting the fault-test dependencies described in `tests/fault/README.md`:

```powershell
$env:LEDGERSYNC_TEST_DATABASE_URL='postgres://ledgersync:fault-test-only@127.0.0.1:15432/ledgersync?sslmode=disable'
$env:LEDGERSYNC_TEST_REDIS_ADDR='127.0.0.1:16379'
go test -p 1 ./... -count=1
go vet ./...
Push-Location web
npm ci
npm run lint
npm test
npm run build
npm run test:performance
npm run test:e2e
Pop-Location
```

Run the real-runtime path against the isolated Compose project:

```powershell
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml up -d --build
$env:LEDGERSYNC_SYSTEM_WEB_URL='http://localhost:3000'
go test ./tests/system -run TestRealBFFAPIAndPostgreSQLRetryPath -count=1
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml down -v
```

Validate a provisioning file without writing data:

```powershell
go run ./cmd/provision-partner -action validate -config docs/pilot/provisioning-example.json -pilot-currency INR
```

CI generates `release-evidence-manifest.json` after all required jobs pass. The manifest is bound to the tested commit SHA and identifies external gates rather than reporting them as passed.

## Expected invariants

1. Retry of a completed transfer returns the original transfer and never posts another debit or credit.
2. Every posted transfer has balanced immutable postings and a durable transfer version.
3. Retention never deletes final idempotency, financial, audit, reconciliation, dead-work, delivery, or provisioning evidence.
4. Replay has durable approval and execution evidence, preserves original event identity, and cannot execute twice.
5. Provisioning never stores a client secret, bearer token, refresh token, or private key.
6. Redis loss affects freshness and delivery only; PostgreSQL remains the financial source of truth.

## External pilot gates (not yet passed by automation)

- The selected managed PostgreSQL provider must be configured for encrypted backups and PITR, followed by an isolated restore drill with retained provider evidence.
- Security/compliance owners must approve the final retention schedule and any jurisdiction-specific legal holds.
- Platform owners must provision real IdP/workload credentials from the selected managed secret store and complete the rotation drill.
- Finance/operations and product owners must approve financial UI terminology and real-device evidence before a pilot-readiness claim.
