# Optional reconciliation scheduler

The scheduler is disabled unless `LEDGERSYNC_SCHEDULER_ENABLED=true`.
Normal Compose does not start it. The isolated study adds a `scheduler` profile.
Run `scripts/start-unit-study.ps1 -APIReplicas 2 -WorkerReplicas 2 -WithScheduler`.

The dedicated login has a separate protected password.
An administrator must add each permitted tenant to `reconciliation_scheduler_tenants`.
Scoped views limit financial reads. The login cannot read base financial tables or submit transfers.
It can insert only reconciliation evidence for permitted tenants.
The study enables only its synthetic tenant. Normal data is not enabled.

## Ownership rules

1. Acquire a tenant lease through a short PostgreSQL transaction.
2. Use database time for expiry. Increase the fence after expiry.
3. Check the owner, fence, and expiry when a job is created or completed.
4. Enforce one job for each tenant and interval.
5. End the scheduling transaction before reconciliation starts.
6. Renew the lease while reconciliation runs. Cancel work if renewal fails.
7. Use the existing tenant reconciliation advisory lock.
8. Let a replacement recover an abandoned running job.

A report can commit before a process loses its job acknowledgment. A replacement can produce another report for that job. This does not repeat a financial movement. The scheduled job interval remains unique. The result is not an exactly-once execution guarantee.

## Checks

The test failed before the migration because scheduler tables were absent.
Lease takeover, stale acknowledgment, duplicate interval, and scoped identity tests passed.
Renewal failure canceled active work and prevented completion.
See [live stop and pause evidence](scheduler-takeover.json).
Manual reconciliation remained usable. Scheduled reports matched.

PostgreSQL remains a dependency. This is database-coordinated leader selection.
It does not implement Bully, Ring, or a consensus protocol.

## Disable and repair

Stop the `reconciliation-scheduler` service in the verified study project.
Set `LEDGERSYNC_SCHEDULER_ENABLED=false` in any separately configured deployment.
Remove the study overlay when returning to normal Compose.
Keep committed ledger and reconciliation records. Prefer a forward migration for repairs.
Stop all old workers before a claim-contract upgrade. Do not mix old and new claim writers.
