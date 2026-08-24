CREATE TABLE reconciliation_runs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    status TEXT NOT NULL CHECK (status IN ('matched', 'mismatch', 'failed')),
    checked_account_count INTEGER NOT NULL CHECK (checked_account_count >= 0),
    mismatch_count INTEGER NOT NULL CHECK (mismatch_count >= 0),
    correlation_id UUID NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX reconciliation_runs_tenant_completed_idx ON reconciliation_runs (tenant_id, completed_at DESC);
