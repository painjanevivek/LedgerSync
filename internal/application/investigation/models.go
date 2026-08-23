package investigation

import (
	"context"
	"time"
)

type TransferFilter struct {
	Cursor, AccountID, Status, Query string
	From, To                         time.Time
	Limit                            int
}

type TransferSummary struct {
	ID                   string    `json:"transfer_id"`
	DebitAccountID       string    `json:"source_account_id"`
	CreditAccountID      string    `json:"destination_account_id"`
	AmountMinor          string    `json:"amount_minor"`
	Currency             string    `json:"currency"`
	FinancialStatus      string    `json:"financial_status"`
	DeliveryStatus       string    `json:"delivery_status"`
	CreatedAt            time.Time `json:"created_at"`
	CompletedAt          time.Time `json:"completed_at"`
	JournalTransactionID string    `json:"journal_transaction_id,omitempty"`
	RejectionCode        string    `json:"rejection_code,omitempty"`
}

type Posting struct {
	ID          string    `json:"posting_id"`
	AccountID   string    `json:"account_id"`
	Direction   string    `json:"direction"`
	AmountMinor string    `json:"amount_minor"`
	Currency    string    `json:"currency"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type EvidenceEvent struct {
	ID         string    `json:"event_id"`
	Kind       string    `json:"kind"`
	Outcome    string    `json:"outcome"`
	Reference  string    `json:"reference,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type TransferDetail struct {
	TransferSummary
	ActorSubjectID string          `json:"actor_subject_id"`
	Postings       []Posting       `json:"postings"`
	Timeline       []EvidenceEvent `json:"timeline"`
}

type ReconciliationRun struct {
	ID                  string                   `json:"run_id"`
	Status              string                   `json:"status"`
	CorrelationID       string                   `json:"correlation_id"`
	Scope               string                   `json:"scope"`
	LedgerWatermark     string                   `json:"ledger_watermark"`
	ApplicationVersion  string                   `json:"application_version"`
	SchemaVersion       string                   `json:"schema_version"`
	CheckedAccountCount string                   `json:"checked_account_count"`
	PostingCount        string                   `json:"posting_count"`
	MismatchCount       string                   `json:"mismatch_count"`
	StartedAt           time.Time                `json:"started_at"`
	CompletedAt         time.Time                `json:"completed_at"`
	Mismatches          []ReconciliationMismatch `json:"mismatches,omitempty"`
}

type ReconciliationMismatch struct {
	ID                     string    `json:"mismatch_id"`
	AccountID              string    `json:"account_id,omitempty"`
	Classification         string    `json:"classification"`
	Currency               string    `json:"currency,omitempty"`
	ExpectedMinor          string    `json:"expected_minor,omitempty"`
	ObservedMinor          string    `json:"observed_minor,omitempty"`
	ObservedAvailableMinor string    `json:"observed_available_minor,omitempty"`
	BalanceVersion         string    `json:"balance_version,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

type Repository interface {
	ListTransfers(ctx context.Context, tenantID string, filter TransferFilter) ([]TransferSummary, string, error)
	GetTransfer(ctx context.Context, tenantID, transferID string) (TransferDetail, error)
	ListReconciliationRuns(ctx context.Context, tenantID, cursor string, limit int) ([]ReconciliationRun, string, error)
	GetReconciliationRun(ctx context.Context, tenantID, runID string) (ReconciliationRun, error)
}
