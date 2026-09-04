-- Expand the financial capability boundary before direct workload DML is
-- revoked. The function owns one complete transfer transaction, validates the
-- caller-supplied tenant and actor against stored authorization, and emits the
-- same immutable journal, projections, audit, outbox, and replay outcome as the
-- existing adapter. Old direct grants intentionally coexist until PR-009.

CREATE FUNCTION controlled_submit_transfer_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_debit_account_id UUID,
  p_credit_account_id UUID,
  p_amount_minor BIGINT,
  p_currency TEXT,
  p_idempotency_key TEXT,
  p_request_fingerprint BYTEA,
  p_correlation_id UUID,
  p_pilot_currency TEXT,
  p_occurred_at TIMESTAMPTZ
)
RETURNS TABLE(response_body JSONB, replayed BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_now TIMESTAMPTZ := p_occurred_at;
  v_expected_fingerprint BYTEA;
  v_reserved INTEGER;
  v_existing_fingerprint BYTEA;
  v_existing_state TEXT;
  v_existing_response JSONB;
  v_account_count INTEGER;
  v_source RECORD;
  v_destination RECORD;
  v_policy RECORD;
  v_actor_total BIGINT := 0;
  v_source_total BIGINT := 0;
  v_tenant_total BIGINT := 0;
  v_transfer_id UUID := gen_random_uuid();
  v_journal_id UUID;
  v_debit_posting_id UUID;
  v_credit_posting_id UUID;
  v_source_version BIGINT;
  v_destination_version BIGINT;
  v_source_available BIGINT;
  v_source_ledger BIGINT;
  v_destination_available BIGINT;
  v_destination_ledger BIGINT;
  v_audit_id UUID := gen_random_uuid();
  v_source_event_id UUID;
  v_destination_event_id UUID;
  v_transfer_event_id UUID;
  v_transfer_payload JSONB;
  v_result JSONB;
BEGIN
  IF p_tenant_id IS NULL OR p_debit_account_id IS NULL OR p_credit_account_id IS NULL
     OR p_correlation_id IS NULL OR p_occurred_at IS NULL OR p_debit_account_id=p_credit_account_id
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_amount_minor IS NULL OR p_amount_minor<=0
     OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
     OR p_pilot_currency IS NULL OR p_pilot_currency!~'^[A-Z]{3}$'
     OR p_idempotency_key IS NULL OR p_idempotency_key!~'^[!-~]{16,255}$'
     OR p_request_fingerprint IS NULL OR octet_length(p_request_fingerprint)<>32 THEN
    RAISE EXCEPTION 'controlled transfer input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_input';
  END IF;

  v_expected_fingerprint := digest(concat_ws(E'\n',
    p_tenant_id::text,
    p_actor_subject_id,
    'transfers.create.v1',
    p_debit_account_id::text,
    p_credit_account_id::text,
    p_currency,
    p_amount_minor::text
  ), 'sha256');
  IF p_request_fingerprint<>v_expected_fingerprint THEN
    RAISE EXCEPTION 'controlled transfer fingerprint is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_fingerprint';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM public.tenants tenant WHERE tenant.id=p_tenant_id) THEN
    RAISE EXCEPTION 'controlled transfer tenant boundary denied'
      USING ERRCODE='42501', CONSTRAINT='controlled_transfer_tenant';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM public.tenants tenant
    JOIN public.tenant_subject_roles role ON role.tenant_id=tenant.id
    WHERE tenant.id=p_tenant_id AND role.subject_id=p_actor_subject_id
      AND role.role IN ('operator','finance')
  ) THEN
    RAISE EXCEPTION 'controlled transfer actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_transfer_actor';
  END IF;

  INSERT INTO public.idempotency_requests(
    tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,expires_at
  ) VALUES(
    p_tenant_id,p_actor_subject_id,'transfers.create.v1',p_idempotency_key,
    p_request_fingerprint,'in_progress',v_now+INTERVAL '30 days'
  )
  ON CONFLICT (tenant_id,actor_subject_id,operation,idempotency_key) DO NOTHING;
  GET DIAGNOSTICS v_reserved=ROW_COUNT;

  IF v_reserved=0 THEN
    SELECT request.request_fingerprint,request.state,request.response_body
      INTO v_existing_fingerprint,v_existing_state,v_existing_response
      FROM public.idempotency_requests request
     WHERE request.tenant_id=p_tenant_id
       AND request.actor_subject_id=p_actor_subject_id
       AND request.operation='transfers.create.v1'
       AND request.idempotency_key=p_idempotency_key
     FOR UPDATE;
    IF NOT FOUND THEN
      RAISE EXCEPTION 'controlled transfer replay state disappeared'
        USING ERRCODE='40001', CONSTRAINT='controlled_transfer_replay';
    END IF;
    IF v_existing_fingerprint<>p_request_fingerprint THEN
      RAISE EXCEPTION 'controlled transfer idempotency conflict'
        USING ERRCODE='23505', CONSTRAINT='controlled_transfer_idempotency';
    END IF;
    IF v_existing_state IN ('completed','failed') AND v_existing_response IS NOT NULL THEN
      RETURN QUERY SELECT v_existing_response,TRUE;
      RETURN;
    END IF;
    RAISE EXCEPTION 'controlled transfer request is already in progress'
      USING ERRCODE='55000', CONSTRAINT='controlled_transfer_in_progress';
  END IF;

  -- Resolve an existing key before applying mutable deployment policy so the
  -- original outcome/conflict remains stable across configuration changes.
  IF p_currency<>p_pilot_currency THEN
    RAISE EXCEPTION 'controlled transfer currency is outside the configured pilot'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_currency';
  END IF;

  -- One tenant policy is the exact serialization boundary for rolling limits.
  PERFORM pg_advisory_xact_lock(hashtextextended('transfer-policy|'||p_tenant_id::text,0));

  SELECT policy.currency,policy.minimum_transfer_minor,policy.maximum_transfer_minor,
         policy.actor_rolling_24h_minor,policy.source_account_rolling_24h_minor,
         policy.tenant_rolling_24h_minor,policy.policy_version
    INTO v_policy
    FROM public.tenant_transfer_policies policy
   WHERE policy.tenant_id=p_tenant_id
   FOR SHARE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled transfer policy is missing'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_policy_missing';
  END IF;
  IF v_policy.currency<>p_currency THEN
    RAISE EXCEPTION 'controlled transfer policy does not authorize currency'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_currency';
  END IF;
  IF p_amount_minor<v_policy.minimum_transfer_minor THEN
    RAISE EXCEPTION 'controlled transfer amount is below policy minimum'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_amount_minimum';
  END IF;
  IF p_amount_minor>v_policy.maximum_transfer_minor THEN
    RAISE EXCEPTION 'controlled transfer amount exceeds policy maximum'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_amount_maximum';
  END IF;

  -- Lock both financial aggregates in a stable order before reading either.
  PERFORM 1
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id
     AND account.id IN (p_debit_account_id,p_credit_account_id)
   ORDER BY account.id
   FOR UPDATE OF account,balance;
  SELECT count(*) INTO v_account_count
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id
     AND account.id IN (p_debit_account_id,p_credit_account_id);
  IF v_account_count<>2 THEN
    RAISE EXCEPTION 'controlled transfer account boundary denied'
      USING ERRCODE='42501', CONSTRAINT='controlled_transfer_tenant';
  END IF;

  SELECT account.status,account.currency,balance.available_minor,balance.ledger_minor,balance.balance_version
    INTO STRICT v_source
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id AND account.id=p_debit_account_id;
  SELECT account.status,account.currency,balance.available_minor,balance.ledger_minor,balance.balance_version
    INTO STRICT v_destination
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id AND account.id=p_credit_account_id;

  IF v_source.status<>'active' OR v_destination.status<>'active' THEN
    RAISE EXCEPTION 'controlled transfer account is inactive'
      USING ERRCODE='55000', CONSTRAINT='controlled_transfer_account_status';
  END IF;
  IF v_source.currency<>p_currency OR v_destination.currency<>p_currency THEN
    RAISE EXCEPTION 'controlled transfer account currency mismatch'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_account_currency';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.account_owners owner
    WHERE owner.tenant_id=p_tenant_id AND owner.account_id=p_debit_account_id
      AND owner.subject_id=p_actor_subject_id AND owner.permission='debit'
  ) THEN
    RAISE EXCEPTION 'controlled transfer source authorization denied'
      USING ERRCODE='42501', CONSTRAINT='controlled_transfer_source_actor';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.account_credit_permissions permission
    WHERE permission.tenant_id=p_tenant_id AND permission.account_id=p_credit_account_id
      AND permission.subject_id=p_actor_subject_id
  ) THEN
    RAISE EXCEPTION 'controlled transfer destination authorization denied'
      USING ERRCODE='42501', CONSTRAINT='controlled_transfer_destination_actor';
  END IF;

  INSERT INTO public.transfer_velocity_totals(tenant_id,dimension_type,dimension_reference,total_minor,updated_at)
  VALUES
    (p_tenant_id,'tenant',p_tenant_id::text,0,v_now),
    (p_tenant_id,'actor',p_actor_subject_id,0,v_now),
    (p_tenant_id,'source',p_debit_account_id::text,0,v_now)
  ON CONFLICT (tenant_id,dimension_type,dimension_reference) DO NOTHING;

  WITH expired AS (
    DELETE FROM public.transfer_velocity_events velocity
     WHERE velocity.tenant_id=p_tenant_id AND velocity.expires_at<=v_now
     RETURNING velocity.tenant_id,velocity.actor_subject_id,velocity.source_account_id,velocity.amount_minor
  ), deductions AS (
    SELECT tenant_id,'tenant'::text AS dimension_type,tenant_id::text AS dimension_reference,sum(amount_minor) AS amount_minor FROM expired GROUP BY tenant_id
    UNION ALL
    SELECT tenant_id,'actor',actor_subject_id,sum(amount_minor) FROM expired GROUP BY tenant_id,actor_subject_id
    UNION ALL
    SELECT tenant_id,'source',source_account_id::text,sum(amount_minor) FROM expired GROUP BY tenant_id,source_account_id
  )
  UPDATE public.transfer_velocity_totals total
     SET total_minor=total.total_minor-deduction.amount_minor,updated_at=v_now
    FROM deductions deduction
   WHERE total.tenant_id=deduction.tenant_id
     AND total.dimension_type=deduction.dimension_type
     AND total.dimension_reference=deduction.dimension_reference;

  PERFORM 1
    FROM public.transfer_velocity_totals total
   WHERE total.tenant_id=p_tenant_id AND (
     (total.dimension_type='tenant' AND total.dimension_reference=p_tenant_id::text) OR
     (total.dimension_type='actor' AND total.dimension_reference=p_actor_subject_id) OR
     (total.dimension_type='source' AND total.dimension_reference=p_debit_account_id::text)
   )
   ORDER BY total.dimension_type,total.dimension_reference
   FOR UPDATE;
  SELECT
    COALESCE(max(total.total_minor) FILTER (WHERE total.dimension_type='actor'),0),
    COALESCE(max(total.total_minor) FILTER (WHERE total.dimension_type='source'),0),
    COALESCE(max(total.total_minor) FILTER (WHERE total.dimension_type='tenant'),0)
    INTO v_actor_total,v_source_total,v_tenant_total
    FROM public.transfer_velocity_totals total
   WHERE total.tenant_id=p_tenant_id AND (
     (total.dimension_type='tenant' AND total.dimension_reference=p_tenant_id::text) OR
     (total.dimension_type='actor' AND total.dimension_reference=p_actor_subject_id) OR
     (total.dimension_type='source' AND total.dimension_reference=p_debit_account_id::text)
   );
  IF v_actor_total>v_policy.actor_rolling_24h_minor-p_amount_minor THEN
    RAISE EXCEPTION 'controlled transfer actor rolling limit exceeded'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_actor_velocity';
  END IF;
  IF v_source_total>v_policy.source_account_rolling_24h_minor-p_amount_minor THEN
    RAISE EXCEPTION 'controlled transfer source rolling limit exceeded'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_source_velocity';
  END IF;
  IF v_tenant_total>v_policy.tenant_rolling_24h_minor-p_amount_minor THEN
    RAISE EXCEPTION 'controlled transfer tenant rolling limit exceeded'
      USING ERRCODE='22023', CONSTRAINT='controlled_transfer_tenant_velocity';
  END IF;

  INSERT INTO public.transfers(
    id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,
    amount_minor,currency,status,created_at,policy_version
  ) VALUES(
    v_transfer_id,p_tenant_id,p_actor_subject_id,p_debit_account_id,p_credit_account_id,
    p_amount_minor,p_currency,'pending',v_now,v_policy.policy_version
  );

  IF v_source.available_minor<p_amount_minor OR v_source.ledger_minor<p_amount_minor THEN
    UPDATE public.transfers transfer
       SET status='rejected',rejection_code='insufficient_funds',completed_at=v_now
     WHERE transfer.id=v_transfer_id AND transfer.tenant_id=p_tenant_id AND transfer.status='pending';
    INSERT INTO public.audit_events(
      id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
      correlation_id,sanitized_metadata,occurred_at
    ) VALUES(
      v_audit_id,p_tenant_id,p_actor_subject_id,'transfer.rejected','transfer',v_transfer_id::text,'failed',
      p_correlation_id,jsonb_build_object('transfer_id',v_transfer_id::text,'function','controlled_submit_transfer_v1'),v_now
    );
    v_result := jsonb_build_object(
      'transfer_id',v_transfer_id::text,'status','rejected','currency',p_currency,
      'amount_minor',p_amount_minor::text,
      'occurred_at',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
      'minimum_balance_versions',jsonb_build_object(),
      'rejection_code','insufficient_funds'
    );
  ELSE
    v_journal_id := gen_random_uuid();
    v_debit_posting_id := gen_random_uuid();
    v_credit_posting_id := gen_random_uuid();
    INSERT INTO public.journal_transactions(id,tenant_id,transfer_id,source_type,source_id,occurred_at)
    VALUES(v_journal_id,p_tenant_id,v_transfer_id,'transfer',v_transfer_id,v_now);
    INSERT INTO public.ledger_postings(
      id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at
    ) VALUES
      (v_debit_posting_id,v_journal_id,p_tenant_id,p_debit_account_id,'debit',p_amount_minor,p_currency,v_now),
      (v_credit_posting_id,v_journal_id,p_tenant_id,p_credit_account_id,'credit',p_amount_minor,p_currency,v_now);

    UPDATE public.account_balance_projections balance
       SET available_minor=balance.available_minor-p_amount_minor,
           ledger_minor=balance.ledger_minor-p_amount_minor,
           balance_version=balance.balance_version+1,
           updated_at=v_now
     WHERE balance.account_id=p_debit_account_id
       AND balance.available_minor>=p_amount_minor AND balance.ledger_minor>=p_amount_minor
     RETURNING balance.available_minor,balance.ledger_minor,balance.balance_version
      INTO v_source_available,v_source_ledger,v_source_version;
    IF NOT FOUND THEN
      RAISE EXCEPTION 'controlled transfer source balance changed'
        USING ERRCODE='40001', CONSTRAINT='controlled_transfer_source_balance';
    END IF;
    UPDATE public.account_balance_projections balance
       SET available_minor=balance.available_minor+p_amount_minor,
           ledger_minor=balance.ledger_minor+p_amount_minor,
           balance_version=balance.balance_version+1,
           updated_at=v_now
     WHERE balance.account_id=p_credit_account_id
     RETURNING balance.available_minor,balance.ledger_minor,balance.balance_version
      INTO v_destination_available,v_destination_ledger,v_destination_version;

    UPDATE public.transfers transfer
       SET status='posted',journal_transaction_id=v_journal_id,completed_at=v_now
     WHERE transfer.id=v_transfer_id AND transfer.tenant_id=p_tenant_id AND transfer.status='pending';

    INSERT INTO public.transfer_velocity_events(
      transfer_id,tenant_id,actor_subject_id,source_account_id,amount_minor,occurred_at,expires_at
    ) VALUES(v_transfer_id,p_tenant_id,p_actor_subject_id,p_debit_account_id,p_amount_minor,v_now,v_now+INTERVAL '24 hours');
    INSERT INTO public.transfer_velocity_totals(tenant_id,dimension_type,dimension_reference,total_minor,updated_at)
    VALUES
      (p_tenant_id,'tenant',p_tenant_id::text,p_amount_minor,v_now),
      (p_tenant_id,'actor',p_actor_subject_id,p_amount_minor,v_now),
      (p_tenant_id,'source',p_debit_account_id::text,p_amount_minor,v_now)
    ON CONFLICT (tenant_id,dimension_type,dimension_reference)
    DO UPDATE SET total_minor=transfer_velocity_totals.total_minor+EXCLUDED.total_minor,updated_at=EXCLUDED.updated_at;

    INSERT INTO public.audit_events(
      id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
      correlation_id,sanitized_metadata,occurred_at
    ) VALUES(
      v_audit_id,p_tenant_id,p_actor_subject_id,'transfer.posted','transfer',v_transfer_id::text,'succeeded',
      p_correlation_id,jsonb_build_object('transfer_id',v_transfer_id::text,'function','controlled_submit_transfer_v1'),v_now
    );

    v_source_event_id := gen_random_uuid();
    INSERT INTO public.outbox_events(
      id,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at
    ) VALUES(
      v_source_event_id,p_tenant_id,v_transfer_id,p_debit_account_id,'account_balance',p_debit_account_id,
      'account.balance.changed.v1',v_source_version,
      jsonb_build_object(
        'event_id',v_source_event_id::text,'event_type','account.balance.changed.v1',
        'account_id',p_debit_account_id::text,'transfer_id',v_transfer_id::text,'currency',p_currency,
        'available_minor',v_source_available::text,'balance_version',v_source_version::text,
        'occurred_at',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
      ),v_now
    );
    v_destination_event_id := gen_random_uuid();
    INSERT INTO public.outbox_events(
      id,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at
    ) VALUES(
      v_destination_event_id,p_tenant_id,v_transfer_id,p_credit_account_id,'account_balance',p_credit_account_id,
      'account.balance.changed.v1',v_destination_version,
      jsonb_build_object(
        'event_id',v_destination_event_id::text,'event_type','account.balance.changed.v1',
        'account_id',p_credit_account_id::text,'transfer_id',v_transfer_id::text,'currency',p_currency,
        'available_minor',v_destination_available::text,'balance_version',v_destination_version::text,
        'occurred_at',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
      ),v_now
    );

    v_transfer_event_id := gen_random_uuid();
    v_transfer_payload := jsonb_build_object(
      'event_id',v_transfer_event_id::text,'event_type','transfer.posted','transfer_id',v_transfer_id::text,
      'debit_account_id',p_debit_account_id::text,'credit_account_id',p_credit_account_id::text,
      'amount_minor',p_amount_minor::text,'currency',p_currency,
      'occurred_at',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
      'delivery_semantics','at_least_once','deduplication_event_id',v_transfer_event_id::text
    );
    INSERT INTO public.outbox_events(
      id,tenant_id,transfer_id,aggregate_type,aggregate_id,event_type,aggregate_version,payload,occurred_at
    ) VALUES(
      v_transfer_event_id,p_tenant_id,v_transfer_id,'transfer',v_transfer_id,
      'transfer.posted.v1',1,v_transfer_payload,v_now
    );
    INSERT INTO public.webhook_delivery_jobs(
      id,tenant_id,transfer_id,outbox_event_id,webhook_id,event_id,event_type,payload,available_at,created_at,updated_at
    )
    SELECT gen_random_uuid(),p_tenant_id,v_transfer_id,v_transfer_event_id,endpoint.id,
           v_transfer_event_id,'transfer.posted',v_transfer_payload,v_now,v_now,v_now
      FROM public.developer_webhook_endpoints endpoint
     WHERE endpoint.tenant_id=p_tenant_id AND endpoint.status='active'
       AND 'transfer.posted'=ANY(endpoint.subscribed_events);

    v_result := jsonb_build_object(
      'transfer_id',v_transfer_id::text,'status','posted','currency',p_currency,
      'amount_minor',p_amount_minor::text,
      'occurred_at',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
      'minimum_balance_versions',jsonb_build_object(p_debit_account_id::text,v_source_version::text),
      'balances',jsonb_build_object(
        p_debit_account_id::text,jsonb_build_object(
          'account_id',p_debit_account_id::text,'currency',p_currency,
          'posted_minor',v_source_available::text,'version',v_source_version::text,
          'as_of',to_char(v_now AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
        )
      )
    );
  END IF;

  UPDATE public.idempotency_requests request
     SET state='completed',response_status=201,response_body=v_result,transfer_id=v_transfer_id,completed_at=v_now
   WHERE request.tenant_id=p_tenant_id AND request.actor_subject_id=p_actor_subject_id
     AND request.operation='transfers.create.v1' AND request.idempotency_key=p_idempotency_key
     AND request.state='in_progress';
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled transfer outcome reservation was lost'
      USING ERRCODE='40001', CONSTRAINT='controlled_transfer_outcome';
  END IF;

  RETURN QUERY SELECT v_result,FALSE;
END;
$$;

CREATE FUNCTION controlled_post_transfer_correction_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_correction_id UUID,
  p_idempotency_key TEXT,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ,
  p_step_up_authenticated_at TIMESTAMPTZ
)
RETURNS TABLE(replayed BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_correction RECORD;
  v_account_count INTEGER;
  v_source RECORD;
  v_destination RECORD;
  v_compensation_id UUID := gen_random_uuid();
  v_journal_id UUID := gen_random_uuid();
  v_debit_posting_id UUID := gen_random_uuid();
  v_credit_posting_id UUID := gen_random_uuid();
  v_source_event_id UUID := gen_random_uuid();
  v_destination_event_id UUID := gen_random_uuid();
  v_audit_id UUID := gen_random_uuid();
  v_approval_id UUID := gen_random_uuid();
  v_source_available BIGINT;
  v_source_version BIGINT;
  v_destination_available BIGINT;
  v_destination_version BIGINT;
BEGIN
  IF p_tenant_id IS NULL OR p_correction_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_idempotency_key IS NULL OR p_idempotency_key!~'^[!-~]{16,255}$' THEN
    RAISE EXCEPTION 'controlled correction input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_correction_input';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM public.tenant_subject_roles role
     WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance'
  ) THEN
    RAISE EXCEPTION 'controlled correction actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_correction_actor';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('transfer-correction-post|'||p_tenant_id::text||'|'||p_correction_id::text,0));

  SELECT correction.requester_subject_id,correction.approver_subject_id,correction.status,
         correction.control_mode,correction.step_up_required,correction.approval_expires_at,
         correction.policy_version,COALESCE(correction.post_idempotency_key,'') AS post_idempotency_key,
         original.id AS original_transfer_id,original.debit_account_id,original.credit_account_id,
         original.amount_minor,original.currency,original.status AS original_status
    INTO v_correction
    FROM public.transfer_corrections correction
    JOIN public.transfers original ON original.id=correction.original_transfer_id
   WHERE correction.tenant_id=p_tenant_id AND correction.id=p_correction_id
   FOR UPDATE OF correction,original;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled correction was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_correction_not_found';
  END IF;

  IF v_correction.status='posted' THEN
    IF v_correction.post_idempotency_key<>'' AND v_correction.post_idempotency_key<>p_idempotency_key THEN
      RAISE EXCEPTION 'controlled correction idempotency conflict'
        USING ERRCODE='23505', CONSTRAINT='controlled_correction_idempotency';
    END IF;
    RETURN QUERY SELECT TRUE;
    RETURN;
  END IF;
  IF v_correction.status<>'approved' OR v_correction.approver_subject_id IS NULL
     OR v_correction.original_status<>'posted' THEN
    RAISE EXCEPTION 'controlled correction state conflicts with posting'
      USING ERRCODE='55000', CONSTRAINT='controlled_correction_conflict';
  END IF;
  IF v_correction.control_mode='production_dual_control'
     AND (v_correction.requester_subject_id=p_actor_subject_id
       OR v_correction.requester_subject_id=v_correction.approver_subject_id) THEN
    RAISE EXCEPTION 'controlled correction requires an independent actor'
      USING ERRCODE='42501', CONSTRAINT='controlled_correction_forbidden';
  END IF;

  IF p_occurred_at>=v_correction.approval_expires_at THEN
    UPDATE public.transfer_corrections correction
       SET status='expired',decision_reason='approval_window_expired',
           cancelled_at=p_occurred_at,updated_at=p_occurred_at
     WHERE correction.tenant_id=p_tenant_id AND correction.id=p_correction_id
       AND correction.status IN ('requested','approved');
    IF NOT FOUND THEN
      RAISE EXCEPTION 'controlled correction expiry conflict'
        USING ERRCODE='55000', CONSTRAINT='controlled_correction_conflict';
    END IF;
    INSERT INTO public.approval_records(
      id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,
      status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at
    ) VALUES(
      v_approval_id,p_tenant_id,'transfer_compensation',p_correction_id,
      v_correction.requester_subject_id,NULL,'expired',p_occurred_at,
      'approval_window_expired',p_correlation_id,v_correction.policy_version,p_occurred_at,p_occurred_at
    );
    INSERT INTO public.audit_events(
      id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
      correlation_id,sanitized_metadata,occurred_at
    ) VALUES(
      v_audit_id,p_tenant_id,p_actor_subject_id,'transfer_correction.expired',
      'transfer_correction',p_correction_id::text,'failed',p_correlation_id,
      jsonb_build_object(
        'function','controlled_post_transfer_correction_v1',
        'requester_subject_id',v_correction.requester_subject_id,
        'approver_subject_id',v_correction.approver_subject_id
      ),p_occurred_at
    );
    RETURN QUERY SELECT FALSE;
    RETURN;
  END IF;

  IF v_correction.control_mode='production_dual_control' AND v_correction.step_up_required
     AND (p_step_up_authenticated_at IS NULL
       OR p_step_up_authenticated_at>p_occurred_at+INTERVAL '1 minute'
       OR p_occurred_at-p_step_up_authenticated_at>INTERVAL '10 minutes') THEN
    RAISE EXCEPTION 'controlled correction step-up is required'
      USING ERRCODE='42501', CONSTRAINT='controlled_correction_step_up';
  END IF;

  PERFORM 1
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id
     AND account.id IN (v_correction.credit_account_id,v_correction.debit_account_id)
   ORDER BY account.id
   FOR UPDATE OF account,balance;
  SELECT count(*) INTO v_account_count
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id
     AND account.id IN (v_correction.credit_account_id,v_correction.debit_account_id);
  IF v_account_count<>2 THEN
    RAISE EXCEPTION 'controlled correction account boundary denied'
      USING ERRCODE='42501', CONSTRAINT='controlled_correction_not_found';
  END IF;

  SELECT account.status,account.currency
    INTO STRICT v_source
    FROM public.accounts account
   WHERE account.tenant_id=p_tenant_id AND account.id=v_correction.credit_account_id;
  SELECT account.status,account.currency
    INTO STRICT v_destination
    FROM public.accounts account
   WHERE account.tenant_id=p_tenant_id AND account.id=v_correction.debit_account_id;
  IF v_source.status='closed' OR v_destination.status='closed'
     OR v_source.currency<>v_correction.currency OR v_destination.currency<>v_correction.currency THEN
    RAISE EXCEPTION 'controlled correction account state conflicts with posting'
      USING ERRCODE='55000', CONSTRAINT='controlled_correction_conflict';
  END IF;

  INSERT INTO public.transfers(
    id,tenant_id,actor_subject_id,debit_account_id,credit_account_id,amount_minor,currency,status,
    journal_transaction_id,created_at,completed_at,policy_version,compensation_of_transfer_id
  ) VALUES(
    v_compensation_id,p_tenant_id,p_actor_subject_id,v_correction.credit_account_id,
    v_correction.debit_account_id,v_correction.amount_minor,v_correction.currency,'posted',
    v_journal_id,p_occurred_at,p_occurred_at,v_correction.policy_version,v_correction.original_transfer_id
  );
  INSERT INTO public.journal_transactions(
    id,tenant_id,transfer_id,source_type,source_id,occurred_at
  ) VALUES(
    v_journal_id,p_tenant_id,v_compensation_id,'transfer',v_compensation_id,p_occurred_at
  );
  INSERT INTO public.ledger_postings(
    id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at
  ) VALUES
    (v_debit_posting_id,v_journal_id,p_tenant_id,v_correction.credit_account_id,'debit',v_correction.amount_minor,v_correction.currency,p_occurred_at),
    (v_credit_posting_id,v_journal_id,p_tenant_id,v_correction.debit_account_id,'credit',v_correction.amount_minor,v_correction.currency,p_occurred_at);

  UPDATE public.account_balance_projections balance
     SET available_minor=balance.available_minor-v_correction.amount_minor,
         ledger_minor=balance.ledger_minor-v_correction.amount_minor,
         balance_version=balance.balance_version+1,updated_at=p_occurred_at
   WHERE balance.account_id=v_correction.credit_account_id
     AND balance.available_minor>=v_correction.amount_minor
     AND balance.ledger_minor>=v_correction.amount_minor
  RETURNING balance.available_minor,balance.balance_version
       INTO v_source_available,v_source_version;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled correction source balance conflict'
      USING ERRCODE='23514', CONSTRAINT='controlled_correction_conflict';
  END IF;
  UPDATE public.account_balance_projections balance
     SET available_minor=balance.available_minor+v_correction.amount_minor,
         ledger_minor=balance.ledger_minor+v_correction.amount_minor,
         balance_version=balance.balance_version+1,updated_at=p_occurred_at
   WHERE balance.account_id=v_correction.debit_account_id
  RETURNING balance.available_minor,balance.balance_version
       INTO v_destination_available,v_destination_version;

  UPDATE public.transfer_corrections correction
     SET status='posted',compensation_transfer_id=v_compensation_id,
         posted_at=p_occurred_at,updated_at=p_occurred_at,post_idempotency_key=p_idempotency_key
   WHERE correction.tenant_id=p_tenant_id AND correction.id=p_correction_id
     AND correction.status='approved';
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled correction state changed'
      USING ERRCODE='40001', CONSTRAINT='controlled_correction_state';
  END IF;

  INSERT INTO public.outbox_events(
    id,tenant_id,transfer_id,account_id,aggregate_type,aggregate_id,
    event_type,aggregate_version,payload,occurred_at
  ) VALUES
    (
      v_source_event_id,p_tenant_id,v_compensation_id,v_correction.credit_account_id,
      'account_balance',v_correction.credit_account_id,'account.balance.changed.v1',v_source_version,
      jsonb_build_object(
        'event_id',v_source_event_id::text,'event_type','account.balance.changed.v1',
        'account_id',v_correction.credit_account_id::text,'transfer_id',v_compensation_id::text,
        'currency',v_correction.currency,'available_minor',v_source_available::text,
        'balance_version',v_source_version::text,
        'occurred_at',to_char(p_occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
        'amount_minor',v_correction.amount_minor::text
      ),p_occurred_at
    ),
    (
      v_destination_event_id,p_tenant_id,v_compensation_id,v_correction.debit_account_id,
      'account_balance',v_correction.debit_account_id,'account.balance.changed.v1',v_destination_version,
      jsonb_build_object(
        'event_id',v_destination_event_id::text,'event_type','account.balance.changed.v1',
        'account_id',v_correction.debit_account_id::text,'transfer_id',v_compensation_id::text,
        'currency',v_correction.currency,'available_minor',v_destination_available::text,
        'balance_version',v_destination_version::text,
        'occurred_at',to_char(p_occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
        'amount_minor',v_correction.amount_minor::text
      ),p_occurred_at
    );

  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    v_audit_id,p_tenant_id,p_actor_subject_id,'transfer_correction.posted',
    'transfer_correction',p_correction_id::text,'succeeded',p_correlation_id,
    jsonb_build_object(
      'function','controlled_post_transfer_correction_v1',
      'original_transfer_id',v_correction.original_transfer_id::text,
      'compensation_transfer_id',v_compensation_id::text,
      'requester_subject_id',v_correction.requester_subject_id,
      'approver_subject_id',v_correction.approver_subject_id,
      'step_up_verified',NOT v_correction.step_up_required OR (
        p_step_up_authenticated_at IS NOT NULL
        AND p_step_up_authenticated_at<=p_occurred_at+INTERVAL '1 minute'
        AND p_occurred_at-p_step_up_authenticated_at<=INTERVAL '10 minutes'
      )
    ),p_occurred_at
  );

  RETURN QUERY SELECT FALSE;
END;
$$;

CREATE FUNCTION controlled_post_funding_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_funding_event_id UUID,
  p_idempotency_key TEXT,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ
)
RETURNS TABLE(replayed BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_event RECORD;
  v_actor_rolling BIGINT := 0;
  v_tenant_rolling BIGINT := 0;
  v_approval_requester TEXT;
  v_approval_approver TEXT;
  v_journal_id UUID := gen_random_uuid();
  v_debit_posting_id UUID := gen_random_uuid();
  v_credit_posting_id UUID := gen_random_uuid();
  v_outbox_id UUID := gen_random_uuid();
  v_audit_id UUID := gen_random_uuid();
  v_debit_account_id UUID;
  v_credit_account_id UUID;
  v_system_delta BIGINT;
  v_destination_delta BIGINT;
  v_destination_available BIGINT;
  v_destination_version BIGINT;
  v_updated INTEGER;
BEGIN
  IF p_tenant_id IS NULL OR p_funding_event_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_idempotency_key IS NULL OR p_idempotency_key!~'^[!-~]{16,255}$' THEN
    RAISE EXCEPTION 'controlled funding input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_funding_input';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM public.tenant_subject_roles role
     WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance'
  ) THEN
    RAISE EXCEPTION 'controlled funding actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_actor';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('funding-post|'||p_tenant_id::text,0));

  SELECT event.requester_subject_id,event.destination_account_id,event.system_account_id,
         event.currency,event.amount_minor,event.status,
         COALESCE(event.compensation_of_event_id::text,'') AS compensation_of_event_id,
         COALESCE(event.post_idempotency_key,'') AS post_idempotency_key,
         policy.mode,policy.finance_activated,policy.per_command_minor,
         policy.operator_rolling_24h_minor,policy.tenant_rolling_24h_minor
    INTO v_event
    FROM public.funding_events event
    JOIN public.tenant_funding_policies policy
      ON policy.tenant_id=event.tenant_id AND policy.currency=event.currency
   WHERE event.tenant_id=p_tenant_id AND event.id=p_funding_event_id
   FOR UPDATE OF event;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled funding event was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_funding_not_found';
  END IF;

  IF v_event.status IN ('posted','compensated') THEN
    IF v_event.post_idempotency_key<>'' AND v_event.post_idempotency_key<>p_idempotency_key THEN
      RAISE EXCEPTION 'controlled funding idempotency conflict'
        USING ERRCODE='23505', CONSTRAINT='controlled_funding_idempotency';
    END IF;
    RETURN QUERY SELECT TRUE;
    RETURN;
  END IF;
  IF v_event.status<>'approved'
     OR (v_event.mode='production_dual_control' AND NOT v_event.finance_activated) THEN
    RAISE EXCEPTION 'controlled funding command is forbidden'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_forbidden';
  END IF;

  SELECT approval.requester_subject_id,approval.approver_subject_id
    INTO v_approval_requester,v_approval_approver
    FROM public.approval_records approval
   WHERE approval.tenant_id=p_tenant_id AND approval.target_id=p_funding_event_id
     AND approval.command_type=CASE WHEN v_event.compensation_of_event_id='' THEN 'funding' ELSE 'funding_compensation' END
     AND approval.status='approved' AND approval.expires_at>p_occurred_at;
  IF NOT FOUND OR v_approval_approver IS NULL THEN
    RAISE EXCEPTION 'controlled funding approval is missing or expired'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_forbidden';
  END IF;

  IF v_event.compensation_of_event_id='' THEN
    SELECT COALESCE(sum(velocity.amount_minor) FILTER (WHERE velocity.actor_subject_id=v_event.requester_subject_id),0),
           COALESCE(sum(velocity.amount_minor),0)
      INTO v_actor_rolling,v_tenant_rolling
      FROM public.funding_velocity_events velocity
     WHERE velocity.tenant_id=p_tenant_id AND velocity.expires_at>p_occurred_at;
    IF v_event.amount_minor>v_event.per_command_minor
       OR v_actor_rolling>v_event.operator_rolling_24h_minor-v_event.amount_minor
       OR v_tenant_rolling>v_event.tenant_rolling_24h_minor-v_event.amount_minor THEN
      RAISE EXCEPTION 'controlled funding limit exceeded'
        USING ERRCODE='22023', CONSTRAINT='controlled_funding_limit';
    END IF;
  END IF;

  PERFORM 1
    FROM public.accounts account
    JOIN public.account_balance_projections balance ON balance.account_id=account.id
   WHERE account.tenant_id=p_tenant_id
     AND account.id IN (v_event.destination_account_id,v_event.system_account_id)
   ORDER BY account.id
   FOR UPDATE OF account,balance;
  IF (SELECT count(*) FROM public.accounts account
       WHERE account.tenant_id=p_tenant_id
         AND account.id IN (v_event.destination_account_id,v_event.system_account_id))<>2 THEN
    RAISE EXCEPTION 'controlled funding account boundary denied'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_not_found';
  END IF;

  v_debit_account_id := v_event.system_account_id;
  v_credit_account_id := v_event.destination_account_id;
  v_system_delta := -v_event.amount_minor;
  v_destination_delta := v_event.amount_minor;
  IF v_event.compensation_of_event_id<>'' THEN
    v_debit_account_id := v_event.destination_account_id;
    v_credit_account_id := v_event.system_account_id;
    v_system_delta := v_event.amount_minor;
    v_destination_delta := -v_event.amount_minor;
  END IF;

  INSERT INTO public.journal_transactions(
    id,tenant_id,funding_event_id,source_type,source_id,occurred_at
  ) VALUES(
    v_journal_id,p_tenant_id,p_funding_event_id,'funding_event',p_funding_event_id,p_occurred_at
  );
  INSERT INTO public.ledger_postings(
    id,journal_transaction_id,tenant_id,account_id,direction,amount_minor,currency,occurred_at
  ) VALUES
    (v_debit_posting_id,v_journal_id,p_tenant_id,v_debit_account_id,'debit',v_event.amount_minor,v_event.currency,p_occurred_at),
    (v_credit_posting_id,v_journal_id,p_tenant_id,v_credit_account_id,'credit',v_event.amount_minor,v_event.currency,p_occurred_at);

  UPDATE public.account_balance_projections balance
     SET available_minor=balance.available_minor+v_system_delta,
         ledger_minor=balance.ledger_minor+v_system_delta,
         balance_version=balance.balance_version+1,updated_at=p_occurred_at
   WHERE balance.account_id=v_event.system_account_id AND balance.allow_negative;
  GET DIAGNOSTICS v_updated=ROW_COUNT;
  IF v_updated<>1 THEN
    RAISE EXCEPTION 'controlled funding clearing balance conflict'
      USING ERRCODE='23514', CONSTRAINT='controlled_funding_balance';
  END IF;

  UPDATE public.account_balance_projections balance
     SET available_minor=balance.available_minor+v_destination_delta,
         ledger_minor=balance.ledger_minor+v_destination_delta,
         balance_version=balance.balance_version+1,updated_at=p_occurred_at
   WHERE balance.account_id=v_event.destination_account_id
     AND balance.available_minor+v_destination_delta>=0
     AND balance.ledger_minor+v_destination_delta>=0
  RETURNING balance.available_minor,balance.balance_version
       INTO v_destination_available,v_destination_version;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled funding destination balance conflict'
      USING ERRCODE='23514', CONSTRAINT='controlled_funding_balance';
  END IF;

  UPDATE public.funding_events event
     SET status='posted',journal_transaction_id=v_journal_id,posted_at=p_occurred_at,
         updated_at=p_occurred_at,post_idempotency_key=p_idempotency_key
   WHERE event.tenant_id=p_tenant_id AND event.id=p_funding_event_id AND event.status='approved';
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled funding state changed'
      USING ERRCODE='40001', CONSTRAINT='controlled_funding_state';
  END IF;

  IF v_event.compensation_of_event_id='' THEN
    INSERT INTO public.funding_velocity_events(
      funding_event_id,tenant_id,actor_subject_id,amount_minor,occurred_at,expires_at
    ) VALUES(
      p_funding_event_id,p_tenant_id,v_event.requester_subject_id,v_event.amount_minor,
      p_occurred_at,p_occurred_at+INTERVAL '24 hours'
    );
  ELSE
    UPDATE public.funding_events original
       SET status='compensated',compensation_event_id=p_funding_event_id,
           compensated_at=p_occurred_at,updated_at=p_occurred_at
     WHERE original.tenant_id=p_tenant_id
       AND original.id=v_event.compensation_of_event_id::uuid AND original.status='posted';
    IF NOT FOUND THEN
      RAISE EXCEPTION 'controlled funding compensation conflict'
        USING ERRCODE='40001', CONSTRAINT='controlled_funding_state';
    END IF;
  END IF;

  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    v_audit_id,p_tenant_id,p_actor_subject_id,
    CASE WHEN v_event.compensation_of_event_id='' THEN 'funding.posted' ELSE 'funding.compensation.posted' END,
    'funding_event',p_funding_event_id::text,'succeeded',p_correlation_id,
    jsonb_build_object(
      'funding_event_id',p_funding_event_id::text,
      'terminology','recorded external value evidence',
      'function','controlled_post_funding_v1',
      'requester_subject_id',v_approval_requester,
      'approver_subject_id',v_approval_approver
    ),p_occurred_at
  );

  INSERT INTO public.outbox_events(
    id,tenant_id,funding_event_id,account_id,aggregate_type,aggregate_id,
    event_type,aggregate_version,payload,occurred_at
  ) VALUES(
    v_outbox_id,p_tenant_id,p_funding_event_id,v_event.destination_account_id,
    'account_balance',v_event.destination_account_id,'account.balance.changed.v1',v_destination_version,
    jsonb_build_object(
      'event_id',v_outbox_id::text,'event_type','account.balance.changed.v1',
      'account_id',v_event.destination_account_id::text,'funding_event_id',p_funding_event_id::text,
      'currency',v_event.currency,'available_minor',v_destination_available::text,
      'balance_version',v_destination_version::text,
      'occurred_at',to_char(p_occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
      'amount_minor',v_destination_delta::text
    ),p_occurred_at
  );

  RETURN QUERY SELECT FALSE;
END;
$$;

CREATE FUNCTION controlled_provision_account_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_account_id UUID,
  p_currency TEXT,
  p_display_name TEXT,
  p_category TEXT,
  p_external_reference TEXT,
  p_read_subject_ids TEXT[],
  p_debit_subject_ids TEXT[],
  p_credit_subject_ids TEXT[],
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
  v_actual_owners TEXT[];
  v_expected_owners TEXT[];
  v_actual_credits TEXT[];
  v_expected_credits TEXT[];
  v_audit_id UUID := gen_random_uuid();
BEGIN
  IF p_tenant_id IS NULL OR p_account_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
     OR p_display_name IS NULL OR p_display_name='' OR btrim(p_display_name)<>p_display_name
     OR p_category IS NULL OR p_category NOT IN ('customer_funds','expenses','operating','payables','payroll','reserve')
     OR p_external_reference IS NULL OR p_external_reference='' OR btrim(p_external_reference)<>p_external_reference
     OR p_read_subject_ids IS NULL OR p_debit_subject_ids IS NULL OR p_credit_subject_ids IS NULL
     OR cardinality(p_read_subject_ids)>10000 OR cardinality(p_debit_subject_ids)>10000
     OR cardinality(p_credit_subject_ids)>10000 THEN
    RAISE EXCEPTION 'controlled account provisioning input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_account_input';
  END IF;

  IF NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER')
     AND NOT EXISTS (
       SELECT 1 FROM public.tenant_subject_roles role
        WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id
          AND role.role IN ('operator','finance')
     ) THEN
    RAISE EXCEPTION 'controlled account actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_account_actor';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM public.tenants tenant WHERE tenant.id=p_tenant_id) THEN
    RAISE EXCEPTION 'controlled account tenant was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_account_not_found';
  END IF;
  IF EXISTS (
    SELECT 1
      FROM unnest(p_read_subject_ids||p_debit_subject_ids||p_credit_subject_ids) subject(subject_id)
     WHERE subject.subject_id IS NULL OR subject.subject_id='' OR btrim(subject.subject_id)<>subject.subject_id
        OR NOT EXISTS (
          SELECT 1 FROM public.tenant_subject_roles role
           WHERE role.tenant_id=p_tenant_id AND role.subject_id=subject.subject_id
        )
  ) THEN
    RAISE EXCEPTION 'controlled account permission subject is invalid'
      USING ERRCODE='42501', CONSTRAINT='controlled_account_subject';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('account-provision|'||p_tenant_id::text||'|'||p_account_id::text,0));

  SELECT account.currency,account.status,account.display_name,account.category,
         account.external_reference,account.account_kind,account.version
    INTO v_existing
    FROM public.accounts account
   WHERE account.tenant_id=p_tenant_id AND account.id=p_account_id;
  IF FOUND THEN
    SELECT COALESCE(array_agg(owner.subject_id||':'||owner.permission ORDER BY owner.subject_id),'{}'::TEXT[])
      INTO v_actual_owners
      FROM public.account_owners owner
     WHERE owner.tenant_id=p_tenant_id AND owner.account_id=p_account_id;
    SELECT COALESCE(array_agg(expected.subject_id||':'||expected.permission ORDER BY expected.subject_id),'{}'::TEXT[])
      INTO v_expected_owners
      FROM (
        SELECT candidate.subject_id,
               CASE WHEN bool_or(candidate.permission='debit') THEN 'debit' ELSE 'read' END AS permission
          FROM (
            SELECT subject_id,'read' AS permission FROM unnest(p_read_subject_ids) subject_id
            UNION ALL
            SELECT subject_id,'debit' AS permission FROM unnest(p_debit_subject_ids) subject_id
          ) candidate
         GROUP BY candidate.subject_id
      ) expected;
    SELECT COALESCE(array_agg(permission.subject_id ORDER BY permission.subject_id),'{}'::TEXT[])
      INTO v_actual_credits
      FROM public.account_credit_permissions permission
     WHERE permission.tenant_id=p_tenant_id AND permission.account_id=p_account_id;
    SELECT COALESCE(array_agg(DISTINCT subject_id ORDER BY subject_id),'{}'::TEXT[])
      INTO v_expected_credits
      FROM unnest(p_credit_subject_ids) subject_id;
    IF v_existing.currency=p_currency AND v_existing.status='active'
       AND v_existing.display_name=p_display_name AND v_existing.category=p_category
       AND v_existing.external_reference=p_external_reference
       AND v_existing.account_kind='customer' AND v_existing.version=1
       AND v_actual_owners=v_expected_owners AND v_actual_credits=v_expected_credits
       AND EXISTS (
         SELECT 1 FROM public.account_balance_projections balance
          WHERE balance.account_id=p_account_id AND balance.available_minor=0
            AND balance.ledger_minor=0 AND balance.balance_version=0
       )
       AND EXISTS (
         SELECT 1 FROM public.account_opening_balances opening
          WHERE opening.account_id=p_account_id AND opening.opening_ledger_minor=0
       ) THEN
      RETURN QUERY SELECT TRUE,FALSE;
      RETURN;
    END IF;
    RAISE EXCEPTION 'controlled account conflicts with existing state'
      USING ERRCODE='23505', CONSTRAINT='controlled_account_conflict';
  END IF;

  INSERT INTO public.accounts(
    id,tenant_id,currency,status,display_name,category,external_reference,
    account_kind,version,created_at,updated_at
  ) VALUES(
    p_account_id,p_tenant_id,p_currency,'active',p_display_name,p_category,
    p_external_reference,'customer',1,p_occurred_at,p_occurred_at
  );
  INSERT INTO public.account_balance_projections(
    account_id,available_minor,ledger_minor,balance_version,updated_at
  ) VALUES(p_account_id,0,0,0,p_occurred_at);
  INSERT INTO public.account_opening_balances(
    account_id,opening_ledger_minor,created_at
  ) VALUES(p_account_id,0,p_occurred_at);
  INSERT INTO public.account_owners(
    tenant_id,account_id,subject_id,permission,created_at
  )
  SELECT p_tenant_id,p_account_id,expected.subject_id,expected.permission,p_occurred_at
    FROM (
      SELECT candidate.subject_id,
             CASE WHEN bool_or(candidate.permission='debit') THEN 'debit' ELSE 'read' END AS permission
        FROM (
          SELECT subject_id,'read' AS permission FROM unnest(p_read_subject_ids) subject_id
          UNION ALL
          SELECT subject_id,'debit' AS permission FROM unnest(p_debit_subject_ids) subject_id
        ) candidate
       GROUP BY candidate.subject_id
    ) expected;
  INSERT INTO public.account_credit_permissions(
    tenant_id,account_id,subject_id,created_at
  )
  SELECT p_tenant_id,p_account_id,subject_id,p_occurred_at
    FROM (SELECT DISTINCT subject_id FROM unnest(p_credit_subject_ids) subject_id) expected;
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    v_audit_id,p_tenant_id,p_actor_subject_id,'account.provisioned_controlled','account',
    p_account_id::text,'succeeded',p_correlation_id,
    jsonb_build_object(
      'function','controlled_provision_account_v1',
      'opening_minor','0',
      'read_subject_count',cardinality(p_read_subject_ids),
      'debit_subject_count',cardinality(p_debit_subject_ids),
      'credit_subject_count',cardinality(p_credit_subject_ids)
    ),p_occurred_at
  );

  RETURN QUERY SELECT FALSE,FALSE;
EXCEPTION
  WHEN unique_violation THEN
    RETURN QUERY SELECT FALSE,TRUE;
END;
$$;

COMMENT ON FUNCTION controlled_submit_transfer_v1(UUID,TEXT,UUID,UUID,BIGINT,TEXT,TEXT,BYTEA,UUID,TEXT,TIMESTAMPTZ)
  IS 'Atomic, tenant-validated transfer command capability; returns the immutable idempotency outcome';

COMMENT ON FUNCTION controlled_post_funding_v1(UUID,TEXT,UUID,TEXT,UUID,TIMESTAMPTZ)
  IS 'Atomic, tenant-validated funding or funding-compensation posting capability';

COMMENT ON FUNCTION controlled_post_transfer_correction_v1(UUID,TEXT,UUID,TEXT,UUID,TIMESTAMPTZ,TIMESTAMPTZ)
  IS 'Atomic, tenant-validated exact transfer-compensation posting capability';

COMMENT ON FUNCTION controlled_provision_account_v1(UUID,TEXT,UUID,TEXT,TEXT,TEXT,TEXT,TEXT[],TEXT[],TEXT[],UUID,TIMESTAMPTZ)
  IS 'Atomic customer-account provisioning capability with a mandatory zero opening baseline';

REVOKE ALL ON FUNCTION controlled_submit_transfer_v1(UUID,TEXT,UUID,UUID,BIGINT,TEXT,TEXT,BYTEA,UUID,TEXT,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_post_funding_v1(UUID,TEXT,UUID,TEXT,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_post_transfer_correction_v1(UUID,TEXT,UUID,TEXT,UUID,TIMESTAMPTZ,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_provision_account_v1(UUID,TEXT,UUID,TEXT,TEXT,TEXT,TEXT,TEXT[],TEXT[],TEXT[],UUID,TIMESTAMPTZ) FROM PUBLIC;
