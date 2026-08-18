# Read-your-writes violation

**Trigger:** `LedgerSyncRYEWViolation`; acceptable count is always zero.

1. Stop the release and identify the transfer ID, account ID, required version, source (`cache` or `primary`), and correlation ID from secure logs/traces.
2. Verify the authoritative PostgreSQL projection version. A cache result below the signed requirement must never have been returned as current.
3. Check worker/outbox/Redis ordering and cache version monotonicity. Rebuild cache only from PostgreSQL after recording evidence.
4. If PostgreSQL itself is below the required version, treat it as a financial consistency incident: freeze affected transfers and run reconciliation.
5. Add a regression test before closing. Do not downgrade this alert to warning.
