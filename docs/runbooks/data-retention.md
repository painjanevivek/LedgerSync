# Pilot data lifecycle and bounded cleanup

This is a conservative engineering default pending written legal, privacy, finance, and jurisdiction approval. A missing approval blocks pilot data onboarding; it does not authorize shorter retention.

| Data class | Pilot handling | Automated deletion | Reason |
|---|---|---:|---|
| Transfers, journal transactions, ledger postings, account opening balances | Retain | Never | Financial explanation and reconciliation source |
| Final idempotency outcomes | Retain and count after expiry | Never | Deleting a used key could reopen duplicate execution |
| Audit, reconciliation mismatch/run, delivery-attempt, recovery and provisioning evidence | Retain | Never | Investigation and change-control evidence |
| Dead/unresolved outbox work or outbox referenced by delivery/replay evidence | Retain | Never | Recovery must remain possible and explainable |
| Successfully published, unreferenced outbox rows | Minimum 30 days | Batches of at most 10,000 | Disposable notification transport after recovery window |
| PostgreSQL API rate windows | 2 hours | Batches of at most 10,000 | Operational anti-abuse state only |
| Redis balance stream | Approximate maximum 5,000,000 entries | Redis trim | Disposable projection transport; PostgreSQL can rebuild truth |

## Procedure

1. Confirm a recent restorable backup and record its identifier.
2. Run the command without `-apply`; dry-run is the default and persists an audited `retention_runs` record.
3. Compare counts with the expected tenant volume. Stop on unexpected growth or any unresolved incident.
4. Obtain the approved change UUID, then run one bounded batch:

```text
go run ./cmd/retention -tenant-id <tenant-uuid> -correlation-id <change-uuid> -batch-size 500 -published-outbox-days 30 -apply
```

5. Check the immutable retention audit event, application version, row counts, outbox backlog, and reconciliation. Repeat only as a separately observable batch.
6. Run the isolated restore compatibility suite after migration or policy changes.

The job cannot delete final financial/audit/reconciliation/delivery/provisioning evidence, final idempotency outcomes, dead work, or outbox rows referenced by delivery/replay evidence. Database constraints and integration tests enforce those protections.
