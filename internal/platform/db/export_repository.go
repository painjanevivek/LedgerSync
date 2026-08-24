package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	appexports "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/exports"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
)

// ExportRepository composes the existing authorization-aware read models and
// adds bounded mismatch paging. It never loads an entire export into memory.
type ExportRepository struct {
	database      *sql.DB
	investigation *InvestigationRepository
	history       *TransactionHistoryRepository
}

func NewExportRepository(database *sql.DB) (*ExportRepository, error) {
	if database == nil {
		return nil, errors.New("export database is required")
	}
	investigationRepository, err := NewInvestigationRepository(database)
	if err != nil {
		return nil, err
	}
	historyRepository, err := NewTransactionHistoryRepository(database)
	if err != nil {
		return nil, err
	}
	return &ExportRepository{database: database, investigation: investigationRepository, history: historyRepository}, nil
}

func (r *ExportRepository) ListTransfers(ctx context.Context, tenantID string, filter investigation.TransferFilter) ([]investigation.TransferSummary, string, error) {
	return r.investigation.ListTransfers(ctx, tenantID, filter)
}

func (r *ExportRepository) ListAccountHistory(ctx context.Context, tenantID, actorID, accountID, cursor string, limit int) ([]transactions.Entry, string, error) {
	return r.history.ListAccountHistory(ctx, tenantID, actorID, accountID, cursor, limit)
}

func (r *ExportRepository) ListReconciliationRuns(ctx context.Context, tenantID string, filter appexports.ReconciliationFilter) ([]investigation.ReconciliationRun, string, error) {
	cursor, err := decodeInvestigationCursor(filter.Cursor)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.database.QueryContext(ctx, `
SELECT id,status,checked_account_count,posting_count,mismatch_count,correlation_id,started_at,completed_at,scope,ledger_watermark,application_version,schema_version
FROM reconciliation_runs
WHERE tenant_id=$1
 AND ($2='' OR id::text=$2)
 AND ($3='' OR status=$3)
 AND ($4::timestamptz IS NULL OR completed_at >= $4)
 AND ($5::timestamptz IS NULL OR completed_at <= $5)
 AND ($6::timestamptz IS NULL OR (completed_at,id)<($6::timestamptz,$7::uuid))
ORDER BY completed_at DESC,id DESC LIMIT $8`, tenantID, filter.RunID, filter.Status, nullableTime(filter.From), nullableTime(filter.To), nullableTime(cursor.At), nullableString(cursor.ID), filter.Limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list export reconciliation runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]investigation.ReconciliationRun, 0, filter.Limit)
	for rows.Next() {
		item, scanErr := scanReconciliation(rows)
		if scanErr != nil {
			return nil, "", scanErr
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > filter.Limit {
		last := items[filter.Limit-1]
		next = encodeInvestigationCursor(last.CompletedAt, last.ID)
		items = items[:filter.Limit]
	}
	return items, next, nil
}

func (r *ExportRepository) ListReconciliationMismatches(ctx context.Context, tenantID, runID, rawCursor string, limit int) ([]investigation.ReconciliationMismatch, string, error) {
	cursor, err := decodeInvestigationCursor(rawCursor)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.database.QueryContext(ctx, `
SELECT id,COALESCE(account_id::text,''),classification,COALESCE(currency,''),COALESCE(expected_minor::text,''),COALESCE(observed_minor::text,''),COALESCE(observed_available_minor::text,''),COALESCE(balance_version::text,''),created_at
FROM reconciliation_mismatches
WHERE tenant_id=$1 AND run_id=$2
 AND ($3::timestamptz IS NULL OR (created_at,id)>($3::timestamptz,$4::uuid))
ORDER BY created_at,id LIMIT $5`, tenantID, runID, nullableTime(cursor.At), nullableString(cursor.ID), limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list export reconciliation mismatches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]investigation.ReconciliationMismatch, 0, limit)
	for rows.Next() {
		var item investigation.ReconciliationMismatch
		if scanErr := rows.Scan(&item.ID, &item.AccountID, &item.Classification, &item.Currency, &item.ExpectedMinor, &item.ObservedMinor, &item.ObservedAvailableMinor, &item.BalanceVersion, &item.CreatedAt); scanErr != nil {
			return nil, "", scanErr
		}
		item.CreatedAt = item.CreatedAt.UTC()
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeInvestigationCursor(last.CreatedAt, last.ID)
		items = items[:limit]
	}
	return items, next, nil
}
