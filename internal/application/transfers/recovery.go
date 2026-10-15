package transfers

import (
	"context"
	"time"
)

// RecoveryIntent is an immutable confirmed intent, not a reservation of funds.
// Private retry identity stays on the server and is never serialized to a list.
type RecoveryIntent struct {
	RequestReference     string    `json:"request_reference"`
	SourceAccountID      string    `json:"source_account_id"`
	DestinationAccountID string    `json:"destination_account_id"`
	Currency             string    `json:"currency"`
	AmountMinor          string    `json:"amount_minor"`
	CreatedAt            time.Time `json:"created_at"`
	State                string    `json:"state"`
	TransferID           string    `json:"transfer_id,omitempty"`
	IdempotencyKey       string    `json:"-"`
}

type RecoveryPage struct {
	Requests   []RecoveryIntent `json:"requests"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type RecoveryRepository interface {
	PrepareRecovery(context.Context, Command) error
	ListRecovery(context.Context, string, string, string) (RecoveryPage, error)
	GetRecovery(context.Context, string, string, string) (RecoveryIntent, error)
}
