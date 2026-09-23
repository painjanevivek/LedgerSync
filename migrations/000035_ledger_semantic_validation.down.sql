DO $$
DECLARE
  rollback_reason TEXT := current_setting('ledgersync.semantic_validation_rollback_reason', true);
BEGIN
  IF rollback_reason IS NULL OR char_length(rollback_reason) < 16 THEN
    RAISE EXCEPTION 'semantic validation rollback requires an incident-approved reason'
      USING ERRCODE='55000';
  END IF;
  INSERT INTO ledger_semantic_control_events(action,actor,reason)
  VALUES('semantic_validation_disabled',session_user,rollback_reason);
END;
$$;

DROP TRIGGER funding_events_semantic_shape ON funding_events;
DROP TRIGGER transfers_semantic_shape ON transfers;
DROP TRIGGER ledger_postings_semantic_shape ON ledger_postings;
DROP TRIGGER journal_transactions_semantic_shape ON journal_transactions;
DROP FUNCTION enforce_ledger_semantic_shape();
DROP FUNCTION validate_ledger_semantic_shape(UUID);

CREATE FUNCTION enforce_journal_balance()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_journal_id UUID;
BEGIN
  IF TG_TABLE_NAME = 'ledger_postings' THEN
    target_journal_id := NEW.journal_transaction_id;
  ELSE
    target_journal_id := NEW.id;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM ledger_postings WHERE journal_transaction_id = target_journal_id
  ) OR EXISTS (
    SELECT 1
    FROM ledger_postings
    WHERE journal_transaction_id = target_journal_id
    GROUP BY journal_transaction_id, currency
    HAVING count(*) < 2
      OR sum(CASE WHEN direction = 'debit' THEN amount_minor ELSE 0 END)
       <> sum(CASE WHEN direction = 'credit' THEN amount_minor ELSE 0 END)
  ) THEN
    RAISE EXCEPTION 'journal transaction is not balanced' USING ERRCODE = '23514';
  END IF;
  RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER ledger_postings_balanced
AFTER INSERT ON ledger_postings
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_journal_balance();
CREATE CONSTRAINT TRIGGER journal_transactions_require_postings
AFTER INSERT ON journal_transactions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION enforce_journal_balance();

ALTER TABLE ledger_postings
  ALTER COLUMN tenant_id DROP NOT NULL,
  DROP CONSTRAINT ledger_posting_tenant_present_check,
  DROP CONSTRAINT ledger_posting_account_tenant_fk,
  DROP CONSTRAINT ledger_posting_journal_tenant_fk;
ALTER TABLE ledger_postings
  ADD CONSTRAINT ledger_posting_journal_tenant_fk
    FOREIGN KEY (journal_transaction_id,tenant_id)
    REFERENCES journal_transactions(id,tenant_id) NOT VALID,
  ADD CONSTRAINT ledger_posting_account_tenant_fk
    FOREIGN KEY (account_id,tenant_id)
    REFERENCES accounts(id,tenant_id) NOT VALID;

ALTER TABLE journal_transactions
  ALTER COLUMN source_type DROP NOT NULL,
  ALTER COLUMN source_id DROP NOT NULL,
  DROP CONSTRAINT journal_source_identity_key,
  DROP CONSTRAINT journal_source_matches_command_check,
  DROP CONSTRAINT journal_source_present_check,
  DROP CONSTRAINT journal_source_pair_expand_check,
  DROP CONSTRAINT journal_source_type_expand_check,
  DROP CONSTRAINT journal_funding_tenant_fk,
  DROP CONSTRAINT journal_transfer_tenant_fk;
ALTER TABLE journal_transactions
  ADD CONSTRAINT journal_transfer_tenant_fk
    FOREIGN KEY (transfer_id,tenant_id) REFERENCES transfers(id,tenant_id) NOT VALID,
  ADD CONSTRAINT journal_funding_tenant_fk
    FOREIGN KEY (funding_event_id,tenant_id) REFERENCES funding_events(id,tenant_id) NOT VALID,
  ADD CONSTRAINT journal_source_type_expand_check
    CHECK (source_type IS NULL OR source_type IN ('transfer','funding_event')) NOT VALID,
  ADD CONSTRAINT journal_source_pair_expand_check
    CHECK ((source_type IS NULL) = (source_id IS NULL)) NOT VALID;
