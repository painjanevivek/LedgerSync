package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	appapprovals "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/approvals"
)

type ApprovalRepository struct {
	database *sql.DB
}

func NewApprovalRepository(database *sql.DB) (*ApprovalRepository, error) {
	if database == nil {
		return nil, errors.New("approval database is required")
	}
	return &ApprovalRepository{database: database}, nil
}

type approvalCursor struct {
	RequestedAt time.Time
	RecordID    string
	Domain      appapprovals.Domain
	Actionable  bool
}

func (r *ApprovalRepository) List(ctx context.Context, tenantID, actorID string, query appapprovals.Query) (appapprovals.Page, error) {
	var finance bool
	if err := r.database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_subject_roles WHERE tenant_id=$1 AND subject_id=$2 AND role='finance')`, tenantID, actorID).Scan(&finance); err != nil {
		return appapprovals.Page{}, err
	}
	if !finance {
		return appapprovals.Page{}, appapprovals.ErrForbidden
	}
	cursor, err := decodeApprovalCursor(query.Cursor)
	if err != nil {
		return appapprovals.Page{}, appapprovals.ErrInvalidQuery
	}
	rows, err := r.database.QueryContext(ctx, approvalListSQL,
		tenantID,
		actorID,
		query.CanApproveFunding,
		query.CanApproveCorrections,
		string(query.Domain),
		string(query.StatusDomain),
		query.Status,
		query.Requester,
		query.Age,
		nullableTime(query.RequestedAfter),
		nullableTime(query.RequestedBefore),
		query.Now,
		query.ActionableOnly,
		nullableTime(cursor.RequestedAt),
		nullableString(cursor.RecordID),
		string(cursor.Domain),
		cursor.Actionable,
		query.Limit+1,
	)
	if err != nil {
		return appapprovals.Page{}, err
	}
	defer func() { _ = rows.Close() }()
	page := appapprovals.Page{Items: make([]appapprovals.Item, 0, query.Limit)}
	for rows.Next() {
		var item appapprovals.Item
		var requestedAt time.Time
		var expiresAt sql.NullTime
		var stepUpRequired bool
		if err := rows.Scan(
			&item.Domain,
			&item.RecordID,
			&item.RequesterSubjectID,
			&requestedAt,
			&item.AgeSeconds,
			&item.Status,
			&item.AmountMinor,
			&item.Currency,
			&item.RelatedAccountID,
			&item.RelatedTransferID,
			&item.EvidenceComplete,
			&item.SelfApprovalBlocked,
			&item.ActionableByMe,
			&stepUpRequired,
			&expiresAt,
		); err != nil {
			return appapprovals.Page{}, err
		}
		item.RequestedAt = requestedAt.UTC().Format(time.RFC3339Nano)
		if expiresAt.Valid {
			item.ApprovalExpiresAt = expiresAt.Time.UTC().Format(time.RFC3339Nano)
		}
		decorateApprovalItem(&item, stepUpRequired, appapprovals.StepUpIsRecent(query.StepUpAuthenticatedAt, query.Now))
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return appapprovals.Page{}, err
	}
	if len(page.Items) > query.Limit {
		last := page.Items[query.Limit-1]
		requestedAt, err := time.Parse(time.RFC3339Nano, last.RequestedAt)
		if err != nil {
			return appapprovals.Page{}, err
		}
		page.NextCursor = encodeApprovalCursor(approvalCursor{RequestedAt: requestedAt, RecordID: last.RecordID, Domain: last.Domain, Actionable: last.ActionableByMe})
		page.Items = page.Items[:query.Limit]
	}
	page.PageCount = len(page.Items)
	return page, nil
}

func decorateApprovalItem(item *appapprovals.Item, stepUpRequired, stepUpSatisfied bool) {
	if item.Domain == appapprovals.DomainFunding {
		item.RequiredScope = "funding:approve"
	} else {
		item.RequiredScope = "corrections:approve"
	}
	item.StepUpStatus = appapprovals.StepUpNotRequired
	if stepUpRequired {
		item.StepUpStatus = appapprovals.StepUpRequired
		if stepUpSatisfied {
			item.StepUpStatus = appapprovals.StepUpSatisfied
		}
	}
	switch {
	case item.SelfApprovalBlocked:
		item.SafeNextAction = "wait_for_independent_approver"
	case !item.EvidenceComplete:
		item.SafeNextAction = "complete_evidence"
	case item.ActionableByMe && item.StepUpStatus == appapprovals.StepUpRequired:
		item.SafeNextAction = "reauthenticate"
	case item.ActionableByMe:
		item.SafeNextAction = "review_decision"
	default:
		item.SafeNextAction = "open_record"
	}
}

func encodeApprovalCursor(cursor approvalCursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join([]string{cursor.RequestedAt.UTC().Format(time.RFC3339Nano), cursor.RecordID, string(cursor.Domain), strconv.FormatBool(cursor.Actionable)}, "|")))
}

func decodeApprovalCursor(value string) (approvalCursor, error) {
	if value == "" {
		return approvalCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return approvalCursor{}, err
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 || parts[1] == "" {
		return approvalCursor{}, errors.New("invalid approval cursor")
	}
	requestedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return approvalCursor{}, err
	}
	domain := appapprovals.Domain(parts[2])
	if domain != appapprovals.DomainFunding && domain != appapprovals.DomainCorrection {
		return approvalCursor{}, errors.New("invalid approval cursor domain")
	}
	actionable, err := strconv.ParseBool(parts[3])
	if err != nil {
		return approvalCursor{}, errors.New("invalid approval cursor actionability")
	}
	return approvalCursor{RequestedAt: requestedAt, RecordID: parts[1], Domain: domain, Actionable: actionable}, nil
}

const approvalListSQL = `
WITH approval_items AS (
 SELECT
  'funding'::text AS domain,
  funding.id::text AS record_id,
  funding.requester_subject_id,
  funding.requested_at,
  funding.status,
  funding.amount_minor::text,
  funding.currency,
  funding.destination_account_id::text AS related_account_id,
  ''::text AS related_transfer_id,
  (funding.external_reference <> '' AND funding.evidence_reference <> '') AS evidence_complete,
  (funding.requester_subject_id=$2 AND NOT funding.demo_policy) AS self_approval_blocked,
  (funding.status='requested' AND NOT (funding.requester_subject_id=$2 AND NOT funding.demo_policy) AND funding.external_reference <> '' AND funding.evidence_reference <> '') AS actionable_by_me,
  false AS step_up_required,
  NULL::timestamptz AS approval_expires_at
 FROM funding_events funding
 WHERE funding.tenant_id=$1 AND $3
 UNION ALL
 SELECT
  'correction'::text AS domain,
  correction.id::text AS record_id,
  correction.requester_subject_id,
  correction.requested_at,
  correction.status,
  original.amount_minor::text,
  original.currency,
  ''::text AS related_account_id,
  correction.original_transfer_id::text AS related_transfer_id,
  (original.journal_transaction_id IS NOT NULL AND correction.reason_code <> '' AND correction.operator_note <> '') AS evidence_complete,
  (correction.requester_subject_id=$2 AND correction.control_mode='production_dual_control') AS self_approval_blocked,
  (correction.status='requested' AND correction.approval_expires_at>$12 AND NOT (correction.requester_subject_id=$2 AND correction.control_mode='production_dual_control') AND original.journal_transaction_id IS NOT NULL AND correction.reason_code <> '' AND correction.operator_note <> '') AS actionable_by_me,
  correction.step_up_required,
  correction.approval_expires_at
 FROM transfer_corrections correction
 JOIN transfers original ON original.id=correction.original_transfer_id
 WHERE correction.tenant_id=$1 AND $4
)
SELECT domain,record_id,requester_subject_id,requested_at,
 GREATEST(0,EXTRACT(EPOCH FROM ($12-requested_at))::bigint)::text AS age_seconds,
 status,amount_minor,currency,
 related_account_id,related_transfer_id,evidence_complete,self_approval_blocked,actionable_by_me,
 step_up_required,approval_expires_at
FROM approval_items
WHERE ($5='' OR domain=$5)
 AND ($6='' OR (domain=$6 AND status=$7))
 AND ($8='' OR requester_subject_id=$8)
 AND ($9='' OR ($9='under_24h' AND requested_at>$12-INTERVAL '24 hours') OR ($9='over_24h' AND requested_at<=$12-INTERVAL '24 hours') OR ($9='over_7d' AND requested_at<=$12-INTERVAL '7 days') OR ($9='over_30d' AND requested_at<=$12-INTERVAL '30 days'))
 AND ($10::timestamptz IS NULL OR requested_at >= $10::timestamptz)
 AND ($11::timestamptz IS NULL OR requested_at <= $11::timestamptz)
	 AND (NOT $13 OR actionable_by_me)
	 AND ($14::timestamptz IS NULL OR
	      (CASE WHEN actionable_by_me THEN 0 ELSE 1 END,requested_at,record_id,domain)>
	      (CASE WHEN $17 THEN 0 ELSE 1 END,$14::timestamptz,$15,$16))
ORDER BY CASE WHEN actionable_by_me THEN 0 ELSE 1 END ASC,requested_at ASC,record_id ASC,domain ASC
LIMIT $18`
