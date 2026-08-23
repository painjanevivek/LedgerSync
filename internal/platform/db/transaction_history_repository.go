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
SELECT id, CASE WHEN debit_account_id = $2 THEN 'debit' ELSE 'credit' END, amount_minor, currency, status, completed_at
FROM transfers
WHERE tenant_id=$1 AND status='posted' AND (debit_account_id=$2 OR credit_account_id=$2)
  AND ($3::timestamptz IS NULL OR (completed_at, id) < ($3::timestamptz, $4::uuid))
ORDER BY completed_at DESC, id DESC LIMIT $5`, tenantID, accountID, nullableTime(cursor.CompletedAt), nullableString(cursor.TransferID), limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var entries []transactions.Entry
	for rows.Next() {
		var item transactions.Entry
		if err := rows.Scan(&item.TransferID, &item.Direction, &item.Amount, &item.Currency, &item.Status, &item.OccurredAt); err != nil {
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
