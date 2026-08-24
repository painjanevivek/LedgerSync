-- Bounded operational lifecycle and reviewed recovery evidence. Financial,
-- audit, reconciliation, delivery, and provisioning evidence remains retained.

CREATE TABLE retention_runs (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  mode TEXT NOT NULL CHECK (mode IN ('dry_run','apply')),
  published_outbox_count BIGINT NOT NULL DEFAULT 0 CHECK (published_outbox_count >= 0),
  retained_idempotency_count BIGINT NOT NULL DEFAULT 0 CHECK (retained_idempotency_count >= 0),
  expired_rate_window_count BIGINT NOT NULL DEFAULT 0 CHECK (expired_rate_window_count >= 0),
  correlation_id UUID NOT NULL,
  application_version TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE outbox_replay_actions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  event_id UUID NOT NULL REFERENCES outbox_events(id),
  action TEXT NOT NULL CHECK (action IN ('approved','executed','failed')),
  actor_subject_id TEXT NOT NULL,
  reason_code TEXT NOT NULL CHECK (reason_code ~ '^[a-z0-9_]{3,64}$'),
  correlation_id UUID NOT NULL,
  sanitized_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (event_id,action,correlation_id)
);
CREATE UNIQUE INDEX outbox_replay_executed_once_idx ON outbox_replay_actions (event_id) WHERE action='executed';
CREATE INDEX outbox_replay_tenant_time_idx ON outbox_replay_actions (tenant_id,created_at DESC,id DESC);

CREATE TABLE delivery_replay_actions (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  attempt_id UUID NOT NULL REFERENCES delivery_attempts(id),
  action TEXT NOT NULL CHECK (action IN ('approved','executed','failed')),
  actor_subject_id TEXT NOT NULL,
  reason_code TEXT NOT NULL CHECK (reason_code ~ '^[a-z0-9_]{3,64}$'),
  correlation_id UUID NOT NULL,
  sanitized_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (attempt_id,action,correlation_id)
);
CREATE UNIQUE INDEX delivery_replay_executed_once_idx ON delivery_replay_actions (attempt_id) WHERE action='executed';
CREATE INDEX delivery_replay_tenant_time_idx ON delivery_replay_actions (tenant_id,created_at DESC,id DESC);

CREATE TABLE partner_provisioning_requests (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL,
  correlation_id UUID NOT NULL,
  configuration_fingerprint BYTEA NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('validated','applied','rolled_back','failed')),
  currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  account_count INTEGER NOT NULL CHECK (account_count > 0 AND account_count <= 10000),
  sanitized_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (correlation_id,status)
);

CREATE TABLE tenant_subject_roles (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  subject_id TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('operator','finance','support','viewer')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,subject_id,role)
);

-- LedgerSync stores only reviewed external IdP/client references and never a
-- client secret, private key, refresh token, or bearer token.
CREATE TABLE partner_credential_events (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  credential_reference TEXT NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('registered','revoked')),
  audience TEXT NOT NULL,
  scopes TEXT[] NOT NULL CHECK (cardinality(scopes) > 0),
  expires_at TIMESTAMPTZ NOT NULL,
  actor_subject_id TEXT NOT NULL,
  correlation_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,credential_reference,action)
);

CREATE TRIGGER retention_runs_append_only BEFORE UPDATE OR DELETE ON retention_runs FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER outbox_replay_actions_append_only BEFORE UPDATE OR DELETE ON outbox_replay_actions FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER delivery_replay_actions_append_only BEFORE UPDATE OR DELETE ON delivery_replay_actions FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER partner_provisioning_requests_append_only BEFORE UPDATE OR DELETE ON partner_provisioning_requests FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER partner_credential_events_append_only BEFORE UPDATE OR DELETE ON partner_credential_events FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE INDEX transfers_account_history_stable_idx
  ON transfers (tenant_id,debit_account_id,completed_at DESC,id DESC) WHERE status='posted';
CREATE INDEX transfers_credit_history_stable_idx
  ON transfers (tenant_id,credit_account_id,completed_at DESC,id DESC) WHERE status='posted';
CREATE INDEX outbox_published_retention_idx
  ON outbox_events (tenant_id,published_at,id) WHERE published_at IS NOT NULL AND dead_at IS NULL;
CREATE INDEX idempotency_expiry_retention_idx
  ON idempotency_requests (tenant_id,expires_at) WHERE state='completed';
