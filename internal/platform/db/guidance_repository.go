package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/guidance"
)

const (
	maxTimelinePostings = 4
	maxTimelineOutbox   = 8
	maxTimelineDelivery = 25
)

type GuidanceRepository struct{ database *sql.DB }

func NewGuidanceRepository(database *sql.DB) (*GuidanceRepository, error) {
	if database == nil {
		return nil, errors.New("guidance database is required")
	}
	return &GuidanceRepository{database: database}, nil
}

func (r *GuidanceRepository) Orientation(ctx context.Context, tenantID, actorID string) (guidance.OrientationFacts, error) {
	if r == nil || r.database == nil || ctx == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return guidance.OrientationFacts{}, guidance.ErrEvidenceUnavailable
	}
	var facts guidance.OrientationFacts
	var err error
	if facts.AuthorizedAccount, err = r.reference(ctx, `SELECT a.id::text,a.created_at FROM accounts a JOIN account_owners owner ON owner.tenant_id=a.tenant_id AND owner.account_id=a.id WHERE a.tenant_id=$1 AND owner.subject_id=$2 AND owner.permission IN ('read','debit') ORDER BY a.created_at DESC,a.id DESC LIMIT 1`, tenantID, actorID); err != nil {
		return facts, fmt.Errorf("read orientation account: %w", err)
	}
	if facts.CreatedAccount, err = r.reference(ctx, `SELECT a.id::text,audit.occurred_at FROM audit_events audit JOIN accounts a ON a.tenant_id=audit.tenant_id AND a.id::text=audit.target_id WHERE audit.tenant_id=$1 AND audit.actor_subject_id=$2 AND audit.event_type='account.created' AND audit.outcome='succeeded' ORDER BY audit.occurred_at DESC,audit.id DESC LIMIT 1`, tenantID, actorID); err != nil {
		return facts, fmt.Errorf("read orientation account creation: %w", err)
	}
	if facts.PostedTransfer, err = r.reference(ctx, `SELECT id::text,completed_at FROM transfers WHERE tenant_id=$1 AND actor_subject_id=$2 AND status='posted' ORDER BY completed_at DESC,id DESC LIMIT 1`, tenantID, actorID); err != nil {
		return facts, fmt.Errorf("read orientation funding: %w", err)
	}
	if facts.AuthorizedTransfer, err = r.reference(ctx, `SELECT t.id::text,COALESCE(t.completed_at,t.created_at) FROM transfers t WHERE t.tenant_id=$1 AND (t.actor_subject_id=$2 OR EXISTS(SELECT 1 FROM account_owners owner WHERE owner.tenant_id=t.tenant_id AND owner.subject_id=$2 AND owner.account_id IN(t.debit_account_id,t.credit_account_id) AND owner.permission IN ('read','debit'))) ORDER BY COALESCE(t.completed_at,t.created_at) DESC,t.id DESC LIMIT 1`, tenantID, actorID); err != nil {
		return facts, fmt.Errorf("read orientation transfer: %w", err)
	}
	if facts.ReconciliationRun, err = r.reference(ctx, `SELECT run.id::text,run.completed_at FROM reconciliation_runs run JOIN audit_events audit ON audit.tenant_id=run.tenant_id AND audit.target_id=run.id::text AND audit.event_type='reconciliation.completed' WHERE run.tenant_id=$1 AND audit.actor_subject_id=$2 ORDER BY run.completed_at DESC,run.id DESC LIMIT 1`, tenantID, actorID); err != nil {
		return facts, fmt.Errorf("read orientation reconciliation: %w", err)
	}
	if facts.DeliveryAttempt, err = r.reference(ctx, `SELECT attempt.id::text,COALESCE(attempt.completed_at,attempt.started_at,attempt.created_at) FROM delivery_attempts attempt JOIN transfers transfer ON transfer.tenant_id=attempt.tenant_id AND transfer.id=attempt.transfer_id WHERE attempt.tenant_id=$1 AND transfer.actor_subject_id=$2 ORDER BY COALESCE(attempt.completed_at,attempt.started_at,attempt.created_at) DESC,attempt.id DESC LIMIT 1`, tenantID, actorID); err != nil {
		return facts, fmt.Errorf("read orientation delivery: %w", err)
	}
	return facts, nil
}

func (r *GuidanceRepository) OrientationPreference(ctx context.Context, tenantID, actorID string) (guidance.OrientationPreference, error) {
	if r == nil || r.database == nil || ctx == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return guidance.OrientationPreference{}, guidance.ErrEvidenceUnavailable
	}
	var preference guidance.OrientationPreference
	var completedJSON []byte
	var updatedAt time.Time
	err := r.database.QueryRowContext(ctx, `SELECT dismissed,completed_step_ids,version,updated_at FROM operator_onboarding_preferences WHERE tenant_id=$1 AND subject_id=$2`, tenantID, actorID).
		Scan(&preference.Dismissed, &completedJSON, &preference.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		preference.CompletedStepIDs = []string{}
		return preference, nil
	}
	if err != nil {
		return guidance.OrientationPreference{}, fmt.Errorf("read orientation preference: %w", err)
	}
	if err := json.Unmarshal(completedJSON, &preference.CompletedStepIDs); err != nil {
		return guidance.OrientationPreference{}, fmt.Errorf("decode orientation preference: %w", err)
	}
	updatedAt = updatedAt.UTC()
	preference.UpdatedAt = &updatedAt
	return preference, nil
}

func (r *GuidanceRepository) UpdateOrientationPreference(ctx context.Context, tenantID, actorID string, update guidance.PreferenceUpdate) (guidance.OrientationPreference, error) {
	if r == nil || r.database == nil || ctx == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" || update.ExpectedVersion < 0 {
		return guidance.OrientationPreference{}, guidance.ErrInvalidPreference
	}
	completedJSON, err := json.Marshal(update.CompletedStepIDs)
	if err != nil {
		return guidance.OrientationPreference{}, guidance.ErrInvalidPreference
	}
	var version int64
	var updatedAt time.Time
	if update.ExpectedVersion == 0 {
		err = r.database.QueryRowContext(ctx, `INSERT INTO operator_onboarding_preferences(tenant_id,subject_id,dismissed,completed_step_ids,version,updated_at) VALUES($1,$2,$3,$4::jsonb,1,now()) ON CONFLICT (tenant_id,subject_id) DO NOTHING RETURNING version,updated_at`, tenantID, actorID, update.Dismissed, string(completedJSON)).Scan(&version, &updatedAt)
	} else {
		err = r.database.QueryRowContext(ctx, `UPDATE operator_onboarding_preferences SET dismissed=$3,completed_step_ids=$4::jsonb,version=version+1,updated_at=now() WHERE tenant_id=$1 AND subject_id=$2 AND version=$5 RETURNING version,updated_at`, tenantID, actorID, update.Dismissed, string(completedJSON), update.ExpectedVersion).Scan(&version, &updatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return guidance.OrientationPreference{}, guidance.ErrPreferenceConflict
	}
	if err != nil {
		return guidance.OrientationPreference{}, fmt.Errorf("persist orientation preference: %w", err)
	}
	updatedAt = updatedAt.UTC()
	return guidance.OrientationPreference{
		Dismissed:        update.Dismissed,
		CompletedStepIDs: append([]string(nil), update.CompletedStepIDs...),
		Version:          version,
		UpdatedAt:        &updatedAt,
	}, nil
}

func (r *GuidanceRepository) reference(ctx context.Context, statement string, arguments ...any) (*guidance.DurableReference, error) {
	var reference guidance.DurableReference
	err := r.database.QueryRowContext(ctx, statement, arguments...).Scan(&reference.ID, &reference.OccurredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	reference.OccurredAt = reference.OccurredAt.UTC()
	return &reference, nil
}

func (r *GuidanceRepository) ExplainTransfer(ctx context.Context, tenantID, actorID, transferID string) (guidance.TransferFacts, error) {
	facts := guidance.TransferFacts{TransferID: transferID}
	var status, amountMinor, currency, journalID string
	var createdAt time.Time
	var completedAt sql.NullTime
	err := r.database.QueryRowContext(ctx, `
SELECT t.status,t.amount_minor::text,t.currency,t.created_at,t.completed_at,COALESCE(t.journal_transaction_id::text,'')
FROM transfers t
WHERE t.tenant_id=$1 AND t.id=$2 AND (
 t.actor_subject_id=$3
 OR EXISTS(SELECT 1 FROM account_owners owner WHERE owner.tenant_id=t.tenant_id AND owner.subject_id=$3 AND owner.account_id IN(t.debit_account_id,t.credit_account_id) AND owner.permission IN ('read','debit'))
 OR EXISTS(SELECT 1 FROM tenant_subject_roles role WHERE role.tenant_id=t.tenant_id AND role.subject_id=$3 AND role.role IN ('operator','finance'))
)`, tenantID, transferID, actorID).Scan(&status, &amountMinor, &currency, &createdAt, &completedAt, &journalID)
	if errors.Is(err, sql.ErrNoRows) {
		return facts, guidance.ErrTransferNotFound
	}
	if err != nil {
		return facts, fmt.Errorf("read explainable transfer: %w", err)
	}
	transferAt := createdAt.UTC()
	if completedAt.Valid {
		transferAt = completedAt.Time.UTC()
	}
	facts.Transfer.Items = []guidance.EvidenceItem{{EvidenceType: "transfer", EvidenceID: transferID, Status: allowedGuidanceStatus(status), AmountMinor: amountMinor, Currency: currency, OccurredAt: guidanceTime(transferAt)}}

	facts.Request = r.requestEvidence(ctx, tenantID, transferID)
	facts.JournalPostings = r.journalEvidence(ctx, tenantID, transferID, journalID)
	facts.Outbox, facts.BalanceVersions = r.outboxEvidence(ctx, tenantID, transferID)
	facts.Delivery = r.deliveryEvidence(ctx, tenantID, transferID)
	facts.Reconciliation = r.reconciliationEvidence(ctx, tenantID, transferID)
	return facts, nil
}

func (r *GuidanceRepository) requestEvidence(ctx context.Context, tenantID, transferID string) guidance.EvidenceLink {
	rows, err := r.database.QueryContext(ctx, `SELECT state,created_at,completed_at FROM idempotency_requests WHERE tenant_id=$1 AND transfer_id=$2 ORDER BY COALESCE(completed_at,created_at),operation LIMIT 3`, tenantID, transferID)
	if err != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	defer func() { _ = rows.Close() }()
	link := guidance.EvidenceLink{Items: []guidance.EvidenceItem{}}
	for rows.Next() {
		var status string
		var created time.Time
		var completed sql.NullTime
		if rows.Scan(&status, &created, &completed) != nil {
			return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
		}
		occurred := created.UTC()
		if completed.Valid {
			occurred = completed.Time.UTC()
		}
		link.Items = append(link.Items, guidance.EvidenceItem{EvidenceType: "idempotency_outcome", RelatedID: transferID, Status: allowedGuidanceStatus(status), OccurredAt: guidanceTime(occurred)})
	}
	if rows.Err() != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	if len(link.Items) > 2 {
		link.Items, link.Truncated = link.Items[:2], true
	}
	return link
}

func (r *GuidanceRepository) journalEvidence(ctx context.Context, tenantID, transferID, journalID string) guidance.EvidenceLink {
	link := guidance.EvidenceLink{Items: []guidance.EvidenceItem{}}
	if journalID == "" {
		return link
	}
	var occurred time.Time
	if err := r.database.QueryRowContext(ctx, `SELECT occurred_at FROM journal_transactions WHERE tenant_id=$1 AND transfer_id=$2 AND id=$3`, tenantID, transferID, journalID).Scan(&occurred); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return link
		}
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	link.Items = append(link.Items, guidance.EvidenceItem{EvidenceType: "journal", EvidenceID: journalID, RelatedID: transferID, OccurredAt: guidanceTime(occurred)})
	rows, err := r.database.QueryContext(ctx, `SELECT p.id::text,p.account_id::text,p.direction,p.amount_minor::text,p.currency,p.occurred_at FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE j.tenant_id=$1 AND j.transfer_id=$2 ORDER BY p.occurred_at,p.id LIMIT $3`, tenantID, transferID, maxTimelinePostings+1)
	if err != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item guidance.EvidenceItem
		var itemTime time.Time
		item.EvidenceType = "posting"
		if rows.Scan(&item.EvidenceID, &item.AccountID, &item.Direction, &item.AmountMinor, &item.Currency, &itemTime) != nil {
			return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
		}
		item.Status, item.Direction, item.RelatedID, item.OccurredAt = allowedGuidanceStatus(item.Direction), allowedGuidanceStatus(item.Direction), journalID, guidanceTime(itemTime)
		link.Items = append(link.Items, item)
	}
	if rows.Err() != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	if len(link.Items) > maxTimelinePostings+1 {
		link.Items, link.Truncated = link.Items[:maxTimelinePostings+1], true
	}
	return link
}

func (r *GuidanceRepository) outboxEvidence(ctx context.Context, tenantID, transferID string) (guidance.EvidenceLink, guidance.EvidenceLink) {
	rows, err := r.database.QueryContext(ctx, `
SELECT id::text,event_type,account_id::text,aggregate_version::text,
 CASE WHEN published_at IS NOT NULL THEN 'published' WHEN dead_at IS NOT NULL THEN 'dead' WHEN last_error_code IS NOT NULL AND attempt_count>0 THEN 'retrying' ELSE 'pending' END,
 occurred_at
FROM outbox_events WHERE tenant_id=$1 AND transfer_id=$2 ORDER BY occurred_at,id LIMIT $3`, tenantID, transferID, maxTimelineOutbox+1)
	if err != nil {
		unavailable := guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
		return unavailable, unavailable
	}
	defer func() { _ = rows.Close() }()
	outbox := guidance.EvidenceLink{Items: []guidance.EvidenceItem{}}
	versions := guidance.EvidenceLink{Items: []guidance.EvidenceItem{}}
	for rows.Next() {
		var id, eventType, accountID, version, status string
		var occurred time.Time
		if rows.Scan(&id, &eventType, &accountID, &version, &status, &occurred) != nil {
			unavailable := guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
			return unavailable, unavailable
		}
		outbox.Items = append(outbox.Items, guidance.EvidenceItem{EvidenceType: "outbox_event", EvidenceID: id, RelatedID: transferID, Status: allowedGuidanceStatus(status), EventType: allowedGuidanceEventType(eventType), AccountID: accountID, BalanceVersion: version, OccurredAt: guidanceTime(occurred)})
		if eventType == "account.balance.changed.v1" {
			versions.Items = append(versions.Items, guidance.EvidenceItem{EvidenceType: "balance_version", EvidenceID: id, RelatedID: transferID, AccountID: accountID, BalanceVersion: version, OccurredAt: guidanceTime(occurred)})
		}
	}
	if rows.Err() != nil {
		unavailable := guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
		return unavailable, unavailable
	}
	if len(outbox.Items) > maxTimelineOutbox {
		outbox.Items, outbox.Truncated = outbox.Items[:maxTimelineOutbox], true
		versions.Truncated = true
	}
	if len(versions.Items) > maxTimelinePostings {
		versions.Items, versions.Truncated = versions.Items[:maxTimelinePostings], true
	}
	return outbox, versions
}

func (r *GuidanceRepository) deliveryEvidence(ctx context.Context, tenantID, transferID string) guidance.EvidenceLink {
	rows, err := r.database.QueryContext(ctx, `
SELECT attempt.id::text,COALESCE(event.id::text,''),attempt.status,attempt.attempt_number::text,COALESCE(attempt.completed_at,attempt.started_at,attempt.created_at)
FROM delivery_attempts attempt
LEFT JOIN outbox_events event ON event.id=attempt.outbox_event_id AND event.tenant_id=attempt.tenant_id AND event.transfer_id=attempt.transfer_id
WHERE attempt.tenant_id=$1 AND attempt.transfer_id=$2
ORDER BY attempt.created_at,attempt.id LIMIT $3`, tenantID, transferID, maxTimelineDelivery+1)
	if err != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	defer func() { _ = rows.Close() }()
	link := guidance.EvidenceLink{Items: []guidance.EvidenceItem{}}
	for rows.Next() {
		var item guidance.EvidenceItem
		var occurred time.Time
		item.EvidenceType = "delivery_attempt"
		if rows.Scan(&item.EvidenceID, &item.RelatedID, &item.Status, &item.AttemptNumber, &occurred) != nil {
			return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
		}
		item.Status, item.OccurredAt = allowedGuidanceStatus(item.Status), guidanceTime(occurred)
		link.Items = append(link.Items, item)
	}
	if rows.Err() != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	if len(link.Items) > maxTimelineDelivery {
		link.Items, link.Truncated = link.Items[:maxTimelineDelivery], true
	}
	return link
}

func (r *GuidanceRepository) reconciliationEvidence(ctx context.Context, tenantID, transferID string) guidance.EvidenceLink {
	var id, status string
	var completed time.Time
	err := r.database.QueryRowContext(ctx, `
SELECT run.id::text,run.status,run.completed_at
FROM reconciliation_runs run
JOIN transfers transfer ON transfer.tenant_id=run.tenant_id AND transfer.id=$2
JOIN journal_transactions journal ON journal.tenant_id=transfer.tenant_id AND journal.transfer_id=transfer.id
WHERE run.tenant_id=$1 AND CASE
 WHEN run.ledger_watermark ~ '^[0-9]+:[0-9]+:(?:[0-9]+(?:,[0-9]+)*)?$' THEN
  pg_visible_in_snapshot(transfer.xmin::text::xid8,run.ledger_watermark::pg_snapshot)
  AND pg_visible_in_snapshot(journal.xmin::text::xid8,run.ledger_watermark::pg_snapshot)
  AND NOT EXISTS(
    SELECT 1 FROM ledger_postings posting WHERE posting.journal_transaction_id=journal.id
      AND NOT pg_visible_in_snapshot(posting.xmin::text::xid8,run.ledger_watermark::pg_snapshot)
  )
 ELSE false END
ORDER BY run.completed_at DESC,run.id DESC LIMIT 1`, tenantID, transferID).Scan(&id, &status, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return guidance.EvidenceLink{Items: []guidance.EvidenceItem{}}
	}
	if err != nil {
		return guidance.EvidenceLink{Unavailable: true, Items: []guidance.EvidenceItem{}}
	}
	return guidance.EvidenceLink{Items: []guidance.EvidenceItem{{EvidenceType: "reconciliation_run", EvidenceID: id, RelatedID: transferID, Status: allowedGuidanceStatus(status), OccurredAt: guidanceTime(completed)}}}
}

func allowedGuidanceStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, allowed := range []string{"in_progress", "completed", "failed", "pending", "posted", "rejected", "debit", "credit", "published", "retrying", "dead", "delivered", "matched", "mismatch"} {
		if value == allowed {
			return value
		}
	}
	return ""
}

func allowedGuidanceEventType(value string) string {
	if strings.TrimSpace(value) == "account.balance.changed.v1" {
		return "account.balance.changed.v1"
	}
	return ""
}

func guidanceTime(value time.Time) *time.Time {
	result := value.UTC()
	return &result
}
