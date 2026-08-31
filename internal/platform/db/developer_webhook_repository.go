package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
)

type DeveloperWebhookRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewDeveloperWebhookRepository(database *sql.DB, clock func() time.Time) (*DeveloperWebhookRepository, error) {
	if database == nil {
		return nil, errors.New("developer webhook database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &DeveloperWebhookRepository{database: database, clock: clock}, nil
}

func (r *DeveloperWebhookRepository) RegisterWebhook(ctx context.Context, command developerplatform.RegisterWebhookCommand, fingerprint [sha256.Size]byte) (submission developerplatform.WebhookSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-webhook-register|"+command.TenantID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RegisterWebhookOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		id, err := newUUID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO developer_webhook_endpoints(id,tenant_id,display_name,endpoint_url,subscribed_events,signing_key_reference,signing_key_id,status,version,challenge_digest,challenge_expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,'pending_verification',1,$8,$9,$10,$10)`, id, command.TenantID, command.DisplayName, command.EndpointURL, command.SubscribedEvents, command.SigningKeyReference, command.SigningKeyID, command.ChallengeDigest[:], command.ChallengeExpiresAt, now)
		if err != nil {
			return classifyDeveloperWebhookError(err)
		}
		expires := command.ChallengeExpiresAt
		submission.Webhook = developerplatform.Webhook{ID: id, DisplayName: command.DisplayName, EndpointURL: command.EndpointURL, SubscribedEvents: append([]string(nil), command.SubscribedEvents...), SigningKeyReference: command.SigningKeyReference, SigningKeyID: command.SigningKeyID, Status: "pending_verification", Version: "1", ChallengeExpiresAt: &expires, CreatedAt: now, UpdatedAt: now}
		if err := insertWebhookEvent(ctx, tx, command.TenantID, id, "registered", 1, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"endpoint_url": command.EndpointURL, "subscribed_events": command.SubscribedEvents, "signing_key_id": command.SigningKeyID}); err != nil {
			return err
		}
		verificationID, err := newUUID()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO webhook_endpoint_verification_jobs(id,tenant_id,webhook_id,challenge,expires_at,available_at,correlation_id,actor_subject_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$6,$6)`, verificationID, command.TenantID, id, []byte(command.VerificationChallenge), command.ChallengeExpiresAt, now, command.CorrelationID, command.ActorSubjectID); err != nil {
			return classifyDeveloperWebhookError(err)
		}
		if err := insertWebhookEvent(ctx, tx, command.TenantID, id, "verification_scheduled", 1, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"verification_expires_at": command.ChallengeExpiresAt}); err != nil {
			return err
		}
		return completeWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RegisterWebhookOperation, command.IdempotencyKey, submission.Webhook, now)
	})
	return submission, err
}

func (r *DeveloperWebhookRepository) VerifyWebhook(ctx context.Context, command developerplatform.VerifyWebhookCommand, fingerprint, challengeDigest [sha256.Size]byte) (submission developerplatform.WebhookSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-webhook-verify|"+command.TenantID+"|"+command.WebhookID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.VerifyWebhookOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		current, storedDigest, err := lockWebhook(ctx, tx, command.TenantID, command.WebhookID)
		if err != nil {
			return err
		}
		version, _ := strconv.ParseInt(current.Version, 10, 64)
		if current.Status != "pending_verification" || current.ChallengeExpiresAt == nil || !now.Before(*current.ChallengeExpiresAt) || subtle.ConstantTimeCompare(storedDigest, challengeDigest[:]) != 1 {
			return developerplatform.ErrConflict
		}
		if version != command.ExpectedVersion {
			return developerplatform.ErrVersionConflict
		}
		newVersion := version + 1
		result, err := tx.ExecContext(ctx, `UPDATE developer_webhook_endpoints SET status='active',version=$4,challenge_digest=NULL,challenge_expires_at=NULL,verified_at=$5,updated_at=$5 WHERE tenant_id=$1 AND id=$2 AND version=$3 AND status='pending_verification'`, command.TenantID, command.WebhookID, version, newVersion, now)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return developerplatform.ErrVersionConflict
		}
		current.Status, current.Version, current.ChallengeExpiresAt, current.VerifiedAt, current.UpdatedAt = "active", strconv.FormatInt(newVersion, 10), nil, &now, now
		submission.Webhook = current
		if err := insertWebhookEvent(ctx, tx, command.TenantID, command.WebhookID, "verified", newVersion, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"verified_at": now}); err != nil {
			return err
		}
		return completeWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.VerifyWebhookOperation, command.IdempotencyKey, current, now)
	})
	return submission, err
}

func (r *DeveloperWebhookRepository) RotateWebhook(ctx context.Context, command developerplatform.RotateWebhookCommand, fingerprint [sha256.Size]byte) (submission developerplatform.WebhookSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-webhook-rotate|"+command.TenantID+"|"+command.WebhookID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RotateWebhookOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		current, _, err := lockWebhook(ctx, tx, command.TenantID, command.WebhookID)
		if err != nil {
			return err
		}
		version, _ := strconv.ParseInt(current.Version, 10, 64)
		if current.Status != "active" {
			return developerplatform.ErrConflict
		}
		if version != command.ExpectedVersion {
			return developerplatform.ErrVersionConflict
		}
		newVersion := version + 1
		previousKeyID := current.SigningKeyID
		result, err := tx.ExecContext(ctx, `UPDATE developer_webhook_endpoints SET signing_key_reference=$4,signing_key_id=$5,version=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2 AND version=$3 AND status='active'`, command.TenantID, command.WebhookID, version, command.SigningKeyReference, command.SigningKeyID, newVersion, now)
		if err != nil {
			return classifyDeveloperWebhookError(err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return developerplatform.ErrVersionConflict
		}
		current.SigningKeyReference, current.SigningKeyID, current.Version, current.UpdatedAt = command.SigningKeyReference, command.SigningKeyID, strconv.FormatInt(newVersion, 10), now
		submission.Webhook = current
		if err := insertWebhookEvent(ctx, tx, command.TenantID, command.WebhookID, "signature_rotated", newVersion, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"previous_signing_key_id": previousKeyID, "signing_key_id": command.SigningKeyID}); err != nil {
			return err
		}
		return completeWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RotateWebhookOperation, command.IdempotencyKey, current, now)
	})
	return submission, err
}

func (r *DeveloperWebhookRepository) DisableWebhook(ctx context.Context, command developerplatform.DisableWebhookCommand, fingerprint [sha256.Size]byte) (submission developerplatform.WebhookSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-webhook-disable|"+command.TenantID+"|"+command.WebhookID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.DisableWebhookOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		current, _, err := lockWebhook(ctx, tx, command.TenantID, command.WebhookID)
		if err != nil {
			return err
		}
		version, _ := strconv.ParseInt(current.Version, 10, 64)
		if current.Status == "disabled" {
			return developerplatform.ErrConflict
		}
		if version != command.ExpectedVersion {
			return developerplatform.ErrVersionConflict
		}
		newVersion := version + 1
		result, err := tx.ExecContext(ctx, `UPDATE developer_webhook_endpoints SET status='disabled',version=$4,challenge_digest=NULL,challenge_expires_at=NULL,disabled_at=$5,updated_at=$5 WHERE tenant_id=$1 AND id=$2 AND version=$3 AND status<>'disabled'`, command.TenantID, command.WebhookID, version, newVersion, now)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return developerplatform.ErrVersionConflict
		}
		current.Status, current.Version, current.ChallengeExpiresAt, current.DisabledAt, current.UpdatedAt = "disabled", strconv.FormatInt(newVersion, 10), nil, &now, now
		submission.Webhook = current
		if err := insertWebhookEvent(ctx, tx, command.TenantID, command.WebhookID, "disabled", newVersion, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"reason": command.Reason}); err != nil {
			return err
		}
		return completeWebhookCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.DisableWebhookOperation, command.IdempotencyKey, current, now)
	})
	return submission, err
}

// PostgreSQL arrays are not portable through every database/sql driver. The
// repository projects the approved subscription list to JSON at the boundary,
// then validates it before exposing it to the application.
const webhookSelect = `SELECT id::text,display_name,endpoint_url,array_to_json(subscribed_events)::text,signing_key_reference,signing_key_id,status,version::text,challenge_expires_at,verified_at,disabled_at,created_at,updated_at FROM developer_webhook_endpoints`

func (r *DeveloperWebhookRepository) GetWebhook(ctx context.Context, tenantID, webhookID string) (developerplatform.Webhook, error) {
	return scanWebhook(r.database.QueryRowContext(ctx, webhookSelect+` WHERE tenant_id=$1 AND id=$2`, tenantID, webhookID))
}

func (r *DeveloperWebhookRepository) ListWebhooks(ctx context.Context, tenantID string, query developerplatform.WebhookQuery) (developerplatform.WebhookPage, error) {
	at, id, err := decodeWebhookCursor(query.Cursor)
	if err != nil {
		return developerplatform.WebhookPage{}, developerplatform.ErrInvalidCommand
	}
	rows, err := r.database.QueryContext(ctx, webhookSelect+` WHERE tenant_id=$1 AND ($2='' OR status=$2) AND ($3::timestamptz IS NULL OR (updated_at,id)<($3,$4::uuid)) ORDER BY updated_at DESC,id DESC LIMIT $5`, tenantID, query.Status, nullableTime(at), nullableString(id), query.Limit+1)
	if err != nil {
		return developerplatform.WebhookPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]developerplatform.Webhook, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanWebhook(rows)
		if scanErr != nil {
			return developerplatform.WebhookPage{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return developerplatform.WebhookPage{}, err
	}
	page := developerplatform.WebhookPage{Items: items}
	if len(items) > query.Limit {
		last := items[query.Limit-1]
		page.Items = items[:query.Limit]
		page.NextCursor = encodeWebhookCursor(last.UpdatedAt, last.ID)
	}
	return page, nil
}

func (r *DeveloperWebhookRepository) ListWebhookDeliveries(ctx context.Context, tenantID, webhookID string, query developerplatform.DeliveryQuery) (developerplatform.DeliveryPage, error) {
	at, id, err := decodeWebhookCursor(query.Cursor)
	if err != nil {
		return developerplatform.DeliveryPage{}, developerplatform.ErrInvalidCommand
	}
	var exists bool
	if err = r.database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM developer_webhook_endpoints WHERE tenant_id=$1 AND id=$2)`, tenantID, webhookID).Scan(&exists); err != nil {
		return developerplatform.DeliveryPage{}, err
	}
	if !exists {
		return developerplatform.DeliveryPage{}, developerplatform.ErrNotFound
	}
	rows, err := r.database.QueryContext(ctx, `SELECT id::text,transfer_id::text,COALESCE(outbox_event_id::text,''),attempt_number,status,COALESCE(response_class,''),COALESCE(sanitized_error_code,''),due_at,started_at,completed_at,created_at FROM delivery_attempts WHERE tenant_id=$1 AND delivery_kind='webhook' AND endpoint_reference=$2 AND ($3='' OR status=$3) AND ($4::timestamptz IS NULL OR (created_at,id)<($4,$5::uuid)) ORDER BY created_at DESC,id DESC LIMIT $6`, tenantID, webhookID, query.Status, nullableTime(at), nullableString(id), query.Limit+1)
	if err != nil {
		return developerplatform.DeliveryPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]developerplatform.Delivery, 0, query.Limit+1)
	for rows.Next() {
		var item developerplatform.Delivery
		if err := rows.Scan(&item.AttemptID, &item.TransferID, &item.OutboxEventID, &item.AttemptNumber, &item.Status, &item.ResponseClass, &item.ErrorCode, &item.DueAt, &item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
			return developerplatform.DeliveryPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return developerplatform.DeliveryPage{}, err
	}
	page := developerplatform.DeliveryPage{Items: items}
	if len(items) > query.Limit {
		last := items[query.Limit-1]
		page.Items = items[:query.Limit]
		page.NextCursor = encodeWebhookCursor(last.CreatedAt, last.AttemptID)
	}
	return page, nil
}

type webhookScanner interface{ Scan(...any) error }

func scanWebhook(scanner webhookScanner) (developerplatform.Webhook, error) {
	var item developerplatform.Webhook
	var subscribedEvents []byte
	err := scanner.Scan(&item.ID, &item.DisplayName, &item.EndpointURL, &subscribedEvents, &item.SigningKeyReference, &item.SigningKeyID, &item.Status, &item.Version, &item.ChallengeExpiresAt, &item.VerifiedAt, &item.DisabledAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, developerplatform.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	if err = json.Unmarshal(subscribedEvents, &item.SubscribedEvents); err != nil || len(item.SubscribedEvents) == 0 {
		return item, errors.New("invalid persisted webhook subscriptions")
	}
	return item, nil
}
func lockWebhook(ctx context.Context, tx *sql.Tx, tenantID, webhookID string) (developerplatform.Webhook, []byte, error) {
	var item developerplatform.Webhook
	var digest, subscribedEvents []byte
	err := tx.QueryRowContext(ctx, `SELECT id::text,display_name,endpoint_url,array_to_json(subscribed_events)::text,signing_key_reference,signing_key_id,status,version::text,challenge_expires_at,verified_at,disabled_at,created_at,updated_at,challenge_digest FROM developer_webhook_endpoints WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, webhookID).Scan(&item.ID, &item.DisplayName, &item.EndpointURL, &subscribedEvents, &item.SigningKeyReference, &item.SigningKeyID, &item.Status, &item.Version, &item.ChallengeExpiresAt, &item.VerifiedAt, &item.DisabledAt, &item.CreatedAt, &item.UpdatedAt, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return item, nil, developerplatform.ErrNotFound
	}
	if err != nil {
		return item, nil, err
	}
	if err = json.Unmarshal(subscribedEvents, &item.SubscribedEvents); err != nil || len(item.SubscribedEvents) == 0 {
		return item, nil, errors.New("invalid persisted webhook subscriptions")
	}
	return item, digest, nil
}

func reserveWebhookCommand(ctx context.Context, tx *sql.Tx, tenantID, actorID, operation, key string, fingerprint [sha256.Size]byte, now time.Time, submission *developerplatform.WebhookSubmission) (bool, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO developer_webhook_command_idempotency(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,created_at) VALUES($1,$2,$3,$4,$5,'in_progress',$6) ON CONFLICT DO NOTHING`, tenantID, actorID, operation, key, fingerprint[:], now)
	if err != nil {
		return false, err
	}
	if affected, _ := result.RowsAffected(); affected == 1 {
		return false, nil
	}
	var stored, body []byte
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT request_fingerprint,state,response_body FROM developer_webhook_command_idempotency WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 FOR UPDATE`, tenantID, actorID, operation, key).Scan(&stored, &state, &body); err != nil {
		return false, err
	}
	if !bytes.Equal(stored, fingerprint[:]) {
		return false, developerplatform.ErrIdempotencyConflict
	}
	if state != "completed" || len(body) == 0 {
		return false, developerplatform.ErrConflict
	}
	if err := json.Unmarshal(body, &submission.Webhook); err != nil {
		return false, err
	}
	submission.Replayed = true
	return true, nil
}
func completeWebhookCommand(ctx context.Context, tx *sql.Tx, tenantID, actorID, operation, key string, webhook developerplatform.Webhook, at time.Time) error {
	body, err := json.Marshal(webhook)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE developer_webhook_command_idempotency SET state='completed',response_body=$5::jsonb,completed_at=$6 WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 AND state='in_progress'`, tenantID, actorID, operation, key, string(body), at)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return developerplatform.ErrConflict
	}
	return nil
}
func insertWebhookEvent(ctx context.Context, tx *sql.Tx, tenantID, webhookID, action string, version int64, actorID, correlationID string, at time.Time, details map[string]any) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO developer_webhook_events(id,tenant_id,webhook_id,action,version,actor_subject_id,correlation_id,sanitized_details,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, tenantID, webhookID, action, version, actorID, correlationID, encoded, at)
	return err
}
func classifyDeveloperWebhookError(err error) error {
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
		return developerplatform.ErrConflict
	}
	return err
}
func encodeWebhookCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}
func decodeWebhookCursor(cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return at.UTC(), parts[1], nil
}
