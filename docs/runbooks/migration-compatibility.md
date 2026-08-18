# Financial schema migration compatibility

LedgerSync financial migrations are forward-only in shared environments. Once a migration has touched financial records, rollback means an application rollback plus a compensating data migration—not destructive schema reversal.

Before release, the migration suite proves that all migrations apply repeatably and the pre-existing account, ledger, transfer, outbox, and reconciliation read contracts still exist after the newest additive migration. Every migration must be additive or otherwise have an approved compatibility window. The release workflow runs this suite against clean PostgreSQL before a shared deployment.

For a failed release: stop new deployment, roll application code back only when it remains schema-compatible, and create a reviewed forward migration for database remediation. Never restore a shared database merely to undo an ordinary application deploy.
