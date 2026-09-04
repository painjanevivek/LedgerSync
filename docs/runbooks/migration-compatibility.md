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

## Controlled transfer capability expansion

Migration `000036_controlled_financial_commands` adds `controlled_submit_transfer_v1` without revoking the legacy application grants. The fixed-search-path `SECURITY DEFINER` function is owned by the non-login migration role after `deploy/postgres/roles.sql` runs; only the API workload receives `EXECUTE`. It computes and verifies the canonical request fingerprint, validates the stored tenant/actor/account/policy boundary, serializes exact rolling limits, and commits the transfer, journal, postings, projections, velocity, audit, outbox, delivery jobs, and replay outcome as one transaction.

Roll out this expand slice in this order:

1. Apply migration `000036` as the database owner, then apply `deploy/postgres/roles.sql` to transfer ownership and grant only API execution.
2. Run the real-role capability matrix. Support, worker, reconciliation, provisioning, break-glass, and `PUBLIC` execution must be denied.
3. Deploy the function-backed transfer repository while the old direct grants still exist. Exercise posted, insufficient-funds, policy-denial, same-key replay, different-body conflict, cross-tenant, and search-path-poisoning cases.
4. Compare transfer/journal/posting/outbox counts and reconcile every pilot tenant for at least one complete operational window.
5. Stop rollout on any controlled-function SQLSTATE outside the documented application mapping, any reconciliation mismatch, replay divergence, or material latency regression.

The down migration may remove the function only before PR-009 revokes direct DML and only while the deployed application can still use the legacy path. Once the capability contract is required, repair forward. Never delete a committed transfer, journal, audit record, or outbox event to roll back this change.

## Financial DML boundary contraction

Migration `000038_revoke_direct_financial_dml` is the contract point after which workload identities cannot mutate protected financial and evidence tables directly. It adds controlled account update, funding-clearing account, provisioning rollback, and append-only audit capabilities for paths found during the PR-008 compatibility rehearsal, then removes the superseded table grants. Its down migration deliberately fails with SQLSTATE `55000`; restoring broad DML is not a safe rollback.

Follow the [database capability boundary runbook](database-capability-boundary.md) for the pre/post reconciliation, real-role smoke matrix, privilege diff, permission-denied alert, break-glass process, and forward-repair procedure. Applying `deploy/postgres/roles.sql` after the migration is mandatory because that step transfers function ownership to the non-login migration role and grants only the enumerated workload executions.
