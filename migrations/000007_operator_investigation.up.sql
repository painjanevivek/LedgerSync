ALTER TABLE accounts ADD COLUMN display_name TEXT;
ALTER TABLE accounts ADD COLUMN category TEXT NOT NULL DEFAULT 'operating'
  CHECK (category IN ('operating', 'customer_funds', 'payroll', 'payables', 'expenses', 'reserve'));
ALTER TABLE accounts ADD COLUMN external_reference TEXT;

CREATE INDEX accounts_tenant_status_created_idx ON accounts (tenant_id, status, created_at DESC, id DESC);
CREATE INDEX transfers_tenant_completed_idx ON transfers (tenant_id, completed_at DESC, id DESC);
