-- Expand immutable ledger rows with tenant- and command-aware keys. This
-- migration intentionally leaves the new columns nullable and the composite
-- foreign keys NOT VALID; PR-007 validates historical data and makes the
-- semantic boundary mandatory after the read-only report is clean.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

ALTER TABLE journal_transactions
  ADD COLUMN source_type TEXT,
  ADD COLUMN source_id UUID;

ALTER TABLE ledger_postings
  ADD COLUMN tenant_id UUID;

COMMENT ON COLUMN journal_transactions.source_type IS 'Derived immutable command family; enforced after expand validation';
COMMENT ON COLUMN journal_transactions.source_id IS 'Derived immutable transfer or funding command identifier';
COMMENT ON COLUMN ledger_postings.tenant_id IS 'Derived immutable tenant boundary copied from the parent journal';

CREATE FUNCTION hydrate_journal_semantic_keys()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
BEGIN
  IF NEW.source_type IS NULL THEN
    NEW.source_type := CASE
      WHEN NEW.transfer_id IS NOT NULL THEN 'transfer'
      WHEN NEW.funding_event_id IS NOT NULL THEN 'funding_event'
      ELSE NULL
    END;
  END IF;
  IF NEW.source_id IS NULL THEN
    NEW.source_id := COALESCE(NEW.transfer_id, NEW.funding_event_id);
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER journal_transactions_hydrate_semantic_keys
BEFORE INSERT OR UPDATE OF transfer_id, funding_event_id, source_type, source_id
ON journal_transactions
FOR EACH ROW EXECUTE FUNCTION hydrate_journal_semantic_keys();

CREATE FUNCTION hydrate_posting_tenant_key()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
BEGIN
  IF NEW.tenant_id IS NULL THEN
    SELECT journal.tenant_id
      INTO NEW.tenant_id
      FROM public.journal_transactions AS journal
     WHERE journal.id = NEW.journal_transaction_id;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER ledger_postings_hydrate_tenant_key
BEFORE INSERT OR UPDATE OF journal_transaction_id, tenant_id
ON ledger_postings
FOR EACH ROW EXECUTE FUNCTION hydrate_posting_tenant_key();

REVOKE ALL ON FUNCTION hydrate_journal_semantic_keys() FROM PUBLIC;
REVOKE ALL ON FUNCTION hydrate_posting_tenant_key() FROM PUBLIC;

-- Use small, deterministic batches to bound statement memory and row-lock
-- acquisition. The migration runner commits the complete expansion atomically.
-- Disable only the two immutable-row guards while the migration owner derives
-- additive keys; all existing business columns remain untouched.
ALTER TABLE journal_transactions DISABLE TRIGGER journal_transactions_append_only;
ALTER TABLE ledger_postings DISABLE TRIGGER ledger_postings_no_update;

DO $$
DECLARE
  affected INTEGER;
BEGIN
  LOOP
    WITH batch AS (
      SELECT ctid
      FROM journal_transactions
      WHERE source_type IS NULL OR source_id IS NULL
      ORDER BY id
      LIMIT 5000
      FOR UPDATE SKIP LOCKED
    )
    UPDATE journal_transactions AS journal
       SET source_type = CASE
             WHEN journal.transfer_id IS NOT NULL THEN 'transfer'
             WHEN journal.funding_event_id IS NOT NULL THEN 'funding_event'
             ELSE NULL
           END,
           source_id = COALESCE(journal.transfer_id, journal.funding_event_id)
      FROM batch
     WHERE journal.ctid = batch.ctid;
    GET DIAGNOSTICS affected = ROW_COUNT;
    EXIT WHEN affected = 0;
  END LOOP;

  LOOP
    WITH batch AS (
      SELECT posting.ctid, journal.tenant_id
      FROM ledger_postings AS posting
      JOIN journal_transactions AS journal ON journal.id = posting.journal_transaction_id
      WHERE posting.tenant_id IS NULL
      ORDER BY posting.id
      LIMIT 5000
      FOR UPDATE OF posting SKIP LOCKED
    )
    UPDATE ledger_postings AS posting
       SET tenant_id = batch.tenant_id
      FROM batch
     WHERE posting.ctid = batch.ctid;
    GET DIAGNOSTICS affected = ROW_COUNT;
    EXIT WHEN affected = 0;
  END LOOP;
END;
$$;

ALTER TABLE journal_transactions ENABLE TRIGGER journal_transactions_append_only;
ALTER TABLE ledger_postings ENABLE TRIGGER ledger_postings_no_update;

CREATE UNIQUE INDEX journal_transactions_id_tenant_key_idx
  ON journal_transactions (id, tenant_id);
ALTER TABLE journal_transactions
  ADD CONSTRAINT journal_transactions_id_tenant_key
  UNIQUE USING INDEX journal_transactions_id_tenant_key_idx;

CREATE UNIQUE INDEX ledger_postings_id_tenant_key_idx
  ON ledger_postings (id, tenant_id);
ALTER TABLE ledger_postings
  ADD CONSTRAINT ledger_postings_id_tenant_key
  UNIQUE USING INDEX ledger_postings_id_tenant_key_idx;

ALTER TABLE journal_transactions
  ADD CONSTRAINT journal_transfer_tenant_fk
    FOREIGN KEY (transfer_id, tenant_id)
    REFERENCES transfers (id, tenant_id)
    NOT VALID,
  ADD CONSTRAINT journal_funding_tenant_fk
    FOREIGN KEY (funding_event_id, tenant_id)
    REFERENCES funding_events (id, tenant_id)
    NOT VALID,
  ADD CONSTRAINT journal_source_type_expand_check
    CHECK (source_type IS NULL OR source_type IN ('transfer', 'funding_event'))
    NOT VALID,
  ADD CONSTRAINT journal_source_pair_expand_check
    CHECK ((source_type IS NULL) = (source_id IS NULL))
    NOT VALID;

ALTER TABLE ledger_postings
  ADD CONSTRAINT ledger_posting_journal_tenant_fk
    FOREIGN KEY (journal_transaction_id, tenant_id)
    REFERENCES journal_transactions (id, tenant_id)
    NOT VALID,
  ADD CONSTRAINT ledger_posting_account_tenant_fk
    FOREIGN KEY (account_id, tenant_id)
    REFERENCES accounts (id, tenant_id)
    NOT VALID;

CREATE VIEW ledger_semantic_key_validation AS
SELECT
  (SELECT count(*) FROM journal_transactions)::bigint AS journal_row_count,
  (SELECT count(*) FROM ledger_postings)::bigint AS posting_row_count,
  (SELECT count(*) FROM journal_transactions
    WHERE source_type IS NULL OR source_id IS NULL)::bigint AS unbackfilled_journal_count,
  (SELECT count(*) FROM ledger_postings
    WHERE tenant_id IS NULL)::bigint AS unbackfilled_posting_count,
  (SELECT count(*) FROM journal_transactions
    WHERE (source_type = 'transfer' AND source_id IS DISTINCT FROM transfer_id)
       OR (source_type = 'funding_event' AND source_id IS DISTINCT FROM funding_event_id))::bigint
    AS journal_source_mismatch_count,
  (SELECT count(*)
     FROM journal_transactions AS journal
     LEFT JOIN transfers AS command
       ON command.id = journal.transfer_id AND command.tenant_id = journal.tenant_id
    WHERE journal.transfer_id IS NOT NULL AND command.id IS NULL)::bigint
    AS transfer_tenant_mismatch_count,
  (SELECT count(*)
     FROM journal_transactions AS journal
     LEFT JOIN funding_events AS command
       ON command.id = journal.funding_event_id AND command.tenant_id = journal.tenant_id
    WHERE journal.funding_event_id IS NOT NULL AND command.id IS NULL)::bigint
    AS funding_tenant_mismatch_count,
  (SELECT count(*)
     FROM ledger_postings AS posting
     LEFT JOIN journal_transactions AS journal
       ON journal.id = posting.journal_transaction_id AND journal.tenant_id = posting.tenant_id
    WHERE journal.id IS NULL)::bigint
    AS posting_journal_tenant_mismatch_count,
  (SELECT count(*)
     FROM ledger_postings AS posting
     LEFT JOIN accounts AS account
       ON account.id = posting.account_id AND account.tenant_id = posting.tenant_id
    WHERE account.id IS NULL)::bigint
    AS posting_account_tenant_mismatch_count,
  (SELECT COALESCE(sum(duplicates - 1), 0)::bigint
     FROM (
       SELECT count(*)::bigint AS duplicates
       FROM journal_transactions
       WHERE source_type IS NOT NULL AND source_id IS NOT NULL
       GROUP BY tenant_id, source_type, source_id
       HAVING count(*) > 1
     ) AS repeated_sources)::bigint AS duplicate_journal_source_count;

COMMENT ON VIEW ledger_semantic_key_validation IS 'Count-only readiness evidence for validating expanded ledger semantic keys';
REVOKE ALL ON ledger_semantic_key_validation FROM PUBLIC;
