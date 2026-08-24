-- Account lifecycle commands are additive to the immutable transfer ledger.
-- Missing legacy directory metadata receives deterministic, non-financial
-- values; existing non-empty values, currencies, balances, and history remain
-- unchanged.

ALTER TABLE accounts
  ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1),
  ADD COLUMN updated_at TIMESTAMPTZ;

-- 000012 intentionally permitted free-form nullable directory metadata. Keep
-- valid unique values, replace invalid/blank values deterministically, and
-- replace every member of a case-insensitive duplicate group. A candidate that
-- collides with any deterministic fallback is also replaced, making the final
-- non-closed reference set provably unique for every previously valid state.
WITH prepared AS (
  SELECT id,tenant_id,status,
    CASE
      WHEN display_name IS NOT NULL
        AND btrim(display_name)<>''
        AND char_length(btrim(display_name))<=120
        AND btrim(display_name) !~ '[[:cntrl:]]'
        THEN btrim(display_name)
      ELSE 'Account ' || left(id::text,8)
    END AS normalized_name,
    CASE
      WHEN external_reference IS NOT NULL
        AND btrim(external_reference) ~ '^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$'
        AND char_length(btrim(external_reference))<=64
        THEN btrim(external_reference)
      ELSE NULL
    END AS candidate_reference,
    'legacy-' || replace(id::text,'-','') AS fallback_reference
  FROM accounts
), candidate_counts AS (
  SELECT tenant_id,lower(candidate_reference) AS normalized_reference,count(*) AS candidate_count
  FROM prepared
  WHERE status<>'closed' AND candidate_reference IS NOT NULL
  GROUP BY tenant_id,lower(candidate_reference)
), resolved AS (
  SELECT prepared.id,prepared.normalized_name,
    CASE
      WHEN prepared.candidate_reference IS NOT NULL
        AND (prepared.status='closed' OR COALESCE(candidate_counts.candidate_count,0)=1)
        AND NOT EXISTS (
          SELECT 1 FROM prepared fallback
          WHERE fallback.tenant_id=prepared.tenant_id
            AND lower(fallback.fallback_reference)=lower(prepared.candidate_reference)
        )
        THEN prepared.candidate_reference
      ELSE prepared.fallback_reference
    END AS normalized_reference
  FROM prepared
  LEFT JOIN candidate_counts
    ON candidate_counts.tenant_id=prepared.tenant_id
   AND candidate_counts.normalized_reference=lower(prepared.candidate_reference)
)
UPDATE accounts
SET display_name=resolved.normalized_name,
    external_reference=resolved.normalized_reference,
    updated_at=accounts.created_at
FROM resolved
WHERE accounts.id=resolved.id;

ALTER TABLE accounts
  ALTER COLUMN display_name SET NOT NULL,
  ALTER COLUMN display_name SET DEFAULT 'Ledger account',
  ALTER COLUMN external_reference SET NOT NULL,
  ALTER COLUMN external_reference SET DEFAULT ('auto-' || gen_random_uuid()::text),
  ALTER COLUMN updated_at SET NOT NULL,
  ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT accounts_display_name_valid CHECK (
    btrim(display_name)=display_name AND char_length(display_name) BETWEEN 1 AND 120
    AND display_name !~ '[[:cntrl:]]'
  ),
  ADD CONSTRAINT accounts_external_reference_valid CHECK (
    btrim(external_reference)=external_reference
    AND char_length(external_reference) BETWEEN 3 AND 64
    AND external_reference ~ '^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$'
  );

CREATE UNIQUE INDEX accounts_tenant_open_reference_unique_idx
  ON accounts (tenant_id,lower(external_reference))
  WHERE status<>'closed';

CREATE INDEX accounts_tenant_lifecycle_idx
  ON accounts (tenant_id,status,updated_at DESC,id);

CREATE OR REPLACE FUNCTION protect_account_identity_and_terminal_status()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.id<>OLD.id OR NEW.tenant_id<>OLD.tenant_id OR
     NEW.currency<>OLD.currency OR NEW.created_at<>OLD.created_at THEN
    RAISE EXCEPTION 'account financial identity is immutable' USING ERRCODE='55000';
  END IF;
  IF OLD.status='closed' AND NEW IS DISTINCT FROM OLD THEN
    RAISE EXCEPTION 'closed account state is terminal' USING ERRCODE='55000';
  END IF;
  IF NEW.updated_at<OLD.updated_at THEN
    RAISE EXCEPTION 'account updated_at cannot move backwards' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER accounts_identity_and_terminal_status
BEFORE UPDATE ON accounts
FOR EACH ROW EXECUTE FUNCTION protect_account_identity_and_terminal_status();

-- Generalize the transactional outbox envelope without manufacturing transfer
-- IDs for account lifecycle events. Existing balance events retain their exact
-- transfer relationship and payload.
ALTER TABLE outbox_events
  ADD COLUMN aggregate_type TEXT NOT NULL DEFAULT 'account_balance',
  ADD COLUMN aggregate_id UUID;

UPDATE outbox_events SET aggregate_id=account_id WHERE aggregate_id IS NULL;

ALTER TABLE outbox_events
  ALTER COLUMN aggregate_id SET NOT NULL,
  ALTER COLUMN transfer_id DROP NOT NULL,
	ALTER COLUMN account_id DROP NOT NULL,
  ADD CONSTRAINT outbox_aggregate_type_valid CHECK (aggregate_type ~ '^[a-z][a-z0-9_]{0,63}$'),
  ADD CONSTRAINT outbox_transfer_aggregate_consistency CHECK (
	(aggregate_type='account_balance' AND transfer_id IS NOT NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
	OR (aggregate_type='account' AND transfer_id IS NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
	OR (aggregate_type NOT IN ('account_balance','account') AND transfer_id IS NULL)
  );

CREATE UNIQUE INDEX outbox_account_lifecycle_version_idx
  ON outbox_events (tenant_id,aggregate_id,event_type,aggregate_version)
  WHERE aggregate_type='account';

CREATE INDEX idempotency_operation_expiry_idx
  ON idempotency_requests (operation,expires_at);

-- Failed command results are terminal replay evidence just like completed
-- results. This replaces the transfer-era function in place while preserving
-- transfers.create.v1 finalization behavior.
CREATE OR REPLACE FUNCTION protect_resolved_idempotency()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR OLD.state IN ('completed','failed') THEN
    RAISE EXCEPTION 'resolved idempotency outcomes are immutable' USING ERRCODE='55000';
  END IF;
  IF NEW.tenant_id<>OLD.tenant_id OR NEW.actor_subject_id<>OLD.actor_subject_id OR
     NEW.operation<>OLD.operation OR NEW.idempotency_key<>OLD.idempotency_key OR
     NEW.request_fingerprint<>OLD.request_fingerprint OR NEW.state NOT IN ('completed','failed') THEN
    RAISE EXCEPTION 'invalid idempotency finalization' USING ERRCODE='55000';
  END IF;
  RETURN NEW;
END;
$$;

-- Preserve the server-controlled local demo operator on upgrades. Fresh local
-- databases receive the same row from the idempotent demo seed.
INSERT INTO tenant_subject_roles (tenant_id,subject_id,role)
SELECT id,'demo-operator','operator'
FROM tenants WHERE external_reference='ledgersync-local-demo'
ON CONFLICT DO NOTHING;

-- Existing installations already ran their initialization SQL. Grant only the
-- additional account-command capabilities during the forward migration when
-- the workload role is present; migration harnesses without roles still work.
DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
	EXECUTE 'GRANT SELECT ON account_opening_balances TO ledgersync_api';
	EXECUTE 'GRANT INSERT ON accounts,account_balance_projections,account_opening_balances,account_owners,account_credit_permissions TO ledgersync_api';
	EXECUTE 'GRANT UPDATE ON accounts TO ledgersync_api';
  END IF;
END;
$$;
