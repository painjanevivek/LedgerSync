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

func (r *InvestigationRepository) Search(ctx context.Context, tenantID, actorID string, filter investigation.SearchFilter) (investigation.SearchPage, error) {
	lookupID := ""
	lookupReference := ""
	if filter.QueryKind == "immutable_id" {
		lookupID = strings.ToLower(filter.Query)
	} else {
		lookupReference = filter.Query
	}
	rows, err := r.database.QueryContext(ctx, `
WITH matches AS (
 SELECT 'account'::text record_type,a.id::text record_id,''::text related_record_type,''::text related_record_id,
        'Account'::text safe_label,a.status::text status,a.created_at occurred_at
 FROM accounts a
 WHERE $5 AND a.tenant_id=$1 AND a.account_kind='customer'
   AND EXISTS(SELECT 1 FROM account_owners owner WHERE owner.tenant_id=a.tenant_id AND owner.account_id=a.id AND owner.subject_id=$2 AND owner.permission IN ('read','debit'))
   AND (($3<>'' AND a.id=NULLIF($3,'')::uuid) OR ($4<>'' AND lower(a.external_reference)=lower($4)))
 UNION ALL
 SELECT 'transfer',t.id::text,'','', 'Transfer',t.status,COALESCE(t.completed_at,t.created_at)
 FROM transfers t WHERE $6 AND t.tenant_id=$1 AND $3<>'' AND t.id=NULLIF($3,'')::uuid
 UNION ALL
 SELECT 'funding',f.id::text,'account',f.destination_account_id::text,'Funding record',f.status,f.updated_at
 FROM funding_events f WHERE $7 AND f.tenant_id=$1
   AND (($3<>'' AND (f.id=NULLIF($3,'')::uuid OR f.correlation_id=NULLIF($3,'')::uuid)) OR ($4<>'' AND f.external_reference=$4))
 UNION ALL
 SELECT 'event',e.id::text,
        CASE WHEN e.transfer_id IS NOT NULL THEN 'transfer' WHEN e.account_id IS NOT NULL THEN 'account' ELSE '' END,
        COALESCE(e.transfer_id::text,e.account_id::text,''),e.event_type,
        CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.last_error_code IS NOT NULL AND e.attempt_count>0 THEN 'retrying' ELSE 'pending' END,
        e.occurred_at
 FROM outbox_events e WHERE $8 AND e.tenant_id=$1 AND $3<>'' AND e.id=NULLIF($3,'')::uuid
 UNION ALL
 SELECT 'reconciliation_run',run.id::text,'','', 'Reconciliation run',run.status,run.completed_at
 FROM reconciliation_runs run WHERE $9 AND run.tenant_id=$1
   AND $3<>'' AND (run.id=NULLIF($3,'')::uuid OR run.correlation_id=NULLIF($3,'')::uuid)
 UNION ALL
 SELECT 'reconciliation_mismatch',mismatch.id::text,'reconciliation_run',mismatch.run_id::text,
        replace(mismatch.classification,'_',' '), 'mismatch',mismatch.created_at
 FROM reconciliation_mismatches mismatch WHERE $9 AND mismatch.tenant_id=$1 AND $3<>'' AND mismatch.id=NULLIF($3,'')::uuid
 UNION ALL
 SELECT 'correction',correction.id::text,'transfer',correction.original_transfer_id::text,
        'Transfer correction',correction.status,correction.updated_at
 FROM transfer_corrections correction WHERE $10 AND correction.tenant_id=$1
   AND $3<>'' AND (correction.id=NULLIF($3,'')::uuid OR correction.correlation_id=NULLIF($3,'')::uuid)
 UNION ALL
 SELECT 'request_reference',audit.correlation_id::text,
        CASE audit.target_type WHEN 'account' THEN 'account' WHEN 'transfer' THEN 'transfer' WHEN 'funding_event' THEN 'funding' WHEN 'reconciliation_run' THEN 'reconciliation_run' WHEN 'transfer_correction' THEN 'correction' ELSE '' END,
        COALESCE(audit.target_id,''),'Request reference',
        CASE WHEN audit.outcome IN ('allowed','denied','succeeded','failed','posted','rejected') THEN audit.outcome ELSE 'recorded' END,
        audit.occurred_at
 FROM audit_events audit
 WHERE $3<>'' AND audit.tenant_id=$1 AND audit.correlation_id=NULLIF($3,'')::uuid
   AND audit.target_id IS NOT NULL
   AND (($5 AND audit.target_type='account' AND EXISTS(
          SELECT 1 FROM account_owners owner WHERE owner.tenant_id=audit.tenant_id AND owner.account_id::text=audit.target_id AND owner.subject_id=$2 AND owner.permission IN ('read','debit')
        )) OR ($6 AND audit.target_type='transfer') OR ($7 AND audit.target_type='funding_event')
		OR ($9 AND audit.target_type='reconciliation_run') OR ($10 AND audit.target_type='transfer_correction'))
)
SELECT record_type,record_id,related_record_type,related_record_id,safe_label,status,occurred_at
FROM matches
ORDER BY occurred_at DESC,record_type,record_id
LIMIT $11`, tenantID, actorID, lookupID, lookupReference, filter.Access.Accounts, filter.Access.Transfers, filter.Access.Funding, filter.Access.Events, filter.Access.Reconciliation, filter.Access.Corrections, filter.Limit*4+1)
	if err != nil {
		return investigation.SearchPage{}, fmt.Errorf("search authorized investigation evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := investigation.SearchPage{Results: make([]investigation.SearchResult, 0, filter.Limit), QueryKind: filter.QueryKind, GeneratedAt: time.Now().UTC()}
	seen := make(map[string]struct{}, filter.Limit)
	for rows.Next() {
		var item investigation.SearchResult
		if err := rows.Scan(&item.RecordType, &item.RecordID, &item.RelatedRecordType, &item.RelatedRecordID, &item.SafeLabel, &item.Status, &item.OccurredAt); err != nil {
			return investigation.SearchPage{}, fmt.Errorf("scan authorized search result: %w", err)
		}
		item.OccurredAt = item.OccurredAt.UTC()
		item.Source = "postgresql"
		item.Freshness = "search_snapshot"
		key := strings.Join([]string{item.RecordType, item.RecordID, item.RelatedRecordType, item.RelatedRecordID}, "|")
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		page.Results = append(page.Results, item)
	}
	if err := rows.Err(); err != nil {
		return investigation.SearchPage{}, fmt.Errorf("iterate authorized search results: %w", err)
	}
	if len(page.Results) > filter.Limit {
		page.Truncated = true
		page.Results = page.Results[:filter.Limit]
	}
	return page, nil
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
 t.created_at,COALESCE(t.completed_at,t.created_at),COALESCE(t.journal_transaction_id::text,''),COALESCE(t.rejection_code,''),
 COALESCE(c.id::text,''),COALESCE(c.status,''),CASE WHEN c.id IS NULL THEN '' WHEN c.original_transfer_id=t.id THEN 'original' ELSE 'compensation' END,
 COALESCE(c.original_transfer_id::text,''),COALESCE(c.compensation_transfer_id::text,''),
 COALESCE(original.journal_transaction_id::text,''),COALESCE(compensation.journal_transaction_id::text,'')
FROM transfers t
LEFT JOIN transfer_corrections c ON c.tenant_id=t.tenant_id AND (c.original_transfer_id=t.id OR c.compensation_transfer_id=t.id)
LEFT JOIN transfers original ON original.id=c.original_transfer_id
LEFT JOIN transfers compensation ON compensation.id=c.compensation_transfer_id
WHERE t.tenant_id=$1
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
		if err := rows.Scan(&item.ID, &item.DebitAccountID, &item.CreditAccountID, &item.AmountMinor, &item.Currency, &item.FinancialStatus, &item.DeliveryStatus, &item.CreatedAt, &item.CompletedAt, &item.JournalTransactionID, &item.RejectionCode, &item.CorrectionID, &item.CorrectionStatus, &item.CorrectionRole, &item.OriginalTransferID, &item.CompensationTransferID, &item.OriginalJournalID, &item.CompensationJournalID); err != nil {
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
	err := r.database.QueryRowContext(ctx, `
SELECT t.id,t.debit_account_id,t.credit_account_id,t.amount_minor,t.currency,t.status,
 CASE WHEN t.status<>'posted' THEN 'not_applicable' ELSE COALESCE((SELECT d.status FROM delivery_attempts d WHERE d.tenant_id=t.tenant_id AND d.transfer_id=t.id ORDER BY d.created_at DESC,d.id DESC LIMIT 1),'not_applicable') END,
 t.created_at,COALESCE(t.completed_at,t.created_at),COALESCE(t.journal_transaction_id::text,''),COALESCE(t.rejection_code,''),t.actor_subject_id,
 COALESCE(c.id::text,''),COALESCE(c.status,''),CASE WHEN c.id IS NULL THEN '' WHEN c.original_transfer_id=t.id THEN 'original' ELSE 'compensation' END,
 COALESCE(c.original_transfer_id::text,''),COALESCE(c.compensation_transfer_id::text,''),
 COALESCE(original.journal_transaction_id::text,''),COALESCE(compensation.journal_transaction_id::text,'')
FROM transfers t
LEFT JOIN transfer_corrections c ON c.tenant_id=t.tenant_id AND (c.original_transfer_id=t.id OR c.compensation_transfer_id=t.id)
LEFT JOIN transfers original ON original.id=c.original_transfer_id
LEFT JOIN transfers compensation ON compensation.id=c.compensation_transfer_id
WHERE t.tenant_id=$1 AND t.id=$2`, tenantID, transferID).Scan(&item.ID, &item.DebitAccountID, &item.CreditAccountID, &item.AmountMinor, &item.Currency, &item.FinancialStatus, &item.DeliveryStatus, &item.CreatedAt, &item.CompletedAt, &item.JournalTransactionID, &item.RejectionCode, &item.ActorSubjectID, &item.CorrectionID, &item.CorrectionStatus, &item.CorrectionRole, &item.OriginalTransferID, &item.CompensationTransferID, &item.OriginalJournalID, &item.CompensationJournalID)
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
	if item.CorrectionID != "" {
		item.Timeline = append(item.Timeline, investigation.EvidenceEvent{ID: item.CorrectionID, Kind: "transfer_correction_" + item.CorrectionRole, Outcome: item.CorrectionStatus, Reference: item.CompensationJournalID, OccurredAt: item.CompletedAt})
	}
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
