-- This rollback is safe only while every expanded fact is reconstructable from
-- the legacy command and account relationships. Refuse to discard evidence
-- written by a future application that cannot be represented by the old shape.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM journal_transactions
    WHERE source_type IS DISTINCT FROM CASE
      WHEN transfer_id IS NOT NULL THEN 'transfer'
      WHEN funding_event_id IS NOT NULL THEN 'funding_event'
      ELSE NULL
    END
       OR source_id IS DISTINCT FROM COALESCE(transfer_id, funding_event_id)
  ) OR EXISTS (
    SELECT 1
    FROM ledger_postings AS posting
    LEFT JOIN journal_transactions AS journal ON journal.id = posting.journal_transaction_id
    WHERE posting.tenant_id IS DISTINCT FROM journal.tenant_id
  ) THEN
    RAISE EXCEPTION 'expanded ledger evidence is not representable by the legacy schema'
      USING ERRCODE = '55000';
  END IF;
END;
$$;

DROP VIEW ledger_semantic_key_validation;

ALTER TABLE ledger_postings
  DROP CONSTRAINT ledger_posting_account_tenant_fk,
  DROP CONSTRAINT ledger_posting_journal_tenant_fk,
  DROP CONSTRAINT ledger_postings_id_tenant_key;

ALTER TABLE journal_transactions
  DROP CONSTRAINT journal_source_pair_expand_check,
  DROP CONSTRAINT journal_source_type_expand_check,
  DROP CONSTRAINT journal_funding_tenant_fk,
  DROP CONSTRAINT journal_transfer_tenant_fk,
  DROP CONSTRAINT journal_transactions_id_tenant_key;

DROP TRIGGER ledger_postings_hydrate_tenant_key ON ledger_postings;
DROP TRIGGER journal_transactions_hydrate_semantic_keys ON journal_transactions;
DROP FUNCTION hydrate_posting_tenant_key();
DROP FUNCTION hydrate_journal_semantic_keys();

ALTER TABLE ledger_postings DROP COLUMN tenant_id;
ALTER TABLE journal_transactions DROP COLUMN source_id, DROP COLUMN source_type;
