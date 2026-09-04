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

## Commit-time ledger semantic validation

Migration `000035_ledger_semantic_validation` refuses to start unless every count-only semantic-key mismatch is zero. It then validates the composite tenant foreign keys, makes expanded keys mandatory, and installs deferred constraint triggers for journals, postings, transfers and funding events.

The validator requires exactly two transfer postings with the command's source debit, destination credit, amount and currency. Funding requires the exact clearing/customer pair. Transfer and funding compensations must invert their original command exactly. Command, journal, posting and account tenant identities must agree, and both command-to-journal references must be mutual. Errors use SQLSTATE `23514`, the fixed constraint name `ledger_semantic_validation`, and messages without command or account identifiers.

Before rollout, run the migration compatibility rehearsal and all real-role transfer, funding, correction and reconciliation tests. After rollout, page on any `ledger_semantic_validation` failure and stop the affected write path until the emitting application version is understood. Do not weaken or disable the trigger to make a malformed writer succeed.

Disabling semantic validation is an incident action, not an ordinary rollback. The down migration requires the session-local `ledgersync.semantic_validation_rollback_reason` to contain the incident-approved reason. It records the actor, reason and time in `ledger_semantic_control_events` before removing the triggers. The incident commander and ledger owner must approve this action, preserve the marker with release evidence, and require reconciliation before and after the change.
