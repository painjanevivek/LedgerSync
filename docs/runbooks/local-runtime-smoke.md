# Local full-stack startup, restart, and cleanup

This runbook operates the loopback-only Compose project on one workstation. It is not a production deployment procedure. Demo credentials, cookies, database contents, and logs must not be reused or published.

## Supported boundary

- Project name: `compose` by default, matching existing local volumes.
- Browser surface: `http://127.0.0.1:3000`, bound only to IPv4 loopback.
- Private services: PostgreSQL, Redis, API, and worker have no host-published ports.
- Financial scope: internal same-currency INR demo transfers only.
- Normal start, stop, and restart preserve the Compose-named volumes
  `<project>_postgres-data` and `<project>_redis-data`.
- PostgreSQL is the recovery authority. The Redis volume is only optional cache
  continuity and may be discarded and rebuilt from PostgreSQL.

An isolated test can set `LEDGERSYNC_LOCAL_COMPOSE_PROJECT` to a validated lowercase project name. Never point these commands at an unrelated project.

## Start or recover the product

From the repository root:

```powershell
.\scripts\start-local.ps1
```

The command performs these checks before it prints the URL:

1. Docker Desktop is running.
2. Port 3000 is free or already belongs to this exact LedgerSync web container.
3. The effective Compose configuration is valid.
4. PostgreSQL and Redis become healthy.
5. Migration and demo-seed jobs exit successfully.
6. API, worker, and web are running and healthy.
7. Session, authorized account, and reconciliation reads succeed through the web/BFF.

The seed is replay-safe: it inserts missing demo records and must never reset projections, versions, opening evidence, postings, or transfers.

## Inspect status and bounded logs

```powershell
.\scripts\status-local.ps1
.\scripts\logs-local.ps1 -Service api -Tail 200 -Since 30m
.\scripts\logs-local.ps1 -Service outbox-worker -Tail 200 -Since 30m
```

The log command accepts only known services, caps output at 1,000 lines, and redacts common credential patterns. Do not commit copied logs without reviewing them for customer data or business identifiers.

`status-local.ps1` reports the applied migration version/count, bounded outbox
counts, and the latest reconciliation result. It never prints credentials, raw
balances, complete transfers, or database contents.

## Back up and prove recovery

Create a finalized logical backup in the Git-ignored
`data/local-backups` directory:

```powershell
.\scripts\backup-local.ps1 -RetentionCount 5
```

Each backup contains `database.dump` and `manifest.json`. The dump and redacted
counts use the same exported repeatable-read PostgreSQL snapshot. The manifest
binds the dump to its byte length and SHA-256 digest and records creation UTC,
source commit, migration version, and counts. A `.partial-*` directory is
never presented as a valid backup. Rotation removes only validated backup names
inside the resolved backup root.

Prove a backup can be recovered without touching the normal volumes:

```powershell
$backup = Get-ChildItem .\data\local-backups -Directory |
  Sort-Object Name -Descending |
  Select-Object -First 1
.\scripts\local-restore-drill.ps1 -BackupDirectory $backup.FullName
```

The drill first mutates a disposable copy and proves digest rejection before
creating any database. It then uses a uniquely named internal Compose project,
applies migrations, compares manifest counts, checks double-entry invariants,
rebuilds Redis from PostgreSQL, requires zero reconciliation mismatches, proves
the normal project's fingerprint/volume set is unchanged, and removes only its
own isolated containers, network, and volume.

Run the controlled dependency suite after runtime/recovery changes:

```powershell
.\scripts\test-local-fault-recovery.ps1
```

It proves Redis flush/unavailability, worker and stateless-service restarts,
sanitized PostgreSQL unavailability, full dependency-order recovery, cache
rebuild, and a final unchanged authoritative fingerprint.

## Stop without deleting data

```powershell
.\scripts\stop-local.ps1
```

This uses Compose `stop`, not `down --volumes`. A later `start-local.ps1` reuses the same PostgreSQL and Redis volumes.

## Controlled restart proof

After the initial healthy start, capture only schema version and opaque evidence identifiers—not raw balances. Restart each service, then require the stack and real BFF path to recover:

```powershell
docker compose -p compose -f deploy/compose/docker-compose.yml restart redis
docker compose -p compose -f deploy/compose/docker-compose.yml restart postgres
docker compose -p compose -f deploy/compose/docker-compose.yml restart api
docker compose -p compose -f deploy/compose/docker-compose.yml restart outbox-worker
docker compose -p compose -f deploy/compose/docker-compose.yml restart web
.\scripts\start-local.ps1 -SkipBuild
```

Then run the idempotent retry and reconciliation proof with a new test key:

```powershell
$env:LEDGERSYNC_SYSTEM_WEB_URL='http://127.0.0.1:3000'
$env:LEDGERSYNC_SYSTEM_IDEMPOTENCY_KEY='local-system-after-restart-000001'
go test ./tests/system -run 'TestRealBFFAPIAndPostgreSQLRetryPath|TestRealBFFReconciliationEvidence' -count=1 -v
```

Reconciliation mismatch, second movement on retry, lost authorization, or changed schema/data evidence blocks the local release.

## Deliberately destructive reset

Reset is not a cleanup step. It permanently removes this exact project's local PostgreSQL and Redis volumes and all local ledger data:

```powershell
.\scripts\reset-local.ps1 -Confirmation 'DELETE LEDGERSYNC LOCAL DATA'
```

The script refuses any other confirmation. Never run it during ordinary testing, stopping, rebuilding, or troubleshooting. Back up first if the local ledger matters.

## Failure handling

- If Docker is unavailable, start Docker Desktop and retry.
- If port 3000 belongs to another process, stop that process yourself or change its port; LedgerSync will not terminate it.
- If a setup job fails, inspect its bounded logs and repair the cause. Do not bypass migration or seed success.
- If a dependency remains unavailable past the bounded startup window, the process exits so Compose restart and health state remain truthful.
- If PostgreSQL data is suspect, stop transfers and follow the [restore runbook](restore.md); never edit ledger rows to make a check pass.
