-- Authoritative financial records. Amounts are BIGINT minor units and are
-- paired with an ISO currency; floats and implicit scale conversions are banned.

CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    external_reference TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status TEXT NOT NULL CHECK (status IN ('active', 'frozen', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at TIMESTAMPTZ,
    UNIQUE (id, tenant_id),
    CHECK ((status = 'closed') = (closed_at IS NOT NULL))
);
CREATE INDEX accounts_tenant_id_idx ON accounts (tenant_id, id);

CREATE TABLE account_owners (
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    subject_id TEXT NOT NULL,
    permission TEXT NOT NULL CHECK (permission IN ('read', 'debit')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, subject_id),
    FOREIGN KEY (account_id, tenant_id) REFERENCES accounts(id, tenant_id)
);
CREATE INDEX account_owners_subject_idx ON account_owners (tenant_id, subject_id, account_id);

CREATE TABLE account_balance_projections (
    account_id UUID PRIMARY KEY REFERENCES accounts(id),
    available_minor BIGINT NOT NULL CHECK (available_minor >= 0),
    ledger_minor BIGINT NOT NULL CHECK (ledger_minor >= 0),
    balance_version BIGINT NOT NULL DEFAULT 0 CHECK (balance_version >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE transfers (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    actor_subject_id TEXT NOT NULL,
    debit_account_id UUID NOT NULL,
    credit_account_id UUID NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status TEXT NOT NULL CHECK (status IN ('pending', 'posted', 'rejected')),
    rejection_code TEXT,
    journal_transaction_id UUID UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK (debit_account_id <> credit_account_id),
    FOREIGN KEY (debit_account_id, tenant_id) REFERENCES accounts(id, tenant_id),
    FOREIGN KEY (credit_account_id, tenant_id) REFERENCES accounts(id, tenant_id),
    UNIQUE (id, tenant_id),
    CHECK ((status = 'pending' AND completed_at IS NULL AND journal_transaction_id IS NULL)
        OR (status = 'posted' AND completed_at IS NOT NULL AND journal_transaction_id IS NOT NULL AND rejection_code IS NULL)
        OR (status = 'rejected' AND completed_at IS NOT NULL AND journal_transaction_id IS NULL AND rejection_code IS NOT NULL))
);
CREATE INDEX transfers_debit_account_idx ON transfers (tenant_id, debit_account_id, created_at DESC);
CREATE INDEX transfers_credit_account_idx ON transfers (tenant_id, credit_account_id, created_at DESC);

CREATE TABLE journal_transactions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    transfer_id UUID NOT NULL UNIQUE REFERENCES transfers(id),
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ledger_postings (
    id UUID PRIMARY KEY,
    journal_transaction_id UUID NOT NULL REFERENCES journal_transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    direction TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ledger_postings_account_idx ON ledger_postings (account_id, occurred_at DESC, id DESC);
CREATE INDEX ledger_postings_journal_idx ON ledger_postings (journal_transaction_id, currency);

ALTER TABLE transfers
    ADD CONSTRAINT transfers_journal_transaction_fk
    FOREIGN KEY (journal_transaction_id) REFERENCES journal_transactions(id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE idempotency_requests (
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    actor_subject_id TEXT NOT NULL,
    operation TEXT NOT NULL,
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 255),
    request_fingerprint BYTEA NOT NULL CHECK (octet_length(request_fingerprint) = 32),
    state TEXT NOT NULL CHECK (state IN ('in_progress', 'completed', 'failed')),
    response_status SMALLINT,
    response_body JSONB,
    transfer_id UUID REFERENCES transfers(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, actor_subject_id, operation, idempotency_key),
    CHECK ((state = 'in_progress' AND response_status IS NULL AND response_body IS NULL AND completed_at IS NULL)
        OR (state IN ('completed', 'failed') AND response_status IS NOT NULL AND response_body IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE INDEX idempotency_expiry_idx ON idempotency_requests (expires_at);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    transfer_id UUID NOT NULL,
    account_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL CHECK (aggregate_version >= 0),
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    published_at TIMESTAMPTZ,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (transfer_id, account_id, event_type),
    FOREIGN KEY (transfer_id, tenant_id) REFERENCES transfers(id, tenant_id),
    FOREIGN KEY (account_id, tenant_id) REFERENCES accounts(id, tenant_id)
);
CREATE INDEX outbox_unpublished_idx ON outbox_events (available_at, created_at) WHERE published_at IS NULL;

CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    actor_subject_id TEXT,
    event_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('allowed', 'denied', 'succeeded', 'failed')),
    correlation_id UUID NOT NULL,
    sanitized_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_tenant_time_idx ON audit_events (tenant_id, occurred_at DESC);
