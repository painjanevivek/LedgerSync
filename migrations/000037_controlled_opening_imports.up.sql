-- Non-zero opening value is admitted only through an immutable, independently
-- approved manifest whose content is re-hashed and reconciled at execution.

CREATE TABLE opening_import_batches (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  requester_subject_id TEXT NOT NULL,
  currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  content_sha256 BYTEA NOT NULL CHECK (octet_length(content_sha256)=32),
  row_count INTEGER NOT NULL CHECK (row_count BETWEEN 1 AND 10000),
  total_minor BIGINT NOT NULL CHECK (total_minor > 0),
  correlation_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (id,tenant_id),
  UNIQUE (tenant_id,content_sha256)
);

CREATE TABLE opening_import_rows (
  batch_id UUID NOT NULL,
  tenant_id UUID NOT NULL,
  row_number INTEGER NOT NULL CHECK (row_number > 0),
  account_id UUID NOT NULL,
  opening_minor BIGINT NOT NULL CHECK (opening_minor > 0),
  PRIMARY KEY (batch_id,row_number),
  UNIQUE (batch_id,account_id),
  FOREIGN KEY (batch_id,tenant_id) REFERENCES opening_import_batches(id,tenant_id),
  FOREIGN KEY (account_id,tenant_id) REFERENCES accounts(id,tenant_id)
);

CREATE TABLE opening_import_approvals (
  id UUID PRIMARY KEY,
  batch_id UUID NOT NULL UNIQUE,
  tenant_id UUID NOT NULL,
  requester_subject_id TEXT NOT NULL,
  approver_subject_id TEXT NOT NULL,
  content_sha256 BYTEA NOT NULL CHECK (octet_length(content_sha256)=32),
  correlation_id UUID NOT NULL,
  approved_at TIMESTAMPTZ NOT NULL,
  CHECK (requester_subject_id<>approver_subject_id),
  FOREIGN KEY (batch_id,tenant_id) REFERENCES opening_import_batches(id,tenant_id)
);

CREATE TABLE opening_import_executions (
  id UUID PRIMARY KEY,
  batch_id UUID NOT NULL UNIQUE,
  tenant_id UUID NOT NULL,
  approval_id UUID NOT NULL UNIQUE REFERENCES opening_import_approvals(id),
  executor_subject_id TEXT NOT NULL,
  content_sha256 BYTEA NOT NULL CHECK (octet_length(content_sha256)=32),
  row_count INTEGER NOT NULL CHECK (row_count BETWEEN 1 AND 10000),
  total_minor BIGINT NOT NULL CHECK (total_minor > 0),
  correlation_id UUID NOT NULL,
  executed_at TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (batch_id,tenant_id) REFERENCES opening_import_batches(id,tenant_id)
);

CREATE TRIGGER opening_import_batches_append_only
  BEFORE UPDATE OR DELETE ON opening_import_batches
  FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER opening_import_rows_append_only
  BEFORE UPDATE OR DELETE ON opening_import_rows
  FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER opening_import_approvals_append_only
  BEFORE UPDATE OR DELETE ON opening_import_approvals
  FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
CREATE TRIGGER opening_import_executions_append_only
  BEFORE UPDATE OR DELETE ON opening_import_executions
  FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE INDEX opening_import_batches_tenant_created_idx
  ON opening_import_batches(tenant_id,created_at DESC,id DESC);
CREATE INDEX opening_import_rows_account_idx
  ON opening_import_rows(tenant_id,account_id);

ALTER TABLE outbox_events
  ADD COLUMN opening_import_id UUID REFERENCES opening_import_batches(id);
ALTER TABLE outbox_events DROP CONSTRAINT outbox_command_aggregate_consistency;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_command_aggregate_consistency CHECK (
  (aggregate_type='account_balance' AND account_id IS NOT NULL AND aggregate_id=account_id
    AND num_nonnulls(transfer_id,funding_event_id,opening_import_id)=1)
  OR (aggregate_type='account' AND transfer_id IS NULL AND funding_event_id IS NULL AND opening_import_id IS NULL AND account_id IS NOT NULL AND aggregate_id=account_id)
  OR (aggregate_type='funding_event' AND transfer_id IS NULL AND funding_event_id IS NOT NULL AND opening_import_id IS NULL AND aggregate_id=funding_event_id)
  OR (aggregate_type='transfer' AND transfer_id IS NOT NULL AND funding_event_id IS NULL AND opening_import_id IS NULL AND account_id IS NULL AND aggregate_id=transfer_id)
  OR (aggregate_type NOT IN ('account_balance','account','funding_event','transfer') AND transfer_id IS NULL AND funding_event_id IS NULL AND opening_import_id IS NULL)
);
CREATE UNIQUE INDEX outbox_opening_import_account_idx
  ON outbox_events(tenant_id,opening_import_id,account_id,event_type)
  WHERE opening_import_id IS NOT NULL;

CREATE FUNCTION controlled_request_opening_import_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_batch_id UUID,
  p_currency TEXT,
  p_account_ids UUID[],
  p_opening_minors BIGINT[],
  p_content_sha256 BYTEA,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ
)
RETURNS TABLE(replayed BOOLEAN, conflicted BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_existing RECORD;
  v_row_count INTEGER;
  v_valid_account_count INTEGER;
  v_total NUMERIC;
  v_computed_sha256 BYTEA;
  v_audit_id UUID := gen_random_uuid();
BEGIN
  IF p_tenant_id IS NULL OR p_batch_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
     OR p_account_ids IS NULL OR p_opening_minors IS NULL OR p_content_sha256 IS NULL
     OR octet_length(p_content_sha256)<>32
     OR cardinality(p_account_ids) NOT BETWEEN 1 AND 10000
     OR cardinality(p_account_ids)<>cardinality(p_opening_minors) THEN
    RAISE EXCEPTION 'opening import manifest input is invalid'
      USING ERRCODE='22023', CONSTRAINT='opening_import_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER') THEN
    RAISE EXCEPTION 'opening import caller is not authorized'
      USING ERRCODE='42501', CONSTRAINT='opening_import_caller';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.tenant_subject_roles role
     WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance'
  ) THEN
    RAISE EXCEPTION 'opening import requester is not authorized'
      USING ERRCODE='42501', CONSTRAINT='opening_import_actor';
  END IF;
  IF EXISTS (SELECT 1 FROM unnest(p_opening_minors) value WHERE value IS NULL OR value<=0)
     OR EXISTS (SELECT 1 FROM unnest(p_account_ids) value WHERE value IS NULL)
     OR (SELECT count(DISTINCT value) FROM unnest(p_account_ids) value)<>cardinality(p_account_ids) THEN
    RAISE EXCEPTION 'opening import rows are invalid'
      USING ERRCODE='22023', CONSTRAINT='opening_import_rows';
  END IF;

  SELECT cardinality(p_account_ids),sum(value::NUMERIC)
    INTO v_row_count,v_total
    FROM unnest(p_opening_minors) value;
  IF v_total>9223372036854775807 THEN
    RAISE EXCEPTION 'opening import total exceeds supported range'
      USING ERRCODE='22003', CONSTRAINT='opening_import_total';
  END IF;
  SELECT public.digest(
    convert_to(
      p_tenant_id::text||E'\n'||p_currency||E'\n'||
      string_agg(row.account_id::text||','||row.opening_minor::text,E'\n' ORDER BY row.account_id)||E'\n',
      'UTF8'
    ),'sha256'
  ) INTO v_computed_sha256
  FROM unnest(p_account_ids,p_opening_minors) row(account_id,opening_minor);
  IF v_computed_sha256<>p_content_sha256 THEN
    RAISE EXCEPTION 'opening import content hash does not match canonical rows'
      USING ERRCODE='22023', CONSTRAINT='opening_import_hash';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('opening-import|'||p_tenant_id::text||'|'||p_batch_id::text,0));
  SELECT batch.requester_subject_id,batch.currency,batch.content_sha256,batch.row_count,
         batch.total_minor,batch.correlation_id
    INTO v_existing
    FROM public.opening_import_batches batch
   WHERE batch.tenant_id=p_tenant_id AND batch.id=p_batch_id;
  IF FOUND THEN
    IF v_existing.requester_subject_id=p_actor_subject_id AND v_existing.currency=p_currency
       AND v_existing.content_sha256=p_content_sha256 AND v_existing.row_count=v_row_count
       AND v_existing.total_minor=v_total::BIGINT AND v_existing.correlation_id=p_correlation_id THEN
      RETURN QUERY SELECT TRUE,FALSE;
      RETURN;
    END IF;
    RETURN QUERY SELECT FALSE,TRUE;
    RETURN;
  END IF;

  SELECT count(*) INTO v_valid_account_count
    FROM unnest(p_account_ids) requested(account_id)
    JOIN public.accounts account ON account.id=requested.account_id AND account.tenant_id=p_tenant_id
    JOIN public.account_opening_balances opening ON opening.account_id=account.id AND opening.opening_ledger_minor=0
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
      AND balance.available_minor=0 AND balance.ledger_minor=0 AND balance.balance_version=0
   WHERE account.currency=p_currency AND account.status='active' AND account.account_kind='customer'
     AND NOT EXISTS (SELECT 1 FROM public.ledger_postings posting WHERE posting.account_id=account.id);
  IF v_valid_account_count<>v_row_count THEN
    RAISE EXCEPTION 'opening import account boundary or zero-state validation failed'
      USING ERRCODE='55000', CONSTRAINT='opening_import_account_state';
  END IF;

  INSERT INTO public.opening_import_batches(
    id,tenant_id,requester_subject_id,currency,content_sha256,row_count,total_minor,
    correlation_id,created_at
  ) VALUES(
    p_batch_id,p_tenant_id,p_actor_subject_id,p_currency,p_content_sha256,v_row_count,
    v_total::BIGINT,p_correlation_id,p_occurred_at
  );
  INSERT INTO public.opening_import_rows(batch_id,tenant_id,row_number,account_id,opening_minor)
  SELECT p_batch_id,p_tenant_id,row_number() OVER (ORDER BY row.account_id),row.account_id,row.opening_minor
    FROM unnest(p_account_ids,p_opening_minors) row(account_id,opening_minor)
   ORDER BY row.account_id;
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    v_audit_id,p_tenant_id,p_actor_subject_id,'opening_import.requested','opening_import',
    p_batch_id::text,'succeeded',p_correlation_id,
    jsonb_build_object(
      'function','controlled_request_opening_import_v1','row_count',v_row_count,
      'total_minor',v_total::TEXT,'content_sha256',encode(p_content_sha256,'hex')
    ),p_occurred_at
  );
  RETURN QUERY SELECT FALSE,FALSE;
EXCEPTION
  WHEN unique_violation THEN
    RETURN QUERY SELECT FALSE,TRUE;
END;
$$;

CREATE FUNCTION controlled_approve_opening_import_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_batch_id UUID,
  p_content_sha256 BYTEA,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ
)
RETURNS TABLE(replayed BOOLEAN, conflicted BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_batch RECORD;
  v_existing RECORD;
  v_approval_id UUID := gen_random_uuid();
  v_audit_id UUID := gen_random_uuid();
BEGIN
  IF p_tenant_id IS NULL OR p_batch_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_content_sha256 IS NULL OR octet_length(p_content_sha256)<>32 THEN
    RAISE EXCEPTION 'opening import approval input is invalid'
      USING ERRCODE='22023', CONSTRAINT='opening_import_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER') THEN
    RAISE EXCEPTION 'opening import caller is not authorized'
      USING ERRCODE='42501', CONSTRAINT='opening_import_caller';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.tenant_subject_roles role
     WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance'
  ) THEN
    RAISE EXCEPTION 'opening import approver is not authorized'
      USING ERRCODE='42501', CONSTRAINT='opening_import_actor';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('opening-import|'||p_tenant_id::text||'|'||p_batch_id::text,0));
  SELECT batch.requester_subject_id,batch.content_sha256
    INTO v_batch
    FROM public.opening_import_batches batch
   WHERE batch.tenant_id=p_tenant_id AND batch.id=p_batch_id
   FOR SHARE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'opening import manifest was not found'
      USING ERRCODE='P0002', CONSTRAINT='opening_import_not_found';
  END IF;
  IF v_batch.content_sha256<>p_content_sha256 THEN
    RAISE EXCEPTION 'opening import approval hash mismatch'
      USING ERRCODE='22023', CONSTRAINT='opening_import_hash';
  END IF;
  IF v_batch.requester_subject_id=p_actor_subject_id THEN
    RAISE EXCEPTION 'opening import requires an independent approver'
      USING ERRCODE='42501', CONSTRAINT='opening_import_dual_control';
  END IF;
  IF EXISTS (SELECT 1 FROM public.opening_import_executions execution WHERE execution.batch_id=p_batch_id) THEN
    RETURN QUERY SELECT FALSE,TRUE;
    RETURN;
  END IF;
  SELECT approval.approver_subject_id,approval.content_sha256
    INTO v_existing
    FROM public.opening_import_approvals approval
   WHERE approval.batch_id=p_batch_id;
  IF FOUND THEN
    IF v_existing.approver_subject_id=p_actor_subject_id AND v_existing.content_sha256=p_content_sha256 THEN
      RETURN QUERY SELECT TRUE,FALSE;
    ELSE
      RETURN QUERY SELECT FALSE,TRUE;
    END IF;
    RETURN;
  END IF;

  INSERT INTO public.opening_import_approvals(
    id,batch_id,tenant_id,requester_subject_id,approver_subject_id,content_sha256,
    correlation_id,approved_at
  ) VALUES(
    v_approval_id,p_batch_id,p_tenant_id,v_batch.requester_subject_id,p_actor_subject_id,
    p_content_sha256,p_correlation_id,p_occurred_at
  );
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    v_audit_id,p_tenant_id,p_actor_subject_id,'opening_import.approved','opening_import',
    p_batch_id::text,'succeeded',p_correlation_id,
    jsonb_build_object(
      'function','controlled_approve_opening_import_v1',
      'requester_subject_id',v_batch.requester_subject_id,
      'approver_subject_id',p_actor_subject_id,
      'content_sha256',encode(p_content_sha256,'hex')
    ),p_occurred_at
  );
  RETURN QUERY SELECT FALSE,FALSE;
EXCEPTION
  WHEN unique_violation THEN
    RETURN QUERY SELECT FALSE,TRUE;
END;
$$;

CREATE FUNCTION controlled_execute_opening_import_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_batch_id UUID,
  p_content_sha256 BYTEA,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ
)
RETURNS TABLE(replayed BOOLEAN, conflicted BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_batch RECORD;
  v_approval RECORD;
  v_existing RECORD;
  v_row_count INTEGER;
  v_locked_count INTEGER;
  v_updated_count INTEGER;
  v_total NUMERIC;
  v_canonical_rows TEXT;
  v_computed_sha256 BYTEA;
  v_execution_id UUID := gen_random_uuid();
  v_audit_id UUID := gen_random_uuid();
BEGIN
  IF p_tenant_id IS NULL OR p_batch_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_content_sha256 IS NULL OR octet_length(p_content_sha256)<>32 THEN
    RAISE EXCEPTION 'opening import execution input is invalid'
      USING ERRCODE='22023', CONSTRAINT='opening_import_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER') THEN
    RAISE EXCEPTION 'opening import caller is not authorized'
      USING ERRCODE='42501', CONSTRAINT='opening_import_caller';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.tenant_subject_roles role
     WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance'
  ) THEN
    RAISE EXCEPTION 'opening import executor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='opening_import_actor';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('opening-import|'||p_tenant_id::text||'|'||p_batch_id::text,0));
  SELECT batch.requester_subject_id,batch.currency,batch.content_sha256,batch.row_count,batch.total_minor
    INTO v_batch
    FROM public.opening_import_batches batch
   WHERE batch.tenant_id=p_tenant_id AND batch.id=p_batch_id
   FOR SHARE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'opening import manifest was not found'
      USING ERRCODE='P0002', CONSTRAINT='opening_import_not_found';
  END IF;
  IF v_batch.content_sha256<>p_content_sha256 THEN
    RAISE EXCEPTION 'opening import execution hash mismatch'
      USING ERRCODE='22023', CONSTRAINT='opening_import_hash';
  END IF;
  SELECT approval.id,approval.requester_subject_id,approval.approver_subject_id,approval.content_sha256
    INTO v_approval
    FROM public.opening_import_approvals approval
   WHERE approval.batch_id=p_batch_id;
  IF NOT FOUND OR v_approval.content_sha256<>p_content_sha256
     OR v_approval.requester_subject_id=v_approval.approver_subject_id THEN
    RAISE EXCEPTION 'opening import does not have a valid independent approval'
      USING ERRCODE='55000', CONSTRAINT='opening_import_approval';
  END IF;
  SELECT execution.executor_subject_id,execution.content_sha256
    INTO v_existing
    FROM public.opening_import_executions execution
   WHERE execution.batch_id=p_batch_id;
  IF FOUND THEN
    IF v_existing.content_sha256=p_content_sha256 THEN
      RETURN QUERY SELECT TRUE,FALSE;
    ELSE
      RETURN QUERY SELECT FALSE,TRUE;
    END IF;
    RETURN;
  END IF;

  SELECT count(*),sum(row.opening_minor::NUMERIC),
         string_agg(row.account_id::text||','||row.opening_minor::text,E'\n' ORDER BY row.account_id)
    INTO v_row_count,v_total,v_canonical_rows
    FROM public.opening_import_rows row
   WHERE row.batch_id=p_batch_id AND row.tenant_id=p_tenant_id;
  v_computed_sha256 := public.digest(
    convert_to(p_tenant_id::text||E'\n'||v_batch.currency||E'\n'||v_canonical_rows||E'\n','UTF8'),
    'sha256'
  );
  IF v_row_count<>v_batch.row_count OR v_total<>v_batch.total_minor
     OR v_computed_sha256<>v_batch.content_sha256 THEN
    RAISE EXCEPTION 'opening import manifest reconciliation failed'
      USING ERRCODE='55000', CONSTRAINT='opening_import_reconciliation';
  END IF;

  PERFORM 1
    FROM public.opening_import_rows row
    JOIN public.accounts account ON account.id=row.account_id AND account.tenant_id=row.tenant_id
    JOIN public.account_opening_balances opening ON opening.account_id=account.id
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE row.batch_id=p_batch_id AND row.tenant_id=p_tenant_id
   ORDER BY account.id
   FOR UPDATE OF account,opening,balance;
  SELECT count(*) INTO v_locked_count
    FROM public.opening_import_rows row
    JOIN public.accounts account ON account.id=row.account_id AND account.tenant_id=row.tenant_id
    JOIN public.account_opening_balances opening ON opening.account_id=account.id
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE row.batch_id=p_batch_id AND row.tenant_id=p_tenant_id
     AND account.currency=v_batch.currency AND account.status='active' AND account.account_kind='customer'
     AND opening.opening_ledger_minor=0 AND balance.available_minor=0
     AND balance.ledger_minor=0 AND balance.balance_version=0
     AND NOT EXISTS (SELECT 1 FROM public.ledger_postings posting WHERE posting.account_id=account.id);
  IF v_locked_count<>v_batch.row_count THEN
    RAISE EXCEPTION 'opening import account state changed before execution'
      USING ERRCODE='55000', CONSTRAINT='opening_import_account_state';
  END IF;

  UPDATE public.account_opening_balances opening
     SET opening_ledger_minor=row.opening_minor
    FROM public.opening_import_rows row
   WHERE row.batch_id=p_batch_id AND row.tenant_id=p_tenant_id AND opening.account_id=row.account_id;
  GET DIAGNOSTICS v_updated_count = ROW_COUNT;
  IF v_updated_count<>v_batch.row_count THEN
    RAISE EXCEPTION 'opening import baseline update was incomplete'
      USING ERRCODE='55000', CONSTRAINT='opening_import_partial';
  END IF;
  UPDATE public.account_balance_projections balance
     SET available_minor=row.opening_minor,ledger_minor=row.opening_minor,
         balance_version=balance.balance_version+1,updated_at=p_occurred_at
    FROM public.opening_import_rows row
   WHERE row.batch_id=p_batch_id AND row.tenant_id=p_tenant_id AND balance.account_id=row.account_id;
  GET DIAGNOSTICS v_updated_count = ROW_COUNT;
  IF v_updated_count<>v_batch.row_count THEN
    RAISE EXCEPTION 'opening import projection update was incomplete'
      USING ERRCODE='55000', CONSTRAINT='opening_import_partial';
  END IF;

  INSERT INTO public.opening_import_executions(
    id,batch_id,tenant_id,approval_id,executor_subject_id,content_sha256,row_count,
    total_minor,correlation_id,executed_at
  ) VALUES(
    v_execution_id,p_batch_id,p_tenant_id,v_approval.id,p_actor_subject_id,p_content_sha256,
    v_batch.row_count,v_batch.total_minor,p_correlation_id,p_occurred_at
  );
  WITH events AS (
    SELECT row.account_id,row.opening_minor,gen_random_uuid() AS event_id
      FROM public.opening_import_rows row
     WHERE row.batch_id=p_batch_id AND row.tenant_id=p_tenant_id
  )
  INSERT INTO public.outbox_events(
    id,tenant_id,account_id,opening_import_id,aggregate_type,aggregate_id,event_type,aggregate_version,
    payload,occurred_at
  )
  SELECT event.event_id,p_tenant_id,event.account_id,p_batch_id,'account_balance',event.account_id,
         'account.balance.changed.v1',balance.balance_version,
         jsonb_build_object(
           'event_id',event.event_id::text,'event_type','account.balance.changed.v1',
           'account_id',event.account_id::text,'opening_import_id',p_batch_id::text,
           'currency',v_batch.currency,'available_minor',balance.available_minor::text,
           'balance_version',balance.balance_version::text,
           'occurred_at',to_char(p_occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
           'amount_minor',event.opening_minor::text
         ),p_occurred_at
    FROM events event
    JOIN public.account_balance_projections balance ON balance.account_id=event.account_id;
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    v_audit_id,p_tenant_id,p_actor_subject_id,'opening_import.executed','opening_import',
    p_batch_id::text,'succeeded',p_correlation_id,
    jsonb_build_object(
      'function','controlled_execute_opening_import_v1',
      'requester_subject_id',v_approval.requester_subject_id,
      'approver_subject_id',v_approval.approver_subject_id,
      'executor_subject_id',p_actor_subject_id,
      'row_count',v_batch.row_count,'total_minor',v_batch.total_minor::TEXT,
      'content_sha256',encode(p_content_sha256,'hex')
    ),p_occurred_at
  );
  RETURN QUERY SELECT FALSE,FALSE;
EXCEPTION
  WHEN unique_violation THEN
    RETURN QUERY SELECT FALSE,TRUE;
END;
$$;

COMMENT ON FUNCTION controlled_request_opening_import_v1(UUID,TEXT,UUID,TEXT,UUID[],BIGINT[],BYTEA,UUID,TIMESTAMPTZ)
  IS 'Stores an immutable, canonical-hash-verified opening import manifest';
COMMENT ON FUNCTION controlled_approve_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ)
  IS 'Records an immutable independent finance approval for an exact opening import hash';
COMMENT ON FUNCTION controlled_execute_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ)
  IS 'Executes an approved opening import exactly once after full hash, count, total, and zero-state reconciliation';

REVOKE ALL ON FUNCTION controlled_request_opening_import_v1(UUID,TEXT,UUID,TEXT,UUID[],BIGINT[],BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_approve_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_execute_opening_import_v1(UUID,TEXT,UUID,BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON opening_import_batches,opening_import_rows,opening_import_approvals,opening_import_executions FROM PUBLIC;
