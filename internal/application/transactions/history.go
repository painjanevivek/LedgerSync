// Package transactions owns the safe, account-scoped transaction history read model.
package transactions

import (
	"context"
	"errors"
	"time"
)

var ErrHistoryNotFound = errors.New("history account not found or not authorized")

type Entry struct {
	TransferID             string    `json:"transfer_id"`
	Direction              string    `json:"direction"`
	Amount                 string    `json:"amount"`
	Currency               string    `json:"currency"`
	Status                 string    `json:"status"`
	OccurredAt             time.Time `json:"occurred_at"`
	CorrectionID           string    `json:"correction_id,omitempty"`
	CorrectionStatus       string    `json:"correction_status,omitempty"`
	CorrectionRole         string    `json:"correction_role,omitempty"`
	OriginalTransferID     string    `json:"original_transfer_id,omitempty"`
	CompensationTransferID string    `json:"compensation_transfer_id,omitempty"`
}

type HistoryRepository interface {
	ListAccountHistory(context.Context, string, string, string, string, int) ([]Entry, string, error)
}

type History struct{ repository HistoryRepository }

func NewHistory(repository HistoryRepository) (*History, error) {
	if repository == nil {
		return nil, errors.New("history repository is required")
	}
	return &History{repository: repository}, nil
}

func (h *History) List(ctx context.Context, tenantID, actorID, accountID, cursor string, limit int) ([]Entry, string, error) {
	if h == nil || tenantID == "" || actorID == "" || accountID == "" {
		return nil, "", ErrHistoryNotFound
	}
	if limit < 1 || limit > 100 {
		return nil, "", errors.New("history limit must be between 1 and 100")
	}
	return h.repository.ListAccountHistory(ctx, tenantID, actorID, accountID, cursor, limit)
}
