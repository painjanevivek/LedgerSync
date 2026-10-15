# Unit 3 and Unit 4: handover

## Scope

LedgerSync remains a PostgreSQL ledger with a web console and background workers.
The study adds API replicas, worker fencing, Lamport evidence, snapshot evidence,
and an optional reconciliation scheduler. Docker Compose connects the services.

The scheduler creates internal reports. It does not move funds. PostgreSQL remains
the financial authority. Redis is a disposable projection and event transport.

## Start the normal workspace

Run these commands from the repository root in PowerShell.

```powershell
.\scripts\start-local.ps1
.\scripts\status-local.ps1
```

Open `http://127.0.0.1:3000`. The normal configuration keeps the scheduler disabled.
It preserves the configured initialization mode and data volumes.

## Start the isolated demonstration

```powershell
.\scripts\start-unit-study.ps1 -APIReplicas 2 -WorkerReplicas 2 -IncludePeer -WithScheduler
.\scripts\test-unit-study-final.ps1
```

Open `http://127.0.0.1:3102`. Port 3103 is a second web instance for cross-instance
journeys. The private API, gateway, PostgreSQL, and Redis have no product host port.
Separate Go test dependencies use loopback ports 35432 and 36379.

The study uses an exact acceptance project identity and separate synthetic data.
The two schedulers can report only on the configured demonstration tenant.
Protected credentials stay in ignored runtime files. Do not copy them into reports.

## Show the functions

1. Open **Developer**. Expand **Distributed systems**.
2. Read the ready API count and its collection time.
3. Read the historical publication evidence. It is not a live worker count.
4. Read the scheduler lease state. Refresh historical observations when required.
5. Download the recorded study evidence.
6. Open an event. Expand **Event order** to read origin, send, and receive counters.
7. Open a reconciliation record. Expand **Snapshot boundary**.

Older events can have no causal metadata. Older reports can have no snapshot
counts. The console labels those fields as unavailable.

The snapshot covers one repeatable-read database transaction. Service observations
have separate collection times. This is not a Chandy-Lamport global snapshot.

## Repeat controlled checks

Run only against synthetic study data. These commands can stop study services.

```powershell
.\scripts\test-unit-study-api-failure.ps1
.\scripts\test-unit-study-workers.ps1
.\scripts\test-unit-study-clocks.ps1
.\scripts\test-unit-study-concurrency.ps1
.\scripts\test-unit-study-snapshot.ps1
.\scripts\test-unit-study-scheduler.ps1
.\scripts\test-unit-study-performance.ps1
```

The performance runner creates a separate benchmark project. It temporarily stops
the original study database and cache to keep measurements comparable. It restores
them in its final cleanup block. The normal workspace stays outside that boundary.

Read [the performance report](performance-report.md) before making a scaling claim.
The recorded demand was approximately ten commands per second. The short,
single-machine test did not establish maximum capacity or production availability.

After a new measurement run, regenerate the report and download asset. Rebuild
the study web images to serve the new asset.

```powershell
node .\scripts\render-unit-study-performance.mjs
node .\scripts\generate-unit-study-evidence.mjs
.\scripts\start-unit-study.ps1 -APIReplicas 2 -WorkerReplicas 2 -IncludePeer -WithScheduler
```

## Disable the scheduler

```powershell
. .\scripts\unit-study-common.ps1
Invoke-LedgerSyncCompose -ComposeArguments @('--profile','scheduler','stop','reconciliation-scheduler')
```

The lease expires with database time. The previous observation can remain as
historical evidence. Existing manual reconciliation remains available.

## Remove the isolated configuration

Use the study helper. It validates the project identity and state path before use.
Check the displayed project before the remove command.

```powershell
. .\scripts\unit-study-common.ps1
$studyState.project
Invoke-LedgerSyncCompose -ComposeArguments @('--profile','*','down','--volumes','--remove-orphans')
```

This command removes synthetic study volumes. It does not target the normal
`compose` project. Keep recorded evidence and source files for review.
Use a fresh PowerShell session for normal workspace commands after study commands.

## Upgrade and recovery

1. Preserve a verified backup before a product upgrade.
2. Stop old APIs and workers before a claim-contract migration.
3. Apply migrations in order. Provision the workload roles.
4. Start the matching application images.
5. Check health, authorized reads, and financial fingerprints.

Use an additive migration to repair a failed upgrade. Do not drop committed
postings, transfers, clock evidence, or reconciliation records to restore service.
The new claim generations require matching workers. A mixed-version rolling
upgrade was not validated.

Publication is at least once. A crash after publication can produce a repeat
delivery with the same event ID. Consumers must remain idempotent and must retain
aggregate-version checks. A scheduler report can repeat after an unknown result;
its unique interval and fencing rules protect job ownership.

## Review records

- [Implementation plan](../../plans/ledgersync-unit-3-unit-4-implementation-plan.md)
- [Phase evidence](progress.md)
- [Copyable commit messages](commit-messages.md)
- [Final validation](final-validation.md)
- [UI review](ui-design-review.md)
- [Scaling measurements](performance-report.md)
- [Recovery measurements](recovery-report.json)

Kubernetes and the other optional syllabus topics remain outside this implementation.
Add them only after a course requirement or a measured use case justifies the work.
