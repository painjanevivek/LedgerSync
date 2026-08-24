-- A short-lived command marker makes an active run ID observable without
-- weakening the append-only, completed-only reconciliation_runs contract.
-- The transaction advisory lock remains the execution authority; an expired
-- marker can only be recovered after that lock is proven free.
CREATE TABLE reconciliation_run_commands (
  tenant_id UUID PRIMARY KEY REFERENCES tenants(id),
  run_id UUID NOT NULL UNIQUE,
  actor_subject_id TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  correlation_id UUID NOT NULL,
  lease_expires_at TIMESTAMPTZ NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX reconciliation_run_commands_lease_idx ON reconciliation_run_commands(lease_expires_at);

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT ON schema_migrations TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT,UPDATE,DELETE ON reconciliation_run_commands TO ledgersync_api';
    EXECUTE 'GRANT INSERT ON reconciliation_runs,reconciliation_mismatches TO ledgersync_api';
  END IF;
END $$;
