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
  p_correlation_id UUID
)
RETURNS TABLE(response_body JSONB, replayed BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_now TIMESTAMPTZ := clock_timestamp();
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
     OR p_correlation_id IS NULL OR p_debit_account_id=p_credit_account_id
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_amount_minor IS NULL OR p_amount_minor<=0
     OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
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

COMMENT ON FUNCTION controlled_submit_transfer_v1(UUID,TEXT,UUID,UUID,BIGINT,TEXT,TEXT,BYTEA,UUID)
  IS 'Atomic, tenant-validated transfer command capability; returns the immutable idempotency outcome';

REVOKE ALL ON FUNCTION controlled_submit_transfer_v1(UUID,TEXT,UUID,UUID,BIGINT,TEXT,TEXT,BYTEA,UUID) FROM PUBLIC;
