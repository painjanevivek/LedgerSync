# Financial schema migration compatibility

LedgerSync financial migrations are forward-only in shared environments. Once a migration has touched financial records, rollback means an application rollback plus a compensating data migration—not destructive schema reversal.

Before release, the migration suite proves that all migrations apply repeatably and the pre-existing account, ledger, transfer, outbox, and reconciliation read contracts still exist after the newest additive migration. Every migration must be additive or otherwise have an approved compatibility window. The release workflow runs this suite against clean PostgreSQL before a shared deployment.

For a failed release: stop new deployment, roll application code back only when it remains schema-compatible, and create a reviewed forward migration for database remediation. Never restore a shared database merely to undo an ordinary application deploy.

## Ledger semantic-key expansion

Migration `000034_ledger_semantic_keys_expand` adds nullable command identity to journals and tenant identity to postings. It backfills in batches of 5,000, uses a five-second lock-acquisition timeout, and leaves the new composite foreign keys `NOT VALID`. Existing rows remain queryable and old writers remain compatible through narrowly scoped hydration triggers; new writers populate every fact explicitly.

Before applying it to a shared environment:

1. Complete a backup checkpoint and tenant reconciliation.
2. Confirm no long-running transaction or unbounded DDL is holding locks on journals or postings.
3. Apply the migration with the migration-owner identity. A lock timeout rolls back the entire migration; investigate the blocker before retrying.
4. Read the single row from `ledger_semantic_key_validation`. Record only row counts and mismatch counts in release evidence.
5. Require every field ending in `_mismatch_count` or beginning with `unbackfilled_` to be zero. Investigate rather than deleting or rewriting financial rows.
6. Reconcile again and compare journal/posting row counts to the pre-migration inventory.

Do not validate the four composite foreign keys in this expand release; PR-007 owns validation and semantic enforcement after production-like evidence is clean. The down migration is permitted only before a new writer depends on expanded facts. It fails with SQLSTATE `55000` if any fact cannot be reconstructed from the legacy command and account relationships.
