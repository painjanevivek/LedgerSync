-- Contract the financial write boundary only after every supported workload
-- path has a fixed-search-path capability. Workload identities retain reads
-- needed by their application contracts, but cannot mutate protected state by
-- issuing arbitrary table DML.

CREATE FUNCTION controlled_append_audit_event_v1(
  p_id UUID,
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_event_type TEXT,
  p_target_type TEXT,
  p_target_id TEXT,
  p_outcome TEXT,
  p_correlation_id UUID,
  p_sanitized_metadata JSONB,
  p_occurred_at TIMESTAMPTZ
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
BEGIN
  IF p_id IS NULL OR p_tenant_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_event_type IS NULL OR p_event_type !~ '^[a-z][a-z0-9_.]{2,127}$'
     OR p_target_type IS NULL OR p_target_type !~ '^[a-z][a-z0-9_.]{1,63}$'
     OR p_outcome NOT IN ('allowed','denied','succeeded','failed')
     OR p_sanitized_metadata IS NULL OR jsonb_typeof(p_sanitized_metadata) <> 'object'
     OR octet_length(p_sanitized_metadata::text) > 8192
     OR (p_actor_subject_id IS NOT NULL AND (p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id))
     OR (p_target_id IS NOT NULL AND (p_target_id='' OR length(p_target_id)>256)) THEN
    RAISE EXCEPTION 'controlled audit input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_audit_input';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM public.tenants tenant WHERE tenant.id=p_tenant_id) THEN
    RAISE EXCEPTION 'controlled audit tenant was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_audit_tenant';
  END IF;
  IF EXISTS (
    SELECT 1 FROM jsonb_object_keys(p_sanitized_metadata) key
    WHERE length(key)>64 OR key !~ '^[a-z][a-z0-9_]*$'
       OR key ~ '(secret|token|authorization|cookie|session|csrf|credential|database_url|connection_string|dsn|private_key|api_key|access_key|amount|balance|email|phone|address|ip_address|payload)'
  ) THEN
    RAISE EXCEPTION 'controlled audit metadata is not sanitized'
      USING ERRCODE='22023', CONSTRAINT='controlled_audit_metadata';
  END IF;

  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    p_id,p_tenant_id,NULLIF(p_actor_subject_id,''),p_event_type,p_target_type,
    NULLIF(p_target_id,''),p_outcome,p_correlation_id,p_sanitized_metadata,p_occurred_at
  );
  RETURN TRUE;
END;
$$;

CREATE FUNCTION controlled_update_account_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_account_id UUID,
  p_expected_version BIGINT,
  p_display_name TEXT,
  p_external_reference TEXT,
  p_category TEXT,
  p_status TEXT,
  p_closed_at TIMESTAMPTZ,
  p_new_version BIGINT,
  p_occurred_at TIMESTAMPTZ
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_account RECORD;
  v_authoritative NUMERIC;
  v_unresolved BOOLEAN;
BEGIN
  IF p_tenant_id IS NULL OR p_account_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_expected_version<1 OR p_new_version<>p_expected_version+1
     OR p_display_name IS NULL OR p_display_name='' OR btrim(p_display_name)<>p_display_name
     OR p_external_reference IS NULL OR p_external_reference='' OR btrim(p_external_reference)<>p_external_reference
     OR p_category NOT IN ('customer_funds','expenses','operating','payables','payroll','reserve')
     OR p_status NOT IN ('active','frozen','closed')
     OR (p_status='closed')<>(p_closed_at IS NOT NULL) THEN
    RAISE EXCEPTION 'controlled account update input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_account_update_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_api','MEMBER') THEN
    RAISE EXCEPTION 'controlled account update caller is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_account_update_caller';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.tenant_subject_roles role
    JOIN public.account_owners owner ON owner.tenant_id=role.tenant_id AND owner.subject_id=role.subject_id
    WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id
      AND role.role IN ('operator','finance') AND owner.account_id=p_account_id AND owner.permission='debit'
  ) THEN
    RAISE EXCEPTION 'controlled account actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_account_update_actor';
  END IF;

  SELECT account.status,account.version
    INTO v_account
    FROM public.accounts account
   WHERE account.tenant_id=p_tenant_id AND account.id=p_account_id
   FOR UPDATE;
  IF NOT FOUND OR v_account.version<>p_expected_version THEN
    RETURN FALSE;
  END IF;
  IF v_account.status='closed' AND p_status<>'closed'
     OR (v_account.status,p_status) NOT IN (('active','active'),('active','frozen'),('active','closed'),('frozen','frozen'),('frozen','active'),('frozen','closed'),('closed','closed')) THEN
    RAISE EXCEPTION 'controlled account status transition is invalid'
      USING ERRCODE='23514', CONSTRAINT='controlled_account_update_transition';
  END IF;

  IF p_status='closed' AND v_account.status<>'closed' THEN
    SELECT EXISTS(SELECT 1 FROM public.transfers transfer WHERE transfer.status='pending' AND (transfer.debit_account_id=p_account_id OR transfer.credit_account_id=p_account_id))
        OR EXISTS(SELECT 1 FROM public.funding_events funding WHERE funding.status IN ('requested','approved') AND funding.destination_account_id=p_account_id)
        OR EXISTS(
          SELECT 1 FROM public.transfer_corrections correction
          JOIN public.transfers original ON original.id=correction.original_transfer_id
          WHERE correction.status IN ('requested','approved') AND (original.debit_account_id=p_account_id OR original.credit_account_id=p_account_id)
        )
      INTO v_unresolved;
    SELECT opening.opening_ledger_minor::numeric + COALESCE(SUM(CASE WHEN posting.direction='credit' THEN posting.amount_minor::numeric ELSE -posting.amount_minor::numeric END),0)
      INTO v_authoritative
      FROM public.account_opening_balances opening
      LEFT JOIN public.ledger_postings posting ON posting.account_id=opening.account_id
     WHERE opening.account_id=p_account_id
     GROUP BY opening.opening_ledger_minor;
    IF v_unresolved OR v_authoritative IS NULL OR v_authoritative<>0
       OR EXISTS(SELECT 1 FROM public.account_balance_projections balance WHERE balance.account_id=p_account_id AND (balance.available_minor<>0 OR balance.ledger_minor<>0)) THEN
      RAISE EXCEPTION 'controlled account cannot close with financial obligations'
        USING ERRCODE='23514', CONSTRAINT='controlled_account_update_close';
    END IF;
  END IF;

  UPDATE public.accounts
     SET display_name=p_display_name,external_reference=p_external_reference,
         category=p_category,status=p_status,closed_at=p_closed_at,
         version=p_new_version,updated_at=p_occurred_at
   WHERE tenant_id=p_tenant_id AND id=p_account_id AND version=p_expected_version;
  RETURN FOUND;
END;
$$;

CREATE FUNCTION controlled_ensure_funding_account_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_currency TEXT,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ
)
RETURNS UUID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_account_id UUID;
BEGIN
  IF p_tenant_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_currency IS NULL OR p_currency !~ '^[A-Z]{3}$' THEN
    RAISE EXCEPTION 'controlled funding account input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_funding_account_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_api','MEMBER')
     OR NOT EXISTS (
       SELECT 1 FROM public.tenant_subject_roles role
        WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance'
     ) THEN
    RAISE EXCEPTION 'controlled funding account actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_account_actor';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('funding-account|'||p_tenant_id::text||'|'||p_currency,0));
  SELECT account.id INTO v_account_id
    FROM public.accounts account
   WHERE account.tenant_id=p_tenant_id AND account.currency=p_currency AND account.account_kind='funding_clearing'
   FOR UPDATE;
  IF FOUND THEN
    RETURN v_account_id;
  END IF;

  v_account_id := gen_random_uuid();
  INSERT INTO public.accounts(
    id,tenant_id,currency,status,display_name,category,external_reference,
    account_kind,created_at,updated_at
  ) VALUES(
    v_account_id,p_tenant_id,p_currency,'active','Funding clearing · '||p_currency,
    'system','system-funding-'||lower(p_currency),'funding_clearing',p_occurred_at,p_occurred_at
  );
  INSERT INTO public.account_balance_projections(account_id,available_minor,ledger_minor,balance_version,allow_negative,updated_at)
  VALUES(v_account_id,0,0,0,TRUE,p_occurred_at);
  INSERT INTO public.account_opening_balances(account_id,opening_ledger_minor,created_at)
  VALUES(v_account_id,0,p_occurred_at);
  RETURN v_account_id;
END;
$$;

CREATE FUNCTION controlled_request_funding_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_destination_account_id UUID,
  p_amount_minor BIGINT,
  p_currency TEXT,
  p_external_reference TEXT,
  p_evidence_reference TEXT,
  p_idempotency_key TEXT,
  p_request_fingerprint BYTEA,
  p_correlation_id UUID,
  p_requested_at TIMESTAMPTZ
)
RETURNS TABLE(funding_event_id UUID, replayed BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_existing RECORD;
  v_policy RECORD;
  v_system_account_id UUID;
  v_event_id UUID := gen_random_uuid();
BEGIN
  IF p_tenant_id IS NULL OR p_destination_account_id IS NULL OR p_amount_minor<=0
     OR p_currency IS NULL OR p_currency!~'^[A-Z]{3}$'
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_external_reference IS NULL OR p_external_reference='' OR btrim(p_external_reference)<>p_external_reference
     OR p_evidence_reference IS NULL OR p_evidence_reference='' OR btrim(p_evidence_reference)<>p_evidence_reference
     OR p_idempotency_key IS NULL OR length(p_idempotency_key)<16 OR length(p_idempotency_key)>255
     OR p_request_fingerprint IS NULL OR octet_length(p_request_fingerprint)<>32
     OR p_correlation_id IS NULL OR p_requested_at IS NULL THEN
    RAISE EXCEPTION 'controlled funding request input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_funding_request_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_api','MEMBER')
     OR NOT EXISTS (SELECT 1 FROM public.tenant_subject_roles role WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance') THEN
    RAISE EXCEPTION 'controlled funding requester is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_request_actor';
  END IF;

  SELECT event.id,event.request_fingerprint INTO v_existing
    FROM public.funding_events event
   WHERE event.tenant_id=p_tenant_id AND event.requester_subject_id=p_actor_subject_id
     AND event.idempotency_key=p_idempotency_key
   FOR UPDATE;
  IF FOUND THEN
    IF v_existing.request_fingerprint<>p_request_fingerprint THEN
      RAISE EXCEPTION 'controlled funding idempotency conflict'
        USING ERRCODE='23505', CONSTRAINT='controlled_funding_request_idempotency';
    END IF;
    RETURN QUERY SELECT v_existing.id,TRUE;
    RETURN;
  END IF;

  SELECT policy.mode,policy.finance_activated,policy.policy_version,policy.per_command_minor
    INTO v_policy
    FROM public.accounts account
    JOIN public.tenant_funding_policies policy ON policy.tenant_id=account.tenant_id AND policy.currency=account.currency
   WHERE account.tenant_id=p_tenant_id AND account.id=p_destination_account_id
     AND account.currency=p_currency AND account.status='active' AND account.account_kind='customer';
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled funding destination was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_funding_request_not_found';
  END IF;
  IF (v_policy.mode='production_dual_control' AND NOT v_policy.finance_activated) THEN
    RAISE EXCEPTION 'controlled funding policy is not active'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_request_forbidden';
  END IF;
  IF p_amount_minor>v_policy.per_command_minor THEN
    RAISE EXCEPTION 'controlled funding amount exceeds policy'
      USING ERRCODE='23514', CONSTRAINT='controlled_funding_request_limit';
  END IF;

  v_system_account_id := public.controlled_ensure_funding_account_v1(
    p_tenant_id,p_actor_subject_id,p_currency,p_correlation_id,p_requested_at
  );
  INSERT INTO public.funding_events(
    id,tenant_id,requester_subject_id,destination_account_id,system_account_id,
    external_reference,evidence_reference,idempotency_key,request_fingerprint,
    amount_minor,currency,status,demo_policy,policy_version,correlation_id,requested_at,updated_at
  ) VALUES(
    v_event_id,p_tenant_id,p_actor_subject_id,p_destination_account_id,v_system_account_id,
    p_external_reference,p_evidence_reference,p_idempotency_key,p_request_fingerprint,
    p_amount_minor,p_currency,'requested',FALSE,v_policy.policy_version,p_correlation_id,p_requested_at,p_requested_at
  );
  INSERT INTO public.approval_records(
    id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,
    correlation_id,policy_version,created_at
  ) VALUES(
    gen_random_uuid(),p_tenant_id,'funding',v_event_id,p_actor_subject_id,'requested',
    p_requested_at+interval '24 hours',p_correlation_id,v_policy.policy_version,p_requested_at
  );
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    gen_random_uuid(),p_tenant_id,p_actor_subject_id,'funding.requested','funding_event',
    v_event_id::text,'succeeded',p_correlation_id,
    jsonb_build_object('funding_event_id',v_event_id::text,'terminology','recorded external value evidence'),p_requested_at
  );
  RETURN QUERY SELECT v_event_id,FALSE;
END;
$$;

CREATE FUNCTION controlled_request_funding_compensation_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_original_funding_event_id UUID,
  p_reason_code TEXT,
  p_operator_note TEXT,
  p_idempotency_key TEXT,
  p_request_fingerprint BYTEA,
  p_correlation_id UUID,
  p_requested_at TIMESTAMPTZ
)
RETURNS TABLE(funding_event_id UUID, replayed BOOLEAN)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_existing RECORD;
  v_original RECORD;
  v_policy RECORD;
  v_event_id UUID := gen_random_uuid();
BEGIN
  IF p_tenant_id IS NULL OR p_original_funding_event_id IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_reason_code IS NULL OR p_reason_code='' OR length(p_reason_code)>64
     OR p_operator_note IS NULL OR p_operator_note='' OR length(p_operator_note)>500
     OR p_idempotency_key IS NULL OR length(p_idempotency_key)<16 OR length(p_idempotency_key)>255
     OR p_request_fingerprint IS NULL OR octet_length(p_request_fingerprint)<>32
     OR p_correlation_id IS NULL OR p_requested_at IS NULL THEN
    RAISE EXCEPTION 'controlled funding compensation input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_funding_compensation_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_api','MEMBER')
     OR NOT EXISTS (SELECT 1 FROM public.tenant_subject_roles role WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance') THEN
    RAISE EXCEPTION 'controlled funding compensation requester is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_compensation_actor';
  END IF;

  SELECT event.id,event.request_fingerprint INTO v_existing
    FROM public.funding_events event
   WHERE event.tenant_id=p_tenant_id AND event.requester_subject_id=p_actor_subject_id
     AND event.idempotency_key=p_idempotency_key
   FOR UPDATE;
  IF FOUND THEN
    IF v_existing.request_fingerprint<>p_request_fingerprint THEN
      RAISE EXCEPTION 'controlled funding compensation idempotency conflict'
        USING ERRCODE='23505', CONSTRAINT='controlled_funding_compensation_idempotency';
    END IF;
    RETURN QUERY SELECT v_existing.id,TRUE;
    RETURN;
  END IF;

  SELECT event.destination_account_id,event.system_account_id,event.currency,event.amount_minor,
         event.evidence_reference,event.status,event.compensation_event_id,event.policy_version
    INTO v_original
    FROM public.funding_events event
   WHERE event.tenant_id=p_tenant_id AND event.id=p_original_funding_event_id
     AND event.compensation_of_event_id IS NULL
   FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled original funding event was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_funding_compensation_not_found';
  END IF;
  IF v_original.status<>'posted' OR v_original.compensation_event_id IS NOT NULL THEN
    RAISE EXCEPTION 'controlled funding event cannot be compensated'
      USING ERRCODE='23514', CONSTRAINT='controlled_funding_compensation_conflict';
  END IF;
  SELECT policy.mode,policy.finance_activated INTO v_policy
    FROM public.tenant_funding_policies policy
   WHERE policy.tenant_id=p_tenant_id AND policy.currency=v_original.currency;
  IF NOT FOUND OR (v_policy.mode='production_dual_control' AND NOT v_policy.finance_activated) THEN
    RAISE EXCEPTION 'controlled funding compensation policy is not active'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_compensation_forbidden';
  END IF;

  INSERT INTO public.funding_events(
    id,tenant_id,requester_subject_id,destination_account_id,system_account_id,
    external_reference,evidence_reference,idempotency_key,request_fingerprint,
    amount_minor,currency,status,demo_policy,policy_version,compensation_of_event_id,
    compensation_reason_code,compensation_operator_note,correlation_id,requested_at,updated_at
  ) VALUES(
    v_event_id,p_tenant_id,p_actor_subject_id,v_original.destination_account_id,v_original.system_account_id,
    'compensation:'||p_original_funding_event_id::text,v_original.evidence_reference,p_idempotency_key,p_request_fingerprint,
    v_original.amount_minor,v_original.currency,'requested',FALSE,v_original.policy_version,p_original_funding_event_id,
    p_reason_code,p_operator_note,p_correlation_id,p_requested_at,p_requested_at
  );
  INSERT INTO public.approval_records(
    id,tenant_id,command_type,target_id,requester_subject_id,status,expires_at,
    correlation_id,policy_version,created_at
  ) VALUES(
    gen_random_uuid(),p_tenant_id,'funding_compensation',v_event_id,p_actor_subject_id,'requested',
    p_requested_at+interval '24 hours',p_correlation_id,v_original.policy_version,p_requested_at
  );
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    gen_random_uuid(),p_tenant_id,p_actor_subject_id,'funding.compensation.requested','funding_event',
    v_event_id::text,'succeeded',p_correlation_id,
    jsonb_build_object('funding_event_id',v_event_id::text,'terminology','recorded external value evidence'),p_requested_at
  );
  RETURN QUERY SELECT v_event_id,FALSE;
END;
$$;

CREATE FUNCTION controlled_decide_funding_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_funding_event_id UUID,
  p_action TEXT,
  p_reason TEXT,
  p_correlation_id UUID,
  p_decided_at TIMESTAMPTZ
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  v_event RECORD;
  v_policy RECORD;
BEGIN
  IF p_tenant_id IS NULL OR p_funding_event_id IS NULL OR p_action NOT IN ('approve','reject')
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id
     OR p_reason IS NULL OR p_reason='' OR length(p_reason)>500
     OR p_correlation_id IS NULL OR p_decided_at IS NULL THEN
    RAISE EXCEPTION 'controlled funding decision input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_funding_decision_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_api','MEMBER')
     OR NOT EXISTS (SELECT 1 FROM public.tenant_subject_roles role WHERE role.tenant_id=p_tenant_id AND role.subject_id=p_actor_subject_id AND role.role='finance') THEN
    RAISE EXCEPTION 'controlled funding decision actor is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_decision_actor';
  END IF;
  SELECT event.requester_subject_id,event.status,event.compensation_of_event_id,event.policy_version,event.currency
    INTO v_event FROM public.funding_events event
   WHERE event.tenant_id=p_tenant_id AND event.id=p_funding_event_id FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'controlled funding event was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_funding_decision_not_found';
  END IF;
  SELECT policy.mode,policy.finance_activated INTO v_policy
    FROM public.tenant_funding_policies policy
   WHERE policy.tenant_id=p_tenant_id AND policy.currency=v_event.currency;
  IF p_action='approve' AND v_event.status='approved' THEN
    RETURN TRUE;
  END IF;
  IF v_event.status<>'requested' THEN
    RAISE EXCEPTION 'controlled funding event is not awaiting a decision'
      USING ERRCODE='23514', CONSTRAINT='controlled_funding_decision_conflict';
  END IF;
  IF p_action='approve' AND (v_policy.mode='production_dual_control' AND (NOT v_policy.finance_activated OR v_event.requester_subject_id=p_actor_subject_id)) THEN
    RAISE EXCEPTION 'controlled funding approval violates dual control'
      USING ERRCODE='42501', CONSTRAINT='controlled_funding_decision_forbidden';
  END IF;

  IF p_action='approve' THEN
    UPDATE public.funding_events
       SET status='approved',approver_subject_id=p_actor_subject_id,decision_reason=p_reason,
           demo_policy=(v_policy.mode='local_demo_single_operator'),approved_at=p_decided_at,updated_at=p_decided_at
     WHERE tenant_id=p_tenant_id AND id=p_funding_event_id AND status='requested';
  ELSE
    UPDATE public.funding_events
       SET status='rejected',approver_subject_id=p_actor_subject_id,decision_reason=p_reason,
           rejected_at=p_decided_at,updated_at=p_decided_at
     WHERE tenant_id=p_tenant_id AND id=p_funding_event_id AND status='requested';
  END IF;
  INSERT INTO public.approval_records(
    id,tenant_id,command_type,target_id,requester_subject_id,approver_subject_id,
    status,expires_at,decision_reason,correlation_id,policy_version,created_at,decided_at
  ) VALUES(
    gen_random_uuid(),p_tenant_id,CASE WHEN v_event.compensation_of_event_id IS NULL THEN 'funding' ELSE 'funding_compensation' END,
    p_funding_event_id,v_event.requester_subject_id,p_actor_subject_id,
    CASE WHEN p_action='approve' THEN 'approved' ELSE 'rejected' END,
    CASE WHEN p_action='approve' THEN p_decided_at+interval '24 hours' ELSE p_decided_at END,
    p_reason,p_correlation_id,v_event.policy_version,p_decided_at,p_decided_at
  );
  INSERT INTO public.audit_events(
    id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,
    correlation_id,sanitized_metadata,occurred_at
  ) VALUES(
    gen_random_uuid(),p_tenant_id,p_actor_subject_id,
    CASE WHEN p_action='approve' THEN 'funding.approved' ELSE 'funding.rejected' END,
    'funding_event',p_funding_event_id::text,'succeeded',p_correlation_id,
    jsonb_build_object('funding_event_id',p_funding_event_id::text,'terminology','recorded external value evidence'),p_decided_at
  );
  RETURN FALSE;
END;
$$;

CREATE FUNCTION controlled_rollback_provisioned_tenant_v1(
  p_tenant_id UUID,
  p_actor_subject_id TEXT,
  p_correlation_id UUID,
  p_occurred_at TIMESTAMPTZ
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
BEGIN
  IF p_tenant_id IS NULL OR p_correlation_id IS NULL OR p_occurred_at IS NULL
     OR p_actor_subject_id IS NULL OR p_actor_subject_id='' OR btrim(p_actor_subject_id)<>p_actor_subject_id THEN
    RAISE EXCEPTION 'controlled provisioning rollback input is invalid'
      USING ERRCODE='22023', CONSTRAINT='controlled_provisioning_rollback_input';
  END IF;
  IF NOT pg_has_role(session_user,'ledgersync_provisioning','MEMBER') THEN
    RAISE EXCEPTION 'controlled provisioning rollback caller is not authorized'
      USING ERRCODE='42501', CONSTRAINT='controlled_provisioning_rollback_caller';
  END IF;
  PERFORM pg_advisory_xact_lock(hashtextextended('provisioning-rollback|'||p_tenant_id::text,0));
  IF NOT EXISTS (
    SELECT 1 FROM public.partner_provisioning_requests request
    WHERE request.tenant_id=p_tenant_id AND request.status='applied'
  ) THEN
    RAISE EXCEPTION 'controlled provisioning request was not found'
      USING ERRCODE='P0002', CONSTRAINT='controlled_provisioning_rollback_not_found';
  END IF;
  IF EXISTS(SELECT 1 FROM public.transfers transfer WHERE transfer.tenant_id=p_tenant_id)
     OR EXISTS(SELECT 1 FROM public.funding_events funding WHERE funding.tenant_id=p_tenant_id AND funding.status IN ('posted','compensated'))
     OR EXISTS(SELECT 1 FROM public.opening_import_executions execution WHERE execution.tenant_id=p_tenant_id) THEN
    RAISE EXCEPTION 'tenant with financial history cannot be rolled back'
      USING ERRCODE='23514', CONSTRAINT='controlled_provisioning_rollback_history';
  END IF;

  DELETE FROM public.account_credit_permissions WHERE tenant_id=p_tenant_id;
  DELETE FROM public.account_owners WHERE tenant_id=p_tenant_id;
  UPDATE public.accounts SET status='closed',closed_at=p_occurred_at,updated_at=p_occurred_at
   WHERE tenant_id=p_tenant_id AND status<>'closed';
  DELETE FROM public.tenant_subject_roles WHERE tenant_id=p_tenant_id;
  RETURN TRUE;
END;
$$;

COMMENT ON FUNCTION controlled_append_audit_event_v1(UUID,UUID,TEXT,TEXT,TEXT,TEXT,TEXT,UUID,JSONB,TIMESTAMPTZ)
  IS 'Append-only, shape-validated audit evidence capability';
COMMENT ON FUNCTION controlled_update_account_v1(UUID,TEXT,UUID,BIGINT,TEXT,TEXT,TEXT,TEXT,TIMESTAMPTZ,BIGINT,TIMESTAMPTZ)
  IS 'Tenant- and ownership-validated account lifecycle capability; never changes balances';
COMMENT ON FUNCTION controlled_ensure_funding_account_v1(UUID,TEXT,TEXT,UUID,TIMESTAMPTZ)
  IS 'Finance-authorized zero-opening funding-clearing account capability';
COMMENT ON FUNCTION controlled_request_funding_v1(UUID,TEXT,UUID,BIGINT,TEXT,TEXT,TEXT,TEXT,BYTEA,UUID,TIMESTAMPTZ)
  IS 'Finance-authorized, idempotent funding request and approval-evidence capability';
COMMENT ON FUNCTION controlled_request_funding_compensation_v1(UUID,TEXT,UUID,TEXT,TEXT,TEXT,BYTEA,UUID,TIMESTAMPTZ)
  IS 'Finance-authorized exact funding-compensation request capability';
COMMENT ON FUNCTION controlled_decide_funding_v1(UUID,TEXT,UUID,TEXT,TEXT,UUID,TIMESTAMPTZ)
  IS 'Finance-authorized funding approval or rejection capability with dual control';
COMMENT ON FUNCTION controlled_rollback_provisioned_tenant_v1(UUID,TEXT,UUID,TIMESTAMPTZ)
  IS 'Provisioning-only rollback capability that refuses tenants with financial history';

REVOKE ALL ON FUNCTION controlled_append_audit_event_v1(UUID,UUID,TEXT,TEXT,TEXT,TEXT,TEXT,UUID,JSONB,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_update_account_v1(UUID,TEXT,UUID,BIGINT,TEXT,TEXT,TEXT,TEXT,TIMESTAMPTZ,BIGINT,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_ensure_funding_account_v1(UUID,TEXT,TEXT,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_request_funding_v1(UUID,TEXT,UUID,BIGINT,TEXT,TEXT,TEXT,TEXT,BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_request_funding_compensation_v1(UUID,TEXT,UUID,TEXT,TEXT,TEXT,BYTEA,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_decide_funding_v1(UUID,TEXT,UUID,TEXT,TEXT,UUID,TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION controlled_rollback_provisioned_tenant_v1(UUID,TEXT,UUID,TIMESTAMPTZ) FROM PUBLIC;

REVOKE INSERT,UPDATE,DELETE ON
  accounts,account_opening_balances,account_balance_projections,
  transfers,transfer_velocity_events,transfer_velocity_totals,funding_events,funding_velocity_events,
  journal_transactions,ledger_postings,account_owners,account_credit_permissions,
  audit_events,opening_import_batches,opening_import_rows,
  opening_import_approvals,opening_import_executions
FROM PUBLIC;

DO $$
DECLARE
  v_role TEXT;
BEGIN
  FOREACH v_role IN ARRAY ARRAY[
    'ledgersync_api','ledgersync_worker','ledgersync_reconciliation',
    'ledgersync_provisioning','ledgersync_support_readonly','ledgersync_break_glass'
  ] LOOP
    IF to_regrole(v_role) IS NOT NULL THEN
      EXECUTE format(
        'REVOKE INSERT,UPDATE,DELETE ON accounts,account_opening_balances,account_balance_projections,transfers,transfer_velocity_events,transfer_velocity_totals,funding_events,funding_velocity_events,journal_transactions,ledger_postings,account_owners,account_credit_permissions,audit_events,opening_import_batches,opening_import_rows,opening_import_approvals,opening_import_executions FROM %I',
        v_role
      );
      EXECUTE format('REVOKE ALL ON FUNCTION reject_ledger_mutation() FROM %I',v_role);
    END IF;
  END LOOP;
END;
$$;
