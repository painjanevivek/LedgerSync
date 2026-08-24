-- Reconciliation and downstream delivery are positive persisted evidence.
-- Absence of a projection, mismatch row, outbox row, or delivery attempt is
-- never interpreted as success.

ALTER TABLE reconciliation_runs
  ADD COLUMN scope TEXT NOT NULL DEFAULT 'tenant_all_accounts',
  ADD COLUMN ledger_watermark TEXT NOT NULL DEFAULT '',
  ADD COLUMN application_version TEXT NOT NULL DEFAULT 'unknown',
  ADD COLUMN schema_version TEXT NOT NULL DEFAULT 'unknown',
  ADD COLUMN posting_count BIGINT NOT NULL DEFAULT 0 CHECK (posting_count >= 0);

CREATE TABLE reconciliation_mismatches (
  id UUID PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES reconciliation_runs(id),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  account_id UUID REFERENCES accounts(id),
  classification TEXT NOT NULL CHECK (classification IN (
    'scope_empty', 'projection_missing', 'opening_balance_missing',
    'ledger_balance_mismatch', 'available_balance_mismatch',
    'posted_transfer_incomplete', 'journal_unbalanced'
  )),
  currency VARCHAR(3),
  expected_minor BIGINT,
  observed_minor BIGINT,
  observed_available_minor BIGINT,
  balance_version BIGINT,
  sanitized_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL,
  CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$')
);
CREATE INDEX reconciliation_mismatches_run_idx ON reconciliation_mismatches (run_id, created_at, id);
CREATE INDEX reconciliation_mismatches_tenant_account_idx ON reconciliation_mismatches (tenant_id, account_id, created_at DESC);

CREATE TABLE delivery_attempts (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  transfer_id UUID NOT NULL,
  outbox_event_id UUID REFERENCES outbox_events(id),
  delivery_kind TEXT NOT NULL CHECK (delivery_kind IN ('webhook', 'notification')),
  endpoint_reference TEXT NOT NULL,
  attempt_number INTEGER NOT NULL CHECK (attempt_number > 0),
  status TEXT NOT NULL CHECK (status IN ('pending', 'retrying', 'delivered', 'dead')),
  response_class TEXT,
  sanitized_error_code TEXT,
  due_at TIMESTAMPTZ NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (transfer_id, delivery_kind, endpoint_reference, attempt_number),
  FOREIGN KEY (transfer_id, tenant_id) REFERENCES transfers(id, tenant_id)
);
CREATE INDEX delivery_attempts_transfer_idx ON delivery_attempts (tenant_id, transfer_id, created_at DESC, id DESC);
CREATE INDEX delivery_attempts_due_idx ON delivery_attempts (due_at, id) WHERE status IN ('pending', 'retrying');
