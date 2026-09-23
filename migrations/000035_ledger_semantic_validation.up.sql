SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

DO $$
DECLARE
  mismatch_count BIGINT;
BEGIN
  SELECT unbackfilled_journal_count + unbackfilled_posting_count + journal_source_mismatch_count
       + transfer_tenant_mismatch_count + funding_tenant_mismatch_count
       + posting_journal_tenant_mismatch_count + posting_account_tenant_mismatch_count
       + duplicate_journal_source_count
    INTO mismatch_count
    FROM ledger_semantic_key_validation;
  IF mismatch_count <> 0 THEN
    RAISE EXCEPTION 'ledger semantic-key validation is not clean'
      USING ERRCODE='23514', CONSTRAINT='ledger_semantic_key_readiness';
  END IF;
END;
$$;

ALTER TABLE journal_transactions
  VALIDATE CONSTRAINT journal_transfer_tenant_fk,
  VALIDATE CONSTRAINT journal_funding_tenant_fk,
  VALIDATE CONSTRAINT journal_source_type_expand_check,
  VALIDATE CONSTRAINT journal_source_pair_expand_check;

ALTER TABLE ledger_postings
  VALIDATE CONSTRAINT ledger_posting_journal_tenant_fk,
  VALIDATE CONSTRAINT ledger_posting_account_tenant_fk;

ALTER TABLE journal_transactions
  ADD CONSTRAINT journal_source_present_check
    CHECK (source_type IS NOT NULL AND source_id IS NOT NULL) NOT VALID,
  ADD CONSTRAINT journal_source_matches_command_check
    CHECK (
      (source_type='transfer' AND source_id=transfer_id AND funding_event_id IS NULL)
      OR
      (source_type='funding_event' AND source_id=funding_event_id AND transfer_id IS NULL)
    ) NOT VALID;
ALTER TABLE journal_transactions
  VALIDATE CONSTRAINT journal_source_present_check,
  VALIDATE CONSTRAINT journal_source_matches_command_check;
ALTER TABLE journal_transactions
  ALTER COLUMN source_type SET NOT NULL,
  ALTER COLUMN source_id SET NOT NULL,
  ADD CONSTRAINT journal_source_identity_key UNIQUE (tenant_id,source_type,source_id);

ALTER TABLE ledger_postings
  ADD CONSTRAINT ledger_posting_tenant_present_check CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE ledger_postings VALIDATE CONSTRAINT ledger_posting_tenant_present_check;
ALTER TABLE ledger_postings ALTER COLUMN tenant_id SET NOT NULL;

CREATE TABLE ledger_semantic_control_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  action TEXT NOT NULL CHECK (action IN ('semantic_validation_disabled')),
  actor TEXT NOT NULL,
  reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 16 AND 500),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON ledger_semantic_control_events FROM PUBLIC;

CREATE FUNCTION validate_ledger_semantic_shape(target_journal_id UUID)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  journal RECORD;
  command RECORD;
  original RECORD;
  posting_count INTEGER;
  debit_count INTEGER;
  credit_count INTEGER;
BEGIN
  SELECT id,tenant_id,source_type,source_id,transfer_id,funding_event_id
    INTO journal
    FROM public.journal_transactions
   WHERE id=target_journal_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'ledger semantic constraint violated: journal missing'
      USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
  END IF;

  IF journal.source_type='transfer' THEN
    SELECT tenant_id,debit_account_id,credit_account_id,amount_minor,currency,status,
           journal_transaction_id,compensation_of_transfer_id
      INTO command
      FROM public.transfers
     WHERE id=journal.source_id AND tenant_id=journal.tenant_id;
    IF NOT FOUND OR command.status<>'posted' OR command.journal_transaction_id IS DISTINCT FROM journal.id
       OR journal.transfer_id IS DISTINCT FROM journal.source_id OR journal.funding_event_id IS NOT NULL THEN
      RAISE EXCEPTION 'ledger semantic constraint violated: transfer linkage'
        USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
    END IF;

    IF command.compensation_of_transfer_id IS NOT NULL THEN
      SELECT tenant_id,debit_account_id,credit_account_id,amount_minor,currency,status
        INTO original
        FROM public.transfers
       WHERE id=command.compensation_of_transfer_id AND tenant_id=journal.tenant_id;
      IF NOT FOUND OR original.status<>'posted'
         OR command.debit_account_id<>original.credit_account_id
         OR command.credit_account_id<>original.debit_account_id
         OR command.amount_minor<>original.amount_minor OR command.currency<>original.currency THEN
        RAISE EXCEPTION 'ledger semantic constraint violated: transfer compensation'
          USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
      END IF;
    END IF;

    SELECT count(*),
           count(*) FILTER (WHERE direction='debit' AND account_id=command.debit_account_id
                            AND amount_minor=command.amount_minor AND currency=command.currency),
           count(*) FILTER (WHERE direction='credit' AND account_id=command.credit_account_id
                            AND amount_minor=command.amount_minor AND currency=command.currency)
      INTO posting_count,debit_count,credit_count
      FROM public.ledger_postings
     WHERE journal_transaction_id=journal.id AND tenant_id=journal.tenant_id;
    IF posting_count<>2 OR debit_count<>1 OR credit_count<>1 THEN
      RAISE EXCEPTION 'ledger semantic constraint violated: transfer postings'
        USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
    END IF;

  ELSIF journal.source_type='funding_event' THEN
    SELECT tenant_id,destination_account_id,system_account_id,amount_minor,currency,status,
           journal_transaction_id,compensation_of_event_id
      INTO command
      FROM public.funding_events
     WHERE id=journal.source_id AND tenant_id=journal.tenant_id;
    IF NOT FOUND OR command.status NOT IN ('posted','compensated')
       OR command.journal_transaction_id IS DISTINCT FROM journal.id
       OR journal.funding_event_id IS DISTINCT FROM journal.source_id OR journal.transfer_id IS NOT NULL THEN
      RAISE EXCEPTION 'ledger semantic constraint violated: funding linkage'
        USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
    END IF;

    IF command.compensation_of_event_id IS NULL THEN
      SELECT count(*),
             count(*) FILTER (WHERE direction='debit' AND account_id=command.system_account_id
                              AND amount_minor=command.amount_minor AND currency=command.currency),
             count(*) FILTER (WHERE direction='credit' AND account_id=command.destination_account_id
                              AND amount_minor=command.amount_minor AND currency=command.currency)
        INTO posting_count,debit_count,credit_count
        FROM public.ledger_postings
       WHERE journal_transaction_id=journal.id AND tenant_id=journal.tenant_id;
    ELSE
      SELECT tenant_id,destination_account_id,system_account_id,amount_minor,currency,status,compensation_event_id
        INTO original
        FROM public.funding_events
       WHERE id=command.compensation_of_event_id AND tenant_id=journal.tenant_id;
      IF NOT FOUND OR original.status<>'compensated'
         OR original.compensation_event_id IS DISTINCT FROM journal.source_id
         OR command.destination_account_id<>original.destination_account_id
         OR command.system_account_id<>original.system_account_id
         OR command.amount_minor<>original.amount_minor OR command.currency<>original.currency THEN
        RAISE EXCEPTION 'ledger semantic constraint violated: funding compensation'
          USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
      END IF;
      SELECT count(*),
             count(*) FILTER (WHERE direction='debit' AND account_id=command.destination_account_id
                              AND amount_minor=command.amount_minor AND currency=command.currency),
             count(*) FILTER (WHERE direction='credit' AND account_id=command.system_account_id
                              AND amount_minor=command.amount_minor AND currency=command.currency)
        INTO posting_count,debit_count,credit_count
        FROM public.ledger_postings
       WHERE journal_transaction_id=journal.id AND tenant_id=journal.tenant_id;
    END IF;
    IF posting_count<>2 OR debit_count<>1 OR credit_count<>1 THEN
      RAISE EXCEPTION 'ledger semantic constraint violated: funding postings'
        USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
    END IF;
  ELSE
    RAISE EXCEPTION 'ledger semantic constraint violated: source type'
      USING ERRCODE='23514', CONSTRAINT='ledger_semantic_validation';
  END IF;
END;
$$;

CREATE FUNCTION enforce_ledger_semantic_shape()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  target_journal_id UUID;
  previous_journal_id UUID;
BEGIN
  CASE TG_TABLE_NAME
    WHEN 'journal_transactions' THEN
      IF TG_OP='DELETE' THEN
        target_journal_id := OLD.id;
      ELSE
        target_journal_id := NEW.id;
        IF TG_OP='UPDATE' THEN previous_journal_id := OLD.id; END IF;
      END IF;
    WHEN 'ledger_postings' THEN
      IF TG_OP='DELETE' THEN
        target_journal_id := OLD.journal_transaction_id;
      ELSE
        target_journal_id := NEW.journal_transaction_id;
        IF TG_OP='UPDATE' THEN previous_journal_id := OLD.journal_transaction_id; END IF;
      END IF;
    WHEN 'transfers' THEN
      IF TG_OP='DELETE' THEN
        IF OLD.status<>'posted' THEN RETURN NULL; END IF;
        target_journal_id := OLD.journal_transaction_id;
      ELSE
        IF NEW.status='posted' THEN target_journal_id := NEW.journal_transaction_id; END IF;
        IF TG_OP='UPDATE' AND OLD.status='posted' THEN previous_journal_id := OLD.journal_transaction_id; END IF;
      END IF;
    WHEN 'funding_events' THEN
      IF TG_OP='DELETE' THEN
        IF OLD.status NOT IN ('posted','compensated') THEN RETURN NULL; END IF;
        target_journal_id := OLD.journal_transaction_id;
      ELSE
        IF NEW.status IN ('posted','compensated') THEN target_journal_id := NEW.journal_transaction_id; END IF;
        IF TG_OP='UPDATE' AND OLD.status IN ('posted','compensated') THEN previous_journal_id := OLD.journal_transaction_id; END IF;
      END IF;
    ELSE
      RAISE EXCEPTION 'ledger semantic constraint trigger misconfigured'
        USING ERRCODE='55000', CONSTRAINT='ledger_semantic_validation';
  END CASE;
  IF previous_journal_id IS NOT NULL AND previous_journal_id IS DISTINCT FROM target_journal_id THEN
    PERFORM public.validate_ledger_semantic_shape(previous_journal_id);
  END IF;
  IF target_journal_id IS NOT NULL THEN
    PERFORM public.validate_ledger_semantic_shape(target_journal_id);
  END IF;
  RETURN NULL;
END;
$$;

REVOKE ALL ON FUNCTION validate_ledger_semantic_shape(UUID) FROM PUBLIC;
REVOKE ALL ON FUNCTION enforce_ledger_semantic_shape() FROM PUBLIC;

-- Validate every previously committed journal before enforcement begins. This
-- emits only the generic constraint diagnostic and keeps invalid legacy state
-- from being grandfathered into the protected schema.
DO $$
DECLARE
  existing_journal_id UUID;
BEGIN
  FOR existing_journal_id IN SELECT id FROM public.journal_transactions ORDER BY id LOOP
    PERFORM public.validate_ledger_semantic_shape(existing_journal_id);
  END LOOP;
END;
$$;

CREATE CONSTRAINT TRIGGER journal_transactions_semantic_shape
AFTER INSERT OR UPDATE OR DELETE ON journal_transactions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_ledger_semantic_shape();

CREATE CONSTRAINT TRIGGER ledger_postings_semantic_shape
AFTER INSERT OR UPDATE OR DELETE ON ledger_postings
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_ledger_semantic_shape();

CREATE CONSTRAINT TRIGGER transfers_semantic_shape
AFTER INSERT OR UPDATE OR DELETE ON transfers
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_ledger_semantic_shape();

CREATE CONSTRAINT TRIGGER funding_events_semantic_shape
AFTER INSERT OR UPDATE OR DELETE ON funding_events
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_ledger_semantic_shape();

DROP TRIGGER ledger_postings_balanced ON ledger_postings;
DROP TRIGGER journal_transactions_require_postings ON journal_transactions;
DROP FUNCTION enforce_journal_balance();
