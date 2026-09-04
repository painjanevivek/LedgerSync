# Tenant RLS rollout

Migration `000039_tenant_rls_expand` enables row-level security on the highest-risk financial and evidence tables while retaining a temporary missing-context compatibility path. When `ledgersync.tenant_id` is present, reads are tenant-confined and mismatched writes are rejected with SQLSTATE `42501`. Core financial commands set the value with transaction-local scope so commit, rollback, retry, and pool reuse cannot leak it.

## Expand gate

1. Apply migration `000039`, then reapply `deploy/postgres/roles.sql`.
2. Confirm all 19 protected tables have RLS enabled, none are forced, all have `tenant_context_expand`, and all 17 tables with a direct `tenant_id` have the mismatch trigger.
3. Run predicate-omission and misbound-tenant tests with an actual API login. Verify a transaction sees only its tenant even without a SQL tenant predicate.
4. Run account, transfer, funding, correction, opening-import, reconciliation, worker, and support workflows. Record any path that executes with no tenant context; identifiers and SQL parameters must not be logged.
5. Verify a one-connection pool returns to compatibility-mode visibility after commit and rollback. Any retained tenant value blocks rollout.

## Monitoring and stop conditions

Count missing-context use by workload, application revision, operation, and database. Count SQLSTATE `42501` with the low-cardinality reason `tenant_context_mismatch`; never emit tenant IDs, SQL text, or parameters. Stop financial writes if mismatches occur on a current application revision, if cross-tenant mutation testing succeeds, or if a pool borrower inherits the previous transaction's context.

## Force readiness

Migration `000040_tenant_rls_force` must not ship until missing-context counts are zero for every supported path through a full soak window and support/reconciliation have explicit scoped access. The force migration removes missing-context compatibility and forces policies for the table owner. Roll back `000040` first if a legitimate path was missed; keep `000039` enabled and repair the application forward.

