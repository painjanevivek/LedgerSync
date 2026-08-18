# Reconciliation mismatch

**Trigger:** `LedgerSyncReconciliationMismatch` or `reconcile --run` exits 3.

1. Freeze only new transfers for the affected tenant; do not alter posted ledger rows or Redis.
2. Record the run ID, tenant ID, correlation ID, deployment version, and alert time in the incident record.
3. Run `reconcile --run --tenant-id <tenant-uuid>` again against PostgreSQL primary. A second mismatch confirms the signal.
4. Confirm every affected account has an `account_opening_balances` record. A missing baseline is intentionally a mismatch and requires a reviewed migration baseline.
5. Compare the immutable ledger delta and projection. Correct a projection only through a reviewed repair migration/tool; correct a financial mistake with a compensating journal, never an update/delete of ledger postings.
6. Rebuild Redis only after PostgreSQL reconciliation is matched: `reconcile --rebuild-cache --tenant-id <tenant-uuid>`.
7. Attach the matched follow-up run and root cause to release evidence before re-enabling transfers.

**Never:** edit `ledger_postings`, treat Redis as evidence, or mark an alert resolved without a persisted matched reconciliation run.
