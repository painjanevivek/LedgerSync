-- Controlled funding records customer-authorized external value evidence. It
-- does not claim bank settlement or custody, and every posted movement remains
-- an exact, balanced, immutable journal.

ALTER TABLE accounts
  ADD COLUMN account_kind TEXT NOT NULL DEFAULT 'customer'
  CHECK (account_kind IN ('customer','funding_clearing'));

CREATE UNIQUE INDEX accounts_funding_clearing_currency_idx
  ON accounts (tenant_id,currency)
  WHERE account_kind='funding_clearing';

CREATE OR REPLACE FUNCTION protect_account_identity_and_terminal_status()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.id<>OLD.id OR NEW.tenant_id<>OLD.tenant_id OR NEW.currency<>OLD.currency
    OR NEW.created_at<>OLD.created_at OR NEW.account_kind<>OLD.account_kind THEN
    RAISE EXCEPTION 'account financial identity is immutable' USING ERRCODE='55000';
  END IF;
  IF OLD.status='closed' AND NEW IS DISTINCT FROM OLD THEN
    RAISE EXCEPTION 'closed account state is terminal' USING ERRCODE='55000';
  END IF;
  IF NEW.updated_at<OLD.updated_at THEN
    RAISE EXCEPTION 'account updated_at cannot move backwards' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END $$;

ALTER TABLE account_balance_projections
  ADD COLUMN allow_negative BOOLEAN NOT NULL DEFAULT false,
  DROP CONSTRAINT account_balance_projections_available_minor_check,
  DROP CONSTRAINT account_balance_projections_ledger_minor_check,
  ADD CONSTRAINT account_balance_available_policy CHECK (allow_negative OR available_minor >= 0),
  ADD CONSTRAINT account_balance_ledger_policy CHECK (allow_negative OR ledger_minor >= 0);

CREATE OR REPLACE FUNCTION enforce_system_negative_balance_policy()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.allow_negative AND NOT EXISTS (
    SELECT 1 FROM accounts WHERE id=NEW.account_id AND account_kind='funding_clearing'
  ) THEN
    RAISE EXCEPTION 'negative balances are reserved for funding clearing accounts' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER account_balance_system_negative_policy
  BEFORE INSERT OR UPDATE OF account_id,allow_negative ON account_balance_projections
  FOR EACH ROW EXECUTE FUNCTION enforce_system_negative_balance_policy();

CREATE TABLE tenant_funding_policies (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  mode TEXT NOT NULL CHECK (mode IN ('production_dual_control','local_demo_single_operator')),
  finance_activated BOOLEAN NOT NULL DEFAULT false,
  policy_version BIGINT NOT NULL CHECK (policy_version > 0),
  per_command_minor BIGINT NOT NULL CHECK (per_command_minor > 0),
  operator_rolling_24h_minor BIGINT NOT NULL CHECK (operator_rolling_24h_minor > 0),
  tenant_rolling_24h_minor BIGINT NOT NULL CHECK (tenant_rolling_24h_minor > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,currency),
  CHECK (operator_rolling_24h_minor >= per_command_minor),
  CHECK (tenant_rolling_24h_minor >= operator_rolling_24h_minor)
);

CREATE TABLE funding_events (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  requester_subject_id TEXT NOT NULL,
  approver_subject_id TEXT,
  destination_account_id UUID NOT NULL,
  system_account_id UUID NOT NULL,
  external_reference TEXT NOT NULL CHECK (char_length(external_reference) BETWEEN 1 AND 256),
  evidence_reference TEXT NOT NULL CHECK (char_length(evidence_reference) BETWEEN 1 AND 512),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 255),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
  amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
  currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  status TEXT NOT NULL CHECK (status IN ('requested','approved','posted','rejected','compensated')),
  decision_reason TEXT,
  demo_policy BOOLEAN NOT NULL DEFAULT false,
  policy_version BIGINT NOT NULL CHECK (policy_version > 0),
  journal_transaction_id UUID,
  compensation_of_event_id UUID,
  compensation_event_id UUID,
  compensation_reason_code TEXT CHECK (compensation_reason_code IS NULL OR char_length(compensation_reason_code) BETWEEN 1 AND 64),
  compensation_operator_note TEXT CHECK (compensation_operator_note IS NULL OR char_length(compensation_operator_note) BETWEEN 1 AND 500),
  correlation_id UUID NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL,
  approved_at TIMESTAMPTZ,
  posted_at TIMESTAMPTZ,
  rejected_at TIMESTAMPTZ,
  compensated_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id,requester_subject_id,idempotency_key),
  UNIQUE (tenant_id,external_reference),
  UNIQUE (id,tenant_id),
  FOREIGN KEY (destination_account_id,tenant_id) REFERENCES accounts(id,tenant_id),
  FOREIGN KEY (system_account_id,tenant_id) REFERENCES accounts(id,tenant_id),
  CHECK (destination_account_id <> system_account_id),
  CHECK ((compensation_of_event_id IS NULL AND compensation_reason_code IS NULL AND compensation_operator_note IS NULL)
    OR (compensation_of_event_id IS NOT NULL AND compensation_reason_code IS NOT NULL AND compensation_operator_note IS NOT NULL)),
  CHECK (
    (status='requested' AND approver_subject_id IS NULL AND approved_at IS NULL AND posted_at IS NULL AND rejected_at IS NULL AND journal_transaction_id IS NULL)
    OR (status='approved' AND approver_subject_id IS NOT NULL AND approved_at IS NOT NULL AND posted_at IS NULL AND rejected_at IS NULL AND journal_transaction_id IS NULL)
    OR (status='posted' AND approver_subject_id IS NOT NULL AND approved_at IS NOT NULL AND posted_at IS NOT NULL AND rejected_at IS NULL AND journal_transaction_id IS NOT NULL)
    OR (status='rejected' AND decision_reason IS NOT NULL AND rejected_at IS NOT NULL AND posted_at IS NULL AND journal_transaction_id IS NULL)
    OR (status='compensated' AND approver_subject_id IS NOT NULL AND posted_at IS NOT NULL AND journal_transaction_id IS NOT NULL AND compensation_event_id IS NOT NULL AND compensated_at IS NOT NULL)
  )
);

CREATE INDEX funding_events_tenant_status_idx ON funding_events (tenant_id,status,requested_at DESC,id DESC);
CREATE INDEX funding_events_destination_idx ON funding_events (tenant_id,destination_account_id,requested_at DESC,id DESC);

ALTER TABLE journal_transactions
  ALTER COLUMN transfer_id DROP NOT NULL,
  ADD COLUMN funding_event_id UUID UNIQUE REFERENCES funding_events(id),
  ADD CONSTRAINT journal_single_source CHECK ((transfer_id IS NOT NULL)::int + (funding_event_id IS NOT NULL)::int = 1);

ALTER TABLE funding_events
  ADD CONSTRAINT funding_journal_fk FOREIGN KEY (journal_transaction_id) REFERENCES journal_transactions(id),
  ADD CONSTRAINT funding_compensation_source_fk FOREIGN KEY (compensation_of_event_id) REFERENCES funding_events(id),
  ADD CONSTRAINT funding_compensation_fk FOREIGN KEY (compensation_event_id) REFERENCES funding_events(id);
CREATE UNIQUE INDEX funding_single_compensation_idx ON funding_events (compensation_of_event_id) WHERE compensation_of_event_id IS NOT NULL;

CREATE TABLE approval_records (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  command_type TEXT NOT NULL CHECK (command_type IN ('funding','funding_compensation')),
  target_id UUID NOT NULL,
  requester_subject_id TEXT NOT NULL,
  approver_subject_id TEXT,
  status TEXT NOT NULL CHECK (status IN ('requested','approved','rejected')),
  expires_at TIMESTAMPTZ NOT NULL,
  decision_reason TEXT,
  correlation_id UUID NOT NULL,
  policy_version BIGINT NOT NULL CHECK (policy_version > 0),
  created_at TIMESTAMPTZ NOT NULL,
  decided_at TIMESTAMPTZ,
  UNIQUE (tenant_id,command_type,target_id,status),
  CHECK ((status='requested' AND approver_subject_id IS NULL AND decided_at IS NULL)
    OR (status IN ('approved','rejected') AND approver_subject_id IS NOT NULL AND decision_reason IS NOT NULL AND decided_at IS NOT NULL))
);

CREATE TABLE funding_velocity_events (
  funding_event_id UUID PRIMARY KEY REFERENCES funding_events(id),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL,
  amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
  occurred_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  CHECK (expires_at=occurred_at+INTERVAL '24 hours')
);
CREATE INDEX funding_velocity_expiry_idx ON funding_velocity_events (tenant_id,expires_at,funding_event_id);

ALTER TABLE outbox_events ADD COLUMN funding_event_id UUID REFERENCES funding_events(id);
ALTER TABLE outbox_events DROP CONSTRAINT outbox_transfer_aggregate_consistency;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_command_aggregate_consistency CHECK (
  (aggregate_type='account_balance' AND account_id IS NOT NULL AND aggregate_id=account_id
    AND ((transfer_id IS NOT NULL AND funding_event_id IS NULL) OR (transfer_id IS NULL AND funding_event_id IS NOT NULL)))
  OR (aggregate_type='account' AND transfer_id IS NULL AND funding_event_id IS NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
  OR (aggregate_type='funding_event' AND transfer_id IS NULL AND funding_event_id IS NOT NULL AND aggregate_id=funding_event_id)
  OR (aggregate_type NOT IN ('account_balance','account','funding_event') AND transfer_id IS NULL AND funding_event_id IS NULL)
);
CREATE UNIQUE INDEX outbox_funding_event_version_idx
  ON outbox_events (tenant_id,funding_event_id,event_type,aggregate_version)
  WHERE funding_event_id IS NOT NULL;

CREATE OR REPLACE FUNCTION protect_funding_event()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' THEN
    RAISE EXCEPTION 'funding evidence cannot be deleted' USING ERRCODE='55000';
  END IF;
  IF OLD.id<>NEW.id OR OLD.tenant_id<>NEW.tenant_id OR OLD.requester_subject_id<>NEW.requester_subject_id
    OR OLD.destination_account_id<>NEW.destination_account_id OR OLD.system_account_id<>NEW.system_account_id
    OR OLD.external_reference<>NEW.external_reference OR OLD.evidence_reference<>NEW.evidence_reference
    OR OLD.idempotency_key<>NEW.idempotency_key OR OLD.request_fingerprint<>NEW.request_fingerprint
    OR OLD.amount_minor<>NEW.amount_minor OR OLD.currency<>NEW.currency OR OLD.policy_version<>NEW.policy_version
    OR OLD.correlation_id<>NEW.correlation_id OR OLD.requested_at<>NEW.requested_at
    OR OLD.compensation_of_event_id IS DISTINCT FROM NEW.compensation_of_event_id
    OR OLD.compensation_reason_code IS DISTINCT FROM NEW.compensation_reason_code
    OR OLD.compensation_operator_note IS DISTINCT FROM NEW.compensation_operator_note THEN
    RAISE EXCEPTION 'funding financial intent is immutable' USING ERRCODE='55000';
  END IF;
  IF OLD.status IN ('rejected','compensated') OR (OLD.status='posted' AND NEW.status<>'compensated') THEN
    RAISE EXCEPTION 'final funding evidence is immutable' USING ERRCODE='55000';
  END IF;
  IF NOT ((OLD.status='requested' AND NEW.status IN ('approved','rejected'))
    OR (OLD.status='approved' AND NEW.status='posted')
    OR (OLD.status='posted' AND NEW.status='compensated')) THEN
    RAISE EXCEPTION 'invalid funding status transition' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER funding_events_protect_evidence BEFORE UPDATE OR DELETE ON funding_events FOR EACH ROW EXECUTE FUNCTION protect_funding_event();
CREATE TRIGGER approval_records_append_only BEFORE UPDATE OR DELETE ON approval_records FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER funding_velocity_append_only BEFORE UPDATE OR DELETE ON funding_velocity_events FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON funding_events TO ledgersync_api';
    EXECUTE 'GRANT SELECT,INSERT ON approval_records,funding_velocity_events TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON tenant_funding_policies TO ledgersync_api';
  END IF;
END $$;
