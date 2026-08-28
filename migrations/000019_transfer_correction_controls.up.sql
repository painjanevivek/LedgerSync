-- Phase 6: immutable transfer corrections and explainable approval controls.
-- Corrections are separate balanced transfers; original journals are never
-- rewritten, deleted, or relabeled as if they had not occurred.

ALTER TABLE tenant_subject_roles
  DROP CONSTRAINT tenant_subject_roles_role_check,
  ADD CONSTRAINT tenant_subject_roles_role_check
    CHECK (role IN ('operator','finance','support','viewer','auditor'));

ALTER TABLE tenant_transfer_policies
  ADD COLUMN policy_version BIGINT NOT NULL DEFAULT 1 CHECK (policy_version > 0),
  ADD COLUMN control_mode TEXT NOT NULL DEFAULT 'production_dual_control'
    CHECK (control_mode IN ('production_dual_control','local_demo_single_operator')),
  ADD COLUMN requires_step_up BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN approval_ttl_minutes INTEGER NOT NULL DEFAULT 1440
    CHECK (approval_ttl_minutes BETWEEN 5 AND 10080);

UPDATE tenant_transfer_policies policy
SET control_mode='local_demo_single_operator',requires_step_up=false
FROM tenants tenant
WHERE tenant.id=policy.tenant_id AND tenant.external_reference='ledgersync-local-demo';

CREATE TABLE transfer_policy_versions (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  policy_version BIGINT NOT NULL CHECK (policy_version > 0),
  currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  minimum_transfer_minor BIGINT NOT NULL CHECK (minimum_transfer_minor > 0),
  maximum_transfer_minor BIGINT NOT NULL CHECK (maximum_transfer_minor >= minimum_transfer_minor),
  actor_rolling_24h_minor BIGINT NOT NULL,
  source_account_rolling_24h_minor BIGINT NOT NULL,
  tenant_rolling_24h_minor BIGINT NOT NULL,
  control_mode TEXT NOT NULL CHECK (control_mode IN ('production_dual_control','local_demo_single_operator')),
  requires_step_up BOOLEAN NOT NULL,
  approval_ttl_minutes INTEGER NOT NULL CHECK (approval_ttl_minutes BETWEEN 5 AND 10080),
  effective_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id,policy_version)
);

INSERT INTO transfer_policy_versions(
 tenant_id,policy_version,currency,minimum_transfer_minor,maximum_transfer_minor,
 actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor,
 control_mode,requires_step_up,approval_ttl_minutes,effective_at
)
SELECT tenant_id,policy_version,currency,minimum_transfer_minor,maximum_transfer_minor,
 actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor,
 control_mode,requires_step_up,approval_ttl_minutes,updated_at
FROM tenant_transfer_policies;

CREATE OR REPLACE FUNCTION version_transfer_policy()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF ROW(NEW.currency,NEW.minimum_transfer_minor,NEW.maximum_transfer_minor,
         NEW.actor_rolling_24h_minor,NEW.source_account_rolling_24h_minor,
         NEW.tenant_rolling_24h_minor,NEW.control_mode,NEW.requires_step_up,
         NEW.approval_ttl_minutes)
     IS DISTINCT FROM
     ROW(OLD.currency,OLD.minimum_transfer_minor,OLD.maximum_transfer_minor,
         OLD.actor_rolling_24h_minor,OLD.source_account_rolling_24h_minor,
         OLD.tenant_rolling_24h_minor,OLD.control_mode,OLD.requires_step_up,
         OLD.approval_ttl_minutes) THEN
    NEW.policy_version=OLD.policy_version+1;
    NEW.updated_at=GREATEST(NEW.updated_at,now());
  ELSE
    NEW.policy_version=OLD.policy_version;
  END IF;
  RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION snapshot_transfer_policy()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO transfer_policy_versions(
   tenant_id,policy_version,currency,minimum_transfer_minor,maximum_transfer_minor,
   actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor,
   control_mode,requires_step_up,approval_ttl_minutes,effective_at
  ) VALUES(
   NEW.tenant_id,NEW.policy_version,NEW.currency,NEW.minimum_transfer_minor,NEW.maximum_transfer_minor,
   NEW.actor_rolling_24h_minor,NEW.source_account_rolling_24h_minor,NEW.tenant_rolling_24h_minor,
   NEW.control_mode,NEW.requires_step_up,NEW.approval_ttl_minutes,NEW.updated_at
  ) ON CONFLICT (tenant_id,policy_version) DO NOTHING;
  RETURN NEW;
END $$;

CREATE TRIGGER tenant_transfer_policy_version
  BEFORE UPDATE ON tenant_transfer_policies
  FOR EACH ROW EXECUTE FUNCTION version_transfer_policy();
CREATE TRIGGER tenant_transfer_policy_snapshot
  AFTER INSERT OR UPDATE ON tenant_transfer_policies
  FOR EACH ROW EXECUTE FUNCTION snapshot_transfer_policy();

ALTER TABLE transfers
  ADD COLUMN policy_version BIGINT NOT NULL DEFAULT 1 CHECK (policy_version > 0),
  ADD COLUMN compensation_of_transfer_id UUID REFERENCES transfers(id),
  ADD CONSTRAINT transfer_not_self_compensation
    CHECK (compensation_of_transfer_id IS NULL OR compensation_of_transfer_id<>id);
CREATE UNIQUE INDEX transfer_single_compensation_idx
  ON transfers(compensation_of_transfer_id)
  WHERE compensation_of_transfer_id IS NOT NULL;

CREATE TABLE transfer_corrections (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  original_transfer_id UUID NOT NULL,
  compensation_transfer_id UUID UNIQUE,
  requester_subject_id TEXT NOT NULL,
  approver_subject_id TEXT,
  reason_code TEXT NOT NULL CHECK (reason_code IN (
    'duplicate','wrong_destination','wrong_amount','customer_request',
    'operational_error','compliance_reversal'
  )),
  operator_note TEXT NOT NULL CHECK (char_length(operator_note) BETWEEN 1 AND 500),
  idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 255),
  request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint)=32),
  status TEXT NOT NULL CHECK (status IN ('requested','approved','rejected','cancelled','expired','posted')),
  decision_reason TEXT,
  policy_version BIGINT NOT NULL CHECK (policy_version > 0),
  control_mode TEXT NOT NULL CHECK (control_mode IN ('production_dual_control','local_demo_single_operator')),
  step_up_required BOOLEAN NOT NULL,
  approval_expires_at TIMESTAMPTZ NOT NULL,
  correlation_id UUID NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL,
  decided_at TIMESTAMPTZ,
  cancelled_at TIMESTAMPTZ,
  posted_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id,requester_subject_id,idempotency_key),
  UNIQUE (tenant_id,original_transfer_id),
  UNIQUE (id,tenant_id),
  FOREIGN KEY (original_transfer_id,tenant_id) REFERENCES transfers(id,tenant_id),
  FOREIGN KEY (compensation_transfer_id,tenant_id) REFERENCES transfers(id,tenant_id),
  CHECK (
    (status='requested' AND approver_subject_id IS NULL AND decided_at IS NULL AND cancelled_at IS NULL AND posted_at IS NULL AND compensation_transfer_id IS NULL)
    OR (status='approved' AND approver_subject_id IS NOT NULL AND decided_at IS NOT NULL AND cancelled_at IS NULL AND posted_at IS NULL AND compensation_transfer_id IS NULL)
    OR (status='rejected' AND approver_subject_id IS NOT NULL AND decision_reason IS NOT NULL AND decided_at IS NOT NULL AND cancelled_at IS NULL AND posted_at IS NULL AND compensation_transfer_id IS NULL)
    OR (status IN ('cancelled','expired') AND decision_reason IS NOT NULL AND cancelled_at IS NOT NULL AND posted_at IS NULL AND compensation_transfer_id IS NULL)
    OR (status='posted' AND approver_subject_id IS NOT NULL AND decided_at IS NOT NULL AND cancelled_at IS NULL AND posted_at IS NOT NULL AND compensation_transfer_id IS NOT NULL)
  )
);
CREATE INDEX transfer_corrections_tenant_status_idx
  ON transfer_corrections(tenant_id,status,requested_at DESC,id DESC);

ALTER TABLE approval_records
  DROP CONSTRAINT approval_records_command_type_check,
  DROP CONSTRAINT approval_records_status_check,
  DROP CONSTRAINT approval_records_check,
  ADD CONSTRAINT approval_records_command_type_check
    CHECK (command_type IN ('funding','funding_compensation','transfer_compensation')),
  ADD CONSTRAINT approval_records_status_check
    CHECK (status IN ('requested','approved','rejected','cancelled','expired')),
  ADD CONSTRAINT approval_records_state_check CHECK (
    (status='requested' AND approver_subject_id IS NULL AND decided_at IS NULL)
    OR (status IN ('approved','rejected') AND approver_subject_id IS NOT NULL AND decision_reason IS NOT NULL AND decided_at IS NOT NULL)
    OR (status IN ('cancelled','expired') AND decision_reason IS NOT NULL AND decided_at IS NOT NULL)
  );

CREATE OR REPLACE FUNCTION protect_transfer_correction()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' THEN
    RAISE EXCEPTION 'transfer correction evidence cannot be deleted' USING ERRCODE='55000';
  END IF;
  IF OLD.id<>NEW.id OR OLD.tenant_id<>NEW.tenant_id OR OLD.original_transfer_id<>NEW.original_transfer_id
    OR OLD.requester_subject_id<>NEW.requester_subject_id OR OLD.reason_code<>NEW.reason_code
    OR OLD.operator_note<>NEW.operator_note OR OLD.idempotency_key<>NEW.idempotency_key
    OR OLD.request_fingerprint<>NEW.request_fingerprint OR OLD.policy_version<>NEW.policy_version
    OR OLD.control_mode<>NEW.control_mode OR OLD.step_up_required<>NEW.step_up_required
    OR OLD.approval_expires_at<>NEW.approval_expires_at OR OLD.correlation_id<>NEW.correlation_id
    OR OLD.requested_at<>NEW.requested_at THEN
    RAISE EXCEPTION 'transfer correction intent is immutable' USING ERRCODE='55000';
  END IF;
  IF OLD.status IN ('rejected','cancelled','expired','posted') THEN
    RAISE EXCEPTION 'final transfer correction evidence is immutable' USING ERRCODE='55000';
  END IF;
  IF NOT ((OLD.status='requested' AND NEW.status IN ('approved','rejected','cancelled','expired'))
    OR (OLD.status='approved' AND NEW.status IN ('cancelled','expired','posted'))) THEN
    RAISE EXCEPTION 'invalid transfer correction transition' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER transfer_corrections_protect_evidence
  BEFORE UPDATE OR DELETE ON transfer_corrections
  FOR EACH ROW EXECUTE FUNCTION protect_transfer_correction();
CREATE TRIGGER transfer_policy_versions_append_only
  BEFORE UPDATE OR DELETE ON transfer_policy_versions
  FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

DO $$ BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON transfer_corrections TO ledgersync_api';
    EXECUTE 'GRANT SELECT ON transfer_policy_versions TO ledgersync_api';
  END IF;
END $$;
