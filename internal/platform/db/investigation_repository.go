package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
)

var ErrInvestigationNotFound = errors.New("investigation record not found")

type InvestigationRepository struct{ database *sql.DB }

func NewInvestigationRepository(database *sql.DB) (*InvestigationRepository, error) {
	if database == nil {
		return nil, errors.New("investigation database is required")
	}
	return &InvestigationRepository{database: database}, nil
}

type investigationCursor struct {
	At time.Time
	ID string
}

type transferCursor struct {
	At          time.Time `json:"at"`
	ID          string    `json:"id"`
	Fingerprint string    `json:"fp"`
}

func decodeInvestigationCursor(raw string) (investigationCursor, error) {
	if raw == "" {
		return investigationCursor{}, nil
	}
	value, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return investigationCursor{}, errors.New("invalid cursor")
	}
	parts := strings.Split(string(value), "|")
	if len(parts) != 2 {
		return investigationCursor{}, errors.New("invalid cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil || parts[1] == "" {
		return investigationCursor{}, errors.New("invalid cursor")
	}
	return investigationCursor{At: at.UTC(), ID: parts[1]}, nil
}

func encodeInvestigationCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeTransferCursor(raw, fingerprint string) (transferCursor, error) {
	if raw == "" {
		return transferCursor{}, nil
	}
	if len(raw) > 768 {
		return transferCursor{}, errors.New("invalid transfer cursor")
	}
	value, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(value) > 512 {
		return transferCursor{}, errors.New("invalid transfer cursor")
	}
	parts := strings.Split(string(value), "|")
	if len(parts) == 2 && fingerprint == transferFilterFingerprint(investigation.TransferFilter{}) {
		at, parseErr := time.Parse(time.RFC3339Nano, parts[0])
		if parseErr == nil && safeEvidenceUUID.MatchString(strings.ToLower(parts[1])) {
			return transferCursor{At: at.UTC(), ID: strings.ToLower(parts[1]), Fingerprint: fingerprint}, nil
		}
	}
	if len(parts) != 3 || parts[2] != fingerprint || !safeEvidenceUUID.MatchString(strings.ToLower(parts[1])) {
		return transferCursor{}, errors.New("invalid transfer cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil || at.IsZero() {
		return transferCursor{}, errors.New("invalid transfer cursor")
	}
	return transferCursor{At: at.UTC(), ID: strings.ToLower(parts[1]), Fingerprint: fingerprint}, nil
}

func encodeTransferCursor(at time.Time, id, fingerprint string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + strings.ToLower(id) + "|" + fingerprint))
}

func transferFilterFingerprint(filter investigation.TransferFilter) string {
	from, to := "", ""
	if !filter.From.IsZero() {
		from = filter.From.UTC().Format(time.RFC3339Nano)
	}
	if !filter.To.IsZero() {
		to = filter.To.UTC().Format(time.RFC3339Nano)
	}
	canonical := strings.Join([]string{strings.ToLower(strings.TrimSpace(filter.AccountID)), strings.ToLower(strings.TrimSpace(filter.Status)), strings.ToLower(strings.TrimSpace(filter.Query)), from, to}, "\x00")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func (r *InvestigationRepository) ListTransfers(ctx context.Context, tenantID string, filter investigation.TransferFilter) ([]investigation.TransferSummary, string, error) {
	fingerprint := transferFilterFingerprint(filter)
	cursor, err := decodeTransferCursor(filter.Cursor, fingerprint)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.database.QueryContext(ctx, `
SELECT t.id,t.debit_account_id,t.credit_account_id,t.amount_minor,t.currency,t.status,
 CASE WHEN t.status<>'posted' THEN 'not_applicable' ELSE COALESCE((SELECT d.status FROM delivery_attempts d WHERE d.tenant_id=t.tenant_id AND d.transfer_id=t.id ORDER BY d.created_at DESC,d.id DESC LIMIT 1),'not_applicable') END,
 t.created_at,COALESCE(t.completed_at,t.created_at),COALESCE(t.journal_transaction_id::text,''),COALESCE(t.rejection_code,'')
FROM transfers t WHERE t.tenant_id=$1
	 AND ($2='' OR t.status=$2) AND ($3='' OR t.debit_account_id=NULLIF($3,'')::uuid OR t.credit_account_id=NULLIF($3,'')::uuid)
 AND ($4='' OR t.id::text ILIKE '%'||$4||'%' OR t.debit_account_id::text ILIKE '%'||$4||'%' OR t.credit_account_id::text ILIKE '%'||$4||'%')
 AND ($5::timestamptz IS NULL OR COALESCE(t.completed_at,t.created_at) >= $5)
 AND ($6::timestamptz IS NULL OR COALESCE(t.completed_at,t.created_at) <= $6)
 AND ($7::timestamptz IS NULL OR (COALESCE(t.completed_at,t.created_at),t.id) < ($7::timestamptz,$8::uuid))
ORDER BY COALESCE(t.completed_at,t.created_at) DESC,t.id DESC LIMIT $9`, tenantID, filter.Status, filter.AccountID, filter.Query, nullableTime(filter.From), nullableTime(filter.To), nullableTime(cursor.At), nullableString(cursor.ID), filter.Limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list transfers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]investigation.TransferSummary, 0, filter.Limit)
	for rows.Next() {
		var item investigation.TransferSummary
		if err := rows.Scan(&item.ID, &item.DebitAccountID, &item.CreditAccountID, &item.AmountMinor, &item.Currency, &item.FinancialStatus, &item.DeliveryStatus, &item.CreatedAt, &item.CompletedAt, &item.JournalTransactionID, &item.RejectionCode); err != nil {
			return nil, "", err
		}
		item.CreatedAt = item.CreatedAt.UTC()
		item.CompletedAt = item.CompletedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > filter.Limit {
		last := items[filter.Limit-1]
		next = encodeTransferCursor(last.CompletedAt, last.ID, fingerprint)
		items = items[:filter.Limit]
	}
	return items, next, nil
}

func (r *InvestigationRepository) GetTransfer(ctx context.Context, tenantID, transferID string) (investigation.TransferDetail, error) {
	var item investigation.TransferDetail
	err := r.database.QueryRowContext(ctx, `SELECT t.id,t.debit_account_id,t.credit_account_id,t.amount_minor,t.currency,t.status,CASE WHEN t.status<>'posted' THEN 'not_applicable' ELSE COALESCE((SELECT d.status FROM delivery_attempts d WHERE d.tenant_id=t.tenant_id AND d.transfer_id=t.id ORDER BY d.created_at DESC,d.id DESC LIMIT 1),'not_applicable') END,t.created_at,COALESCE(t.completed_at,t.created_at),COALESCE(t.journal_transaction_id::text,''),COALESCE(t.rejection_code,''),t.actor_subject_id FROM transfers t WHERE t.tenant_id=$1 AND t.id=$2`, tenantID, transferID).Scan(&item.ID, &item.DebitAccountID, &item.CreditAccountID, &item.AmountMinor, &item.Currency, &item.FinancialStatus, &item.DeliveryStatus, &item.CreatedAt, &item.CompletedAt, &item.JournalTransactionID, &item.RejectionCode, &item.ActorSubjectID)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrInvestigationNotFound
	}
	if err != nil {
		return item, err
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.CompletedAt = item.CompletedAt.UTC()
	rows, err := r.database.QueryContext(ctx, `SELECT p.id,p.account_id,p.direction,p.amount_minor,p.currency,p.occurred_at FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.tenant_id=$1 AND j.transfer_id=$2 ORDER BY p.direction DESC,p.id`, tenantID, transferID)
	if err != nil {
		return item, err
	}
	defer func() { _ = rows.Close() }()
	item.Postings = []investigation.Posting{}
	for rows.Next() {
		var p investigation.Posting
		if err := rows.Scan(&p.ID, &p.AccountID, &p.Direction, &p.AmountMinor, &p.Currency, &p.OccurredAt); err != nil {
			return item, err
		}
		p.OccurredAt = p.OccurredAt.UTC()
		item.Postings = append(item.Postings, p)
	}
	item.Timeline = []investigation.EvidenceEvent{{ID: item.ID, Kind: "transfer_created", Outcome: item.FinancialStatus, Reference: item.JournalTransactionID, OccurredAt: item.CreatedAt}}
	return item, nil
}

func (r *InvestigationRepository) ListReconciliationRuns(ctx context.Context, tenantID, rawCursor string, limit int) ([]investigation.ReconciliationRun, string, error) {
	cursor, err := decodeInvestigationCursor(rawCursor)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.database.QueryContext(ctx, `SELECT id,status,checked_account_count,posting_count,mismatch_count,correlation_id,started_at,completed_at,scope,ledger_watermark,application_version,schema_version FROM reconciliation_runs WHERE tenant_id=$1 AND ($2::timestamptz IS NULL OR (completed_at,id)<($2::timestamptz,$3::uuid)) ORDER BY completed_at DESC,id DESC LIMIT $4`, tenantID, nullableTime(cursor.At), nullableString(cursor.ID), limit+1)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()
	items := make([]investigation.ReconciliationRun, 0, limit)
	for rows.Next() {
		item, err := scanReconciliation(rows)
		if err != nil {
			return nil, "", err
		}
		items = append(items, item)
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeInvestigationCursor(last.CompletedAt, last.ID)
		items = items[:limit]
	}
	return items, next, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanReconciliation(row rowScanner) (investigation.ReconciliationRun, error) {
	var item investigation.ReconciliationRun
	var checked, postings, mismatches int64
	if err := row.Scan(&item.ID, &item.Status, &checked, &postings, &mismatches, &item.CorrelationID, &item.StartedAt, &item.CompletedAt, &item.Scope, &item.LedgerWatermark, &item.ApplicationVersion, &item.SchemaVersion); err != nil {
		return item, err
	}
	item.CheckedAccountCount = strconv.FormatInt(checked, 10)
	item.PostingCount = strconv.FormatInt(postings, 10)
	item.MismatchCount = strconv.FormatInt(mismatches, 10)
	item.StartedAt = item.StartedAt.UTC()
	item.CompletedAt = item.CompletedAt.UTC()
	return item, nil
}
func (r *InvestigationRepository) GetReconciliationRun(ctx context.Context, tenantID, runID string) (investigation.ReconciliationRun, error) {
	row := r.database.QueryRowContext(ctx, `SELECT id,status,checked_account_count,posting_count,mismatch_count,correlation_id,started_at,completed_at,scope,ledger_watermark,application_version,schema_version FROM reconciliation_runs WHERE tenant_id=$1 AND id=$2`, tenantID, runID)
	item, err := scanReconciliation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrInvestigationNotFound
	}
	if err != nil {
		return item, err
	}
	rows, err := r.database.QueryContext(ctx, `SELECT id,COALESCE(account_id::text,''),classification,COALESCE(currency,''),COALESCE(expected_minor::text,''),COALESCE(observed_minor::text,''),COALESCE(observed_available_minor::text,''),COALESCE(balance_version::text,''),created_at FROM reconciliation_mismatches WHERE tenant_id=$1 AND run_id=$2 ORDER BY created_at,id`, tenantID, runID)
	if err != nil {
		return item, err
	}
	defer func() { _ = rows.Close() }()
	item.Mismatches = []investigation.ReconciliationMismatch{}
	for rows.Next() {
		var mismatch investigation.ReconciliationMismatch
		if err := rows.Scan(&mismatch.ID, &mismatch.AccountID, &mismatch.Classification, &mismatch.Currency, &mismatch.ExpectedMinor, &mismatch.ObservedMinor, &mismatch.ObservedAvailableMinor, &mismatch.BalanceVersion, &mismatch.CreatedAt); err != nil {
			return item, err
		}
		mismatch.CreatedAt = mismatch.CreatedAt.UTC()
		item.Mismatches = append(item.Mismatches, mismatch)
	}
	return item, rows.Err()
}
