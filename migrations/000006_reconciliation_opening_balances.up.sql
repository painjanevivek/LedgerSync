-- Reconciliation needs an immutable baseline for accounts whose opening
-- balance predates the LedgerSync journal history. New LedgerSync accounts
-- start at zero; migrated accounts receive one reviewed baseline record.
CREATE TABLE account_opening_balances (
    account_id UUID PRIMARY KEY REFERENCES accounts(id),
    opening_ledger_minor BIGINT NOT NULL CHECK (opening_ledger_minor >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
