# Local full-stack startup, restart, and cleanup

This runbook operates only the disposable local Compose project. It is not a production deployment procedure and its demonstration credentials must never be reused in a shared environment.

## Clean startup

1. Confirm Docker is available with `docker info`.
2. From the repository root, run `docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml up -d --build --wait`.
3. Confirm `migrate` and `demo-seed` exited with code 0; all long-running services must be healthy.
4. Open `http://localhost:3000/api/session`. A local demo session is expected only because Compose explicitly sets development/demo mode.
5. Run the real BFF smoke with a unique key:

```powershell
$env:LEDGERSYNC_SYSTEM_WEB_URL='http://localhost:3000'
$env:LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY='local-system-before-restart-000001'
go test ./tests/system -run TestRealBFFAPIAndPostgreSQLRetryPath -count=1 -v
```

The test reads authorized account/detail evidence, posts one exact minor unit, performs a requirement-bearing balance read, retries with the same key, verifies no second debit, and opens immutable transfer detail.

Re-running `docker compose ... up` may execute the one-shot seed again. That is supported: existing projection versions, balances, and opening-balance evidence must remain unchanged. The seed may only insert missing demo financial rows.

## Controlled restart proof

Restart stateless/cache components first:

```powershell
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml restart web api outbox-worker redis
$env:LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY='local-system-after-service-restart-000001'
go test ./tests/system -run TestRealBFFAPIAndPostgreSQLRetryPath -count=1 -v
```

Then restart PostgreSQL, wait for readiness, re-run the migration job, and use another unique key. Re-running migrations must be a no-op, not a schema mutation by API startup.

```powershell
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml restart postgres
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml exec postgres pg_isready -U ledgersync -d ledgersync
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml run --rm migrate
$env:LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY='local-system-after-database-restart-000001'
go test ./tests/system -run TestRealBFFAPIAndPostgreSQLRetryPath -count=1 -v
```

## Reconciliation after restart

```powershell
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml run --rm --entrypoint /usr/local/bin/reconcile migrate --run --tenant-id 00000000-0000-4000-8000-000000000001
go test ./tests/system -run TestRealBFFReconciliationEvidence -count=1 -v
```

A mismatch blocks completion. Do not overwrite or delete evidence to make the run pass.

## Runtime boundary checks

- API, worker, and web run as the `ledgersync` user with read-only root filesystems and dropped Linux capabilities.
- Redis has no host-published port.
- PostgreSQL, Redis, and the API have no host-published ports. Migrations and reconciliation run as short-lived tools inside the private network.
- Only the web/BFF is a browser-facing product surface.

## Evidence and cleanup

Capture only bounded, secret-free evidence: service health/status, one correlation or transfer ID, migration exit status, reconciliation run ID/status/count, and failing logs when applicable. Never commit environment files, cookies, bearer tokens, database dumps, or raw unbounded logs.

Remove only this explicit disposable project:

```powershell
docker compose -p ledgersync-system -f deploy/compose/docker-compose.yml down -v
```

The command deletes the named project's local PostgreSQL/Redis volumes. It does not touch the separate fault-test project or other Docker resources.
