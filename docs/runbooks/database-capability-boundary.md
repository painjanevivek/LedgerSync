# Database capability boundary

Migration `000038_revoke_direct_financial_dml` removes general workload DML from protected account, balance, journal, posting, ownership, audit, and opening-import tables. The API and provisioning applications must be procedure-capable before this migration is applied. Workload roles receive only the fixed-search-path functions and the reads needed by their documented contracts.

Funding request, approval, rejection, compensation, and posting are confined to the `controlled_*_funding_v1` functions. Transfer-correction request, approval, rejection, cancellation, expiry, and exact compensating posting are confined to the `controlled_*_transfer_correction_v1` functions. The API role retains read-only access to `funding_events`, `transfer_corrections`, and `approval_records`; it must never receive direct mutation grants on those evidence tables.

## Pre-deployment gate

1. Confirm the deployed application contains migrations `000036` through `000038` and calls the controlled transfer, funding, correction, account, audit, provisioning, and opening-import capabilities.
2. Take the approved backup checkpoint and reconcile every affected tenant. Record command, journal, posting, projection, and outbox counts.
3. Run the fresh-install and production-like upgrade suites with actual ephemeral logins inheriting exactly one workload role.
4. Diff `information_schema.role_table_grants`, `information_schema.role_routine_grants`, `pg_roles`, and protected-object owners against the committed capability matrix. Any unexplained privilege blocks rollout.
5. Apply `000038`, then apply `deploy/postgres/roles.sql`. Smoke account create/update, funding, transfer, correction, reconciliation, worker delivery, provisioning rollback, and opening import as their real identities.
6. Reconcile again. Stop the rollout on any mismatch, unexpected SQLSTATE, missing audit evidence, or post-commit outcome divergence.

## Monitoring

Alert when PostgreSQL SQLSTATE `42501` for a LedgerSync workload exceeds five events in five minutes, or immediately when it targets a controlled function. Group by deployment revision, workload identity, database, and sanitized operation name; never include SQL parameters. A spike usually means an old application revision or privilege drift. Pause the affected writer, compare the active revision and capability matrix, and repair forward.

Also alert on any membership or login change involving `ledgersync_break_glass`, any protected object owned by a workload role, any workload with `BYPASSRLS`, or any new `PUBLIC` DML/routine grant.

## Break-glass

`ledgersync_break_glass` is `NOLOGIN`, `NOINHERIT`, and has no standing schema, table, sequence, or routine grant. It is not assigned to a persistent login. Emergency access requires the external privileged-access system to create a ticket-bound, time-limited assumption with two approvers, an expiry of at most 30 minutes, session recording, and an exact command scope.

At expiry, revoke both the role membership and every temporary object grant, terminate remaining sessions, capture the privilege diff, and reconcile affected tenants. Record the assumption and revocation through the approved audit path. Never add an emergency grant to `roles.sql` or a down migration.

## Rollback

There is no routine down migration that restores direct financial DML. If the procedure-capable application fails, stop financial traffic and roll the application forward or repair the capability. During an incident only, security may authorize the narrowest object/verb grant for a named workload login; the grant must carry a ticket, explicit expiry, and revocation evidence. Reconcile before and after it. Do not broadly re-grant a workload role.
