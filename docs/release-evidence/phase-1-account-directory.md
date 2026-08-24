# Phase 1 account-directory evidence

Phase 1 replaces the former full-dataset browser filter with an authorized, bounded PostgreSQL read path suitable for the pilot's 10,000-account target.

## Implemented contract

- `GET /api/me/accounts` accepts `limit`, opaque `cursor`, case-insensitive display-name/external-reference prefix or exact account-ID `q`, `status`, and `category`.
- Page size defaults to 25 and is bounded to 100. Ordering is stable by account creation time and immutable account ID.
- `GET /api/accounts/{accountId}` returns one authorized account identity/balance summary. An inaccessible and a missing account both return the same no-disclosure result.
- Account detail independently loads current balance and paginated immutable history. A failure in either path does not fabricate the other path's state.
- Directory query/filter/cursor state is carried in the URL. Opening an account and returning restores the same bounded page and focuses the originating account link.
- If an overview/transfer account scope exceeds the 100-record bounded selection page, LedgerSync suppresses partial aggregate claims and disables the incomplete dashboard account picker. The API remains the supported large-scope transfer surface until a server-backed picker is approved.

## Scale and query evidence

`TestAccountDirectoryPaginatesTenThousandAuthorizedAccountsWithStableIndexes` creates 10,000 authorized accounts in disposable PostgreSQL, traverses every 100-record cursor page, rejects duplicate IDs, exercises combined search/status/category filters, checks no-disclosure object lookup, and rejects any query plan that sequentially scans `accounts` or omits `accounts_tenant_created_stable_idx` on the common unfiltered path.

The fixture creation, analysis, 100 page reads, filter checks, authorization/audit-context checks, and plan assertion completed locally in 1.46 seconds on 24 August 2026. This is reproducibility evidence, not a production latency claim; production-like p95/p99 measurements remain a later capacity gate.

## Reproduce

```powershell
$env:LEDGERSYNC_TEST_DATABASE_URL='postgres://ledgersync:fault-test-only@127.0.0.1:15432/ledgersync?sslmode=disable'
go test ./tests/integration -run TestAccountDirectoryPaginatesTenThousandAuthorizedAccountsWithStableIndexes -count=1 -v
Push-Location web
npx playwright test tests/e2e/account-directory.spec.ts --workers=1
Pop-Location
npx --yes @redocly/cli@1.34.0 lint contracts/openapi.yaml
```

## Remaining gates

- Production-like account-list latency, database IO/CPU, and connection-pool headroom must be recorded in the Phase 5 capacity environment.
- The dashboard transfer account picker intentionally fails closed above one bounded page; a reviewed server-search picker is required before enabling console-originated transfers for such a tenant.
- Finance must approve account category and aggregate semantics before overview totals are treated as pilot-ready.
