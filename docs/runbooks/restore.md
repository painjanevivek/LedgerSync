# Isolated PostgreSQL restore drill

This procedure proves recovery; it must never target production or a network that can reach production credentials.

1. Create an isolated project/account, private network, temporary KMS grant, and new database credentials.
2. Restore the managed PostgreSQL snapshot and continuous WAL archive to the approved point in time. Record the requested and achieved recovery timestamp.
3. Block application traffic. Apply no new migrations until the restore baseline is measured.
4. Run `reconcile --run --tenant-id <tenant-uuid>` for every pilot tenant. All runs must be `matched`; a missing opening baseline is a mismatch.
5. Start a disposable Redis instance and rebuild it solely from PostgreSQL: `reconcile --rebuild-cache --tenant-id <tenant-uuid>`.
6. Run migration compatibility plus lifecycle/recovery/account-scale tests against the restore. Confirm migration 000010 tables, append-only triggers, stable history indexes, migration 000011 account-directory indexes, dry-run retention, and inspect-only recovery work without executing a production replay.
7. Run safe idempotency replay and RYEW fault checks against the restored environment. No new posting may be created by a replay.
8. Record backup age, restore start/end, RPO, RTO, tool/output versions, reconciliation run IDs, lifecycle dry-run ID, and operator approvals in release evidence.
9. Destroy the isolated restored environment and revoke its temporary credentials/KMS grants.
