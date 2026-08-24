# PostgreSQL backup and isolated restore

PostgreSQL is LedgerSync's financial recovery authority. Redis is disposable
and must never be used to repair a balance or ledger record.

## Local-only MVP procedure

From a healthy local stack, create a bounded retained backup:

```powershell
.\scripts\backup-local.ps1 -RetentionCount 5
```

The default destination is `data/local-backups`, which is excluded from Git.
Protect that directory as financial data: do not attach its dumps to issues,
copy them into release evidence, or sync them to an unapproved service.

Validate and restore the newest backup:

```powershell
$backup = Get-ChildItem .\data\local-backups -Directory |
  Sort-Object Name -Descending |
  Select-Object -First 1
.\scripts\local-restore-drill.ps1 -BackupDirectory $backup.FullName
```

The backup command binds `pg_dump` and manifest counts to the same exported
repeatable-read snapshot, so writes committed during a backup cannot create a
false manifest mismatch. The drill then performs these steps in order:

1. Validate manifest format, required fields, byte length, and SHA-256.
2. Mutate a temporary copy and require digest rejection before any recovery
   database or volume is created.
3. Capture an opaque fingerprint and exact named-volume set from the normal
   project.
4. Create a uniquely named Compose recovery project on an internal network with
   a fresh PostgreSQL volume and memory-only Redis.
5. Restore the validated dump, apply current migrations, and compare migration,
   account, transfer, and posting counts to the manifest.
6. Require balanced two-posting journals, valid posted-transfer journals, and
   non-negative projections.
7. Reconcile the selected tenant and rebuild Redis only from restored
   PostgreSQL projections.
8. Require zero mismatches and non-empty rebuilt cache evidence.
9. Prove the normal financial fingerprint and normal volume names did not
   change.
10. Remove only the exact isolated recovery project and its new volume.

`-ValidateOnly` stops after integrity/corruption checks. `-SkipCorruptionGuard`
exists only for focused troubleshooting; it must not be used as release
evidence. A digest failure, count drift, invariant failure, mismatch, or changed
normal fingerprint blocks recovery acceptance.

## Shared or production environment procedure

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

This local logical exercise validates the procedure and compatibility. It does
not validate managed continuous WAL archiving, backup age, an achieved recovery
point, provider KMS isolation, or production RPO/RTO. Those remain mandatory
provider-backed Phase 7 evidence.
