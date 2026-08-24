# Reconciliation mismatch

**Trigger:** `LedgerSyncReconciliationMismatch` or `reconcile --run` exits 3.

1. Freeze only new transfers for the affected tenant; do not alter posted ledger rows or Redis.
2. Record the run ID, tenant ID, correlation ID, application/schema versions, ledger watermark, and alert time in the incident record.
3. Run `reconcile --run --tenant-id <tenant-uuid>` again against PostgreSQL primary. A second mismatch confirms the signal.
4. Open `/reconciliation/<run-id>` and export the cursor-addressable mismatch records. Each record preserves the account/transfer scope, expected and observed exact values, evidence type, and ledger watermark used by the run.
5. Confirm every affected account has both an `account_opening_balances` record and a projection. Missing baselines, missing/orphan projections, unintended empty scopes, and incomplete or unbalanced posted transfers are intentionally mismatches.
6. Compare the immutable ledger delta and projection at the recorded watermark. Correct a projection only through a reviewed repair migration/tool; correct a financial mistake with a compensating journal, never an update/delete of ledger postings.
7. Rebuild Redis only after PostgreSQL reconciliation is matched: `reconcile --rebuild-cache --tenant-id <tenant-uuid>`.
8. Attach the mismatch export, immutable audit event, matched follow-up run, and root cause to release evidence before re-enabling transfers.

**Never:** edit `ledger_postings`, treat Redis as evidence, or mark an alert resolved without a persisted matched reconciliation run.
