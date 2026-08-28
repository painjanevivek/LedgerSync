package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
)

var safeEvidenceCode = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
var safeEvidenceEventType = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
var safeEvidenceUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type OperationsRepository struct{ database *sql.DB }

func NewOperationsRepository(database *sql.DB) (*OperationsRepository, error) {
	if database == nil {
		return nil, errors.New("operations database is required")
	}
	return &OperationsRepository{database: database}, nil
}

func (r *OperationsRepository) Facts(ctx context.Context, tenantID string) (operations.DatabaseFacts, error) {
	var facts operations.DatabaseFacts
	if err := r.database.PingContext(ctx); err != nil {
		return facts, fmt.Errorf("ping diagnostics database: %w", err)
	}
	var published, oldestPending, reconciled sql.NullTime
	var runID, reconciliationStatus sql.NullString
	err := r.database.QueryRowContext(ctx, `
SELECT COALESCE((SELECT max(version) FROM schema_migrations),'unknown'),
  count(*) FILTER (WHERE published_at IS NULL AND dead_at IS NULL),
  count(*) FILTER (WHERE dead_at IS NOT NULL),
  max(published_at),min(created_at) FILTER (WHERE published_at IS NULL AND dead_at IS NULL),
  (SELECT id::text FROM reconciliation_runs WHERE tenant_id=$1 ORDER BY completed_at DESC,id DESC LIMIT 1),
  (SELECT status FROM reconciliation_runs WHERE tenant_id=$1 ORDER BY completed_at DESC,id DESC LIMIT 1),
  (SELECT completed_at FROM reconciliation_runs WHERE tenant_id=$1 ORDER BY completed_at DESC,id DESC LIMIT 1)
FROM outbox_events WHERE tenant_id=$1`, tenantID).Scan(&facts.SchemaVersion, &facts.PendingOutboxCount, &facts.DeadOutboxCount, &published, &oldestPending, &runID, &reconciliationStatus, &reconciled)
	if err != nil {
		return facts, fmt.Errorf("read approved diagnostic facts: %w", err)
	}
	facts.LatestPublishedAt = utcNullTime(published)
	facts.OldestPendingAt = utcNullTime(oldestPending)
	facts.ReconciliationID, facts.ReconciliationStatus = runID.String, safeCode(reconciliationStatus.String)
	facts.ReconciledAt = utcNullTime(reconciled)
	return facts, nil
}

type eventCursor struct {
	At          time.Time `json:"at"`
	ID          string    `json:"id"`
	Fingerprint string    `json:"fp"`
}

func (r *OperationsRepository) ListEvents(ctx context.Context, tenantID, actorID string, filter operations.EventFilter) ([]operations.EventEvidence, string, error) {
	fingerprint := eventFilterFingerprint(filter)
	cursor, err := decodeEventCursor(filter.Cursor, fingerprint)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.database.QueryContext(ctx, `
SELECT e.id::text,e.event_type,
 CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.last_error_code IS NOT NULL AND e.attempt_count>0 THEN 'retrying' ELSE 'pending' END,
 e.aggregate_type,e.aggregate_id::text,e.aggregate_version::text,e.attempt_count::text,
 COALESCE(e.transfer_id::text,''),COALESCE(e.account_id::text,''),COALESCE(transfer_audit.correlation_id::text,''),COALESCE(e.last_error_code,''),
 e.occurred_at,e.available_at,e.claimed_until,e.published_at,e.dead_at
FROM outbox_events e
LEFT JOIN transfers t ON t.id=e.transfer_id AND t.tenant_id=e.tenant_id
LEFT JOIN accounts a ON a.id=e.account_id AND a.tenant_id=e.tenant_id
LEFT JOIN LATERAL (
 SELECT audit.correlation_id
 FROM audit_events audit
 WHERE audit.tenant_id=e.tenant_id AND audit.target_id=e.transfer_id::text AND audit.event_type='transfer.posted'
 ORDER BY audit.occurred_at DESC,audit.id DESC LIMIT 1
) transfer_audit ON e.transfer_id IS NOT NULL
WHERE e.tenant_id=$1
 AND EXISTS(SELECT 1 FROM tenant_subject_roles role WHERE role.tenant_id=e.tenant_id AND role.subject_id=$11 AND role.role IN ('operator','finance'))
 AND (e.transfer_id IS NULL OR t.id IS NOT NULL) AND (e.account_id IS NULL OR a.id IS NOT NULL)
 AND ($2='' OR e.event_type=$2)
 AND ($3='' OR CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.last_error_code IS NOT NULL AND e.attempt_count>0 THEN 'retrying' ELSE 'pending' END=$3)
 AND ($4='' OR e.transfer_id::text=$4 OR e.account_id::text=$4)
 AND ($5='' OR transfer_audit.correlation_id::text=$5)
 AND ($6::timestamptz IS NULL OR e.occurred_at >= $6) AND ($7::timestamptz IS NULL OR e.occurred_at <= $7)
 AND ($8::timestamptz IS NULL OR (e.occurred_at,e.id)<($8::timestamptz,$9::uuid))
ORDER BY e.occurred_at DESC,e.id DESC LIMIT $10`, tenantID, filter.EventType, filter.State, filter.RelatedID, filter.CorrelationID, nullableTime(filter.From), nullableTime(filter.To), nullableTime(cursor.At), nullableString(cursor.ID), filter.Limit+1, actorID)
	if err != nil {
		return nil, "", fmt.Errorf("list event evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]operations.EventEvidence, 0, filter.Limit)
	for rows.Next() {
		item, err := scanEventEvidence(rows)
		if err != nil {
			return nil, "", err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > filter.Limit {
		last := items[filter.Limit-1]
		next = encodeEventCursor(eventCursor{At: last.OccurredAt, ID: last.EventID, Fingerprint: fingerprint})
		items = items[:filter.Limit]
	}
	return items, next, nil
}

func (r *OperationsRepository) GetEvent(ctx context.Context, tenantID, actorID, eventID string) (operations.EventDetail, error) {
	row := r.database.QueryRowContext(ctx, `
SELECT e.id::text,e.event_type,
 CASE WHEN e.published_at IS NOT NULL THEN 'published' WHEN e.dead_at IS NOT NULL THEN 'dead' WHEN e.last_error_code IS NOT NULL AND e.attempt_count>0 THEN 'retrying' ELSE 'pending' END,
 e.aggregate_type,e.aggregate_id::text,e.aggregate_version::text,e.attempt_count::text,
 COALESCE(e.transfer_id::text,''),COALESCE(e.account_id::text,''),COALESCE(transfer_audit.correlation_id::text,''),COALESCE(e.last_error_code,''),
 e.occurred_at,e.available_at,e.claimed_until,e.published_at,e.dead_at
FROM outbox_events e
LEFT JOIN transfers t ON t.id=e.transfer_id AND t.tenant_id=e.tenant_id
LEFT JOIN accounts a ON a.id=e.account_id AND a.tenant_id=e.tenant_id
LEFT JOIN LATERAL (
 SELECT audit.correlation_id
 FROM audit_events audit
 WHERE audit.tenant_id=e.tenant_id AND audit.target_id=e.transfer_id::text AND audit.event_type='transfer.posted'
 ORDER BY audit.occurred_at DESC,audit.id DESC LIMIT 1
) transfer_audit ON e.transfer_id IS NOT NULL
WHERE e.tenant_id=$1 AND e.id=$2 AND (e.transfer_id IS NULL OR t.id IS NOT NULL) AND (e.account_id IS NULL OR a.id IS NOT NULL)
 AND EXISTS(SELECT 1 FROM tenant_subject_roles role WHERE role.tenant_id=e.tenant_id AND role.subject_id=$3 AND role.role IN ('operator','finance'))`, tenantID, eventID, actorID)
	item, err := scanEventEvidence(row)
	if errors.Is(err, sql.ErrNoRows) {
		return operations.EventDetail{}, ErrInvestigationNotFound
	}
	if err != nil {
		return operations.EventDetail{}, err
	}
	detail := operations.EventDetail{EventEvidence: item, DeliveryAttempts: []operations.DeliveryEvidence{}, Timeline: eventTimeline(item)}
	rows, err := r.database.QueryContext(ctx, `SELECT id::text,delivery_kind,status,attempt_number::text,COALESCE(response_class,''),COALESCE(sanitized_error_code,''),due_at,started_at,completed_at FROM delivery_attempts WHERE tenant_id=$1 AND outbox_event_id=$2 ORDER BY attempt_number DESC,id DESC LIMIT 26`, tenantID, eventID)
	if err != nil {
		return operations.EventDetail{}, fmt.Errorf("read event delivery evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var attempt operations.DeliveryEvidence
		var started, completed sql.NullTime
		if err := rows.Scan(&attempt.AttemptID, &attempt.Kind, &attempt.State, &attempt.AttemptNumber, &attempt.ResponseClass, &attempt.ErrorCode, &attempt.DueAt, &started, &completed); err != nil {
			return operations.EventDetail{}, err
		}
		attempt.Kind = allowedEvidenceValue(attempt.Kind, "unknown", "webhook", "notification")
		attempt.State = allowedEvidenceValue(attempt.State, "unknown", "pending", "retrying", "delivered", "dead")
		attempt.ResponseClass = allowedEvidenceValue(attempt.ResponseClass, "redacted", "", "2xx", "3xx", "4xx", "5xx", "network_error", "timeout")
		attempt.ErrorCode = allowedEvidenceValue(attempt.ErrorCode, "redacted", "", "timeout", "publish_failed", "invalid_event", "redis_unavailable", "recipient_unavailable", "connection_failed")
		attempt.DueAt, attempt.StartedAt, attempt.CompletedAt = attempt.DueAt.UTC(), optionalDatabaseTime(started), optionalDatabaseTime(completed)
		detail.DeliveryAttempts = append(detail.DeliveryAttempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return operations.EventDetail{}, err
	}
	if len(detail.DeliveryAttempts) > 25 {
		detail.DeliveryAttemptsTruncated = true
		detail.DeliveryAttempts = detail.DeliveryAttempts[:25]
	}
	for left, right := 0, len(detail.DeliveryAttempts)-1; left < right; left, right = left+1, right-1 {
		detail.DeliveryAttempts[left], detail.DeliveryAttempts[right] = detail.DeliveryAttempts[right], detail.DeliveryAttempts[left]
	}
	for _, attempt := range detail.DeliveryAttempts {
		if attempt.StartedAt != nil {
			appendBoundedTimeline(&detail.Timeline, operations.EventTimelineItem{Kind: "delivery_started", OccurredAt: *attempt.StartedAt})
		}
		if attempt.CompletedAt != nil {
			appendBoundedTimeline(&detail.Timeline, operations.EventTimelineItem{Kind: "delivery_" + attempt.State, OccurredAt: *attempt.CompletedAt})
		}
	}
	return detail, nil
}

func appendBoundedTimeline(timeline *[]operations.EventTimelineItem, item operations.EventTimelineItem) {
	if len(*timeline) < 32 {
		*timeline = append(*timeline, item)
	}
}

func scanEventEvidence(row rowScanner) (operations.EventEvidence, error) {
	var item operations.EventEvidence
	var claimed, published, dead sql.NullTime
	if err := row.Scan(&item.EventID, &item.EventType, &item.State, &item.AggregateType, &item.AggregateID, &item.AggregateVersion, &item.AttemptCount, &item.TransferID, &item.AccountID, &item.CorrelationID, &item.LastErrorCode, &item.OccurredAt, &item.AvailableAt, &claimed, &published, &dead); err != nil {
		return item, err
	}
	item.EventType = safeEventType(item.EventType)
	item.State = allowedEvidenceValue(item.State, "unknown", "pending", "retrying", "published", "dead")
	item.AggregateType = safeCode(item.AggregateType)
	item.LastErrorCode = allowedEvidenceValue(item.LastErrorCode, "redacted", "", "publish_failed", "invalid_event", "redis_unavailable", "approved_replay")
	item.OccurredAt, item.AvailableAt = item.OccurredAt.UTC(), item.AvailableAt.UTC()
	item.ClaimedUntil, item.PublishedAt, item.DeadAt = optionalDatabaseTime(claimed), optionalDatabaseTime(published), optionalDatabaseTime(dead)
	return item, nil
}

func eventTimeline(item operations.EventEvidence) []operations.EventTimelineItem {
	timeline := []operations.EventTimelineItem{{Kind: "committed", OccurredAt: item.OccurredAt}}
	if item.AvailableAt.After(item.OccurredAt) {
		timeline = append(timeline, operations.EventTimelineItem{Kind: "available", OccurredAt: item.AvailableAt})
	}
	if item.ClaimedUntil != nil {
		timeline = append(timeline, operations.EventTimelineItem{Kind: "claim_lease_expires", OccurredAt: *item.ClaimedUntil})
	}
	if item.PublishedAt != nil {
		timeline = append(timeline, operations.EventTimelineItem{Kind: "published", OccurredAt: *item.PublishedAt})
	}
	if item.DeadAt != nil {
		timeline = append(timeline, operations.EventTimelineItem{Kind: "dead", OccurredAt: *item.DeadAt})
	}
	return timeline
}

func eventFilterFingerprint(filter operations.EventFilter) string {
	canonical := strings.Join([]string{filter.EventType, filter.State, filter.RelatedID, filter.CorrelationID, filter.From.UTC().Format(time.RFC3339Nano), filter.To.UTC().Format(time.RFC3339Nano), strconv.Itoa(filter.Limit)}, "\x00")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func encodeEventCursor(cursor eventCursor) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeEventCursor(raw, fingerprint string) (eventCursor, error) {
	if raw == "" {
		return eventCursor{}, nil
	}
	if len(raw) > 768 {
		return eventCursor{}, errors.New("invalid event cursor")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) > 512 {
		return eventCursor{}, errors.New("invalid event cursor")
	}
	var cursor eventCursor
	if json.Unmarshal(decoded, &cursor) != nil || cursor.At.IsZero() || !safeEvidenceUUID.MatchString(strings.ToLower(cursor.ID)) || cursor.Fingerprint != fingerprint {
		return eventCursor{}, errors.New("invalid event cursor")
	}
	return cursor, nil
}

func safeCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if !safeEvidenceCode.MatchString(value) {
		return "redacted"
	}
	return value
}

func safeEventType(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 || !safeEvidenceEventType.MatchString(value) {
		return "redacted_event"
	}
	return value
}

func allowedEvidenceValue(value, fallback string, allowed ...string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func utcNullTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

func optionalDatabaseTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
