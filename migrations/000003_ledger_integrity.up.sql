-- Make the ledger append-only and fail a transaction whose postings do not
-- balance per currency. Corrections must be compensating transactions.

CREATE OR REPLACE FUNCTION reject_ledger_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'ledger_postings are immutable' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER ledger_postings_no_update
BEFORE UPDATE OR DELETE ON ledger_postings
FOR EACH ROW EXECUTE FUNCTION reject_ledger_mutation();

CREATE OR REPLACE FUNCTION enforce_journal_balance()
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
        RAISE EXCEPTION 'journal transaction % is not balanced', target_journal_id
            USING ERRCODE = '23514';
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

CREATE OR REPLACE FUNCTION enforce_posting_account_currency()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM accounts WHERE id = NEW.account_id AND currency = NEW.currency
    ) THEN
        RAISE EXCEPTION 'posting currency must match account currency' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER ledger_posting_currency_matches_account
BEFORE INSERT ON ledger_postings
FOR EACH ROW EXECUTE FUNCTION enforce_posting_account_currency();

REVOKE UPDATE, DELETE ON ledger_postings FROM PUBLIC;
