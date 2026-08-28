package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
)

type TransactionHistoryRepository struct{ database *sql.DB }

func NewTransactionHistoryRepository(database *sql.DB) (*TransactionHistoryRepository, error) {
	if database == nil {
		return nil, errors.New("transaction history database is required")
	}
	return &TransactionHistoryRepository{database: database}, nil
}

type historyCursor struct {
	CompletedAt time.Time
	TransferID  string
}

func decodeHistoryCursor(raw string) (historyCursor, error) {
	if raw == "" {
		return historyCursor{}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return historyCursor{}, errors.New("invalid history cursor")
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return historyCursor{}, errors.New("invalid history cursor")
	}
	when, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil || strings.TrimSpace(parts[1]) == "" {
		return historyCursor{}, errors.New("invalid history cursor")
	}
	return historyCursor{CompletedAt: when.UTC(), TransferID: parts[1]}, nil
}
func encodeHistoryCursor(entry transactions.Entry) string {
	return base64.RawURLEncoding.EncodeToString([]byte(entry.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + entry.TransferID))
}

func (r *TransactionHistoryRepository) ListAccountHistory(ctx context.Context, tenantID, actorID, accountID, rawCursor string, limit int) ([]transactions.Entry, string, error) {
	var allowed bool
	if err := r.database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_owners WHERE tenant_id=$1 AND account_id=$2 AND subject_id=$3 AND permission IN ('read','debit'))`, tenantID, accountID, actorID).Scan(&allowed); err != nil {
		return nil, "", fmt.Errorf("authorize history: %w", err)
	}
	if !allowed {
		return nil, "", transactions.ErrHistoryNotFound
	}
	cursor, err := decodeHistoryCursor(rawCursor)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.database.QueryContext(ctx, `
SELECT transfer.id, CASE WHEN transfer.debit_account_id = $2 THEN 'debit' ELSE 'credit' END, transfer.amount_minor,
 transfer.currency,transfer.status,transfer.completed_at,COALESCE(correction.id::text,''),COALESCE(correction.status,''),
 CASE WHEN correction.id IS NULL THEN '' WHEN correction.original_transfer_id=transfer.id THEN 'original' ELSE 'compensation' END,
 COALESCE(correction.original_transfer_id::text,''),COALESCE(correction.compensation_transfer_id::text,'')
FROM transfers transfer
LEFT JOIN transfer_corrections correction ON correction.tenant_id=transfer.tenant_id AND (correction.original_transfer_id=transfer.id OR correction.compensation_transfer_id=transfer.id)
WHERE transfer.tenant_id=$1 AND transfer.status='posted' AND (transfer.debit_account_id=$2 OR transfer.credit_account_id=$2)
  AND ($3::timestamptz IS NULL OR (transfer.completed_at, transfer.id) < ($3::timestamptz, $4::uuid))
ORDER BY transfer.completed_at DESC, transfer.id DESC LIMIT $5`, tenantID, accountID, nullableTime(cursor.CompletedAt), nullableString(cursor.TransferID), limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var entries []transactions.Entry
	for rows.Next() {
		var item transactions.Entry
		if err := rows.Scan(&item.TransferID, &item.Direction, &item.Amount, &item.Currency, &item.Status, &item.OccurredAt, &item.CorrectionID, &item.CorrectionStatus, &item.CorrectionRole, &item.OriginalTransferID, &item.CompensationTransferID); err != nil {
			return nil, "", fmt.Errorf("scan history: %w", err)
		}
		item.OccurredAt = item.OccurredAt.UTC()
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate history: %w", err)
	}
	next := ""
	if len(entries) > limit {
		next = encodeHistoryCursor(entries[limit-1])
		entries = entries[:limit]
	}
	return entries, next, nil
}
func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
