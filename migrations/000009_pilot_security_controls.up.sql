-- Phase 0C pilot controls. Policy and rate state are mutable operational data;
-- financial, audit, idempotency outcome, and reconciliation evidence remain
-- append-only once final.

CREATE TABLE tenant_transfer_policies (
  tenant_id UUID PRIMARY KEY REFERENCES tenants(id),
  currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  minimum_transfer_minor BIGINT NOT NULL CHECK (minimum_transfer_minor > 0),
  maximum_transfer_minor BIGINT NOT NULL CHECK (maximum_transfer_minor >= minimum_transfer_minor),
  actor_rolling_24h_minor BIGINT NOT NULL CHECK (actor_rolling_24h_minor >= maximum_transfer_minor),
  source_account_rolling_24h_minor BIGINT NOT NULL CHECK (source_account_rolling_24h_minor >= maximum_transfer_minor),
  tenant_rolling_24h_minor BIGINT NOT NULL CHECK (tenant_rolling_24h_minor >= GREATEST(actor_rolling_24h_minor,source_account_rolling_24h_minor)),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Existing local/demo tenants receive the demonstration-only USD policy. A
-- production pilot must explicitly configure and verify its selected currency.
INSERT INTO tenant_transfer_policies (tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor)
SELECT id,'USD',1,1000000000,5000000000,5000000000,10000000000 FROM tenants;

CREATE TABLE account_credit_permissions (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  account_id UUID NOT NULL REFERENCES accounts(id),
  subject_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (account_id,subject_id),
  FOREIGN KEY (account_id,tenant_id) REFERENCES accounts(id,tenant_id)
);
CREATE INDEX account_credit_permissions_subject_idx ON account_credit_permissions (tenant_id,subject_id,account_id);

CREATE TABLE api_rate_limit_windows (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  principal_hash BYTEA NOT NULL,
  route_key TEXT NOT NULL,
  window_started_at TIMESTAMPTZ NOT NULL,
  request_count INTEGER NOT NULL CHECK (request_count > 0),
  PRIMARY KEY (tenant_id, principal_hash, route_key, window_started_at)
);
CREATE INDEX api_rate_limit_expiry_idx ON api_rate_limit_windows (window_started_at);

CREATE INDEX transfers_tenant_actor_completed_idx
  ON transfers (tenant_id, actor_subject_id, completed_at DESC)
  WHERE status='posted';

CREATE OR REPLACE FUNCTION reject_row_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE='55000';
END;
$$;

CREATE TRIGGER journal_transactions_append_only
BEFORE UPDATE OR DELETE ON journal_transactions
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER reconciliation_runs_append_only
BEFORE UPDATE OR DELETE ON reconciliation_runs
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER reconciliation_mismatches_append_only
BEFORE UPDATE OR DELETE ON reconciliation_mismatches
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER delivery_attempts_append_only
BEFORE UPDATE OR DELETE ON delivery_attempts
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE OR REPLACE FUNCTION protect_final_transfer()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR OLD.status IN ('posted','rejected') THEN
    RAISE EXCEPTION 'final transfer outcomes are immutable' USING ERRCODE='55000';
  END IF;
  IF OLD.status<>'pending' OR NEW.status NOT IN ('posted','rejected') OR
     NEW.id<>OLD.id OR NEW.tenant_id<>OLD.tenant_id OR
     NEW.actor_subject_id<>OLD.actor_subject_id OR
     NEW.debit_account_id<>OLD.debit_account_id OR
     NEW.credit_account_id<>OLD.credit_account_id OR
     NEW.amount_minor<>OLD.amount_minor OR NEW.currency<>OLD.currency OR
     NEW.created_at<>OLD.created_at THEN
    RAISE EXCEPTION 'invalid transfer finalization' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER transfers_final_outcome_immutable
BEFORE UPDATE OR DELETE ON transfers
FOR EACH ROW EXECUTE FUNCTION protect_final_transfer();

CREATE OR REPLACE FUNCTION protect_resolved_idempotency()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR OLD.state='completed' THEN
    RAISE EXCEPTION 'resolved idempotency outcomes are immutable' USING ERRCODE='55000';
  END IF;
  IF NEW.tenant_id<>OLD.tenant_id OR NEW.actor_subject_id<>OLD.actor_subject_id OR
     NEW.operation<>OLD.operation OR NEW.idempotency_key<>OLD.idempotency_key OR
     NEW.request_fingerprint<>OLD.request_fingerprint OR NEW.state<>'completed' THEN
    RAISE EXCEPTION 'invalid idempotency finalization' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER idempotency_final_outcome_immutable
BEFORE UPDATE OR DELETE ON idempotency_requests
FOR EACH ROW EXECUTE FUNCTION protect_resolved_idempotency();

REVOKE UPDATE, DELETE ON journal_transactions, ledger_postings, audit_events,
  reconciliation_runs, reconciliation_mismatches, delivery_attempts FROM PUBLIC;
