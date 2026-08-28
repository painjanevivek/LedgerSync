package db

import (
	"bytes"
	"context"
	"crypto/sha256"
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

type DeveloperCredentialRepository struct {
	database *sql.DB
	clock    func() time.Time
}

func NewDeveloperCredentialRepository(database *sql.DB, clock func() time.Time) (*DeveloperCredentialRepository, error) {
	if database == nil {
		return nil, errors.New("developer credential database is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &DeveloperCredentialRepository{database: database, clock: clock}, nil
}

func (r *DeveloperCredentialRepository) CreateCredential(ctx context.Context, command developerplatform.CreateCredentialCommand, fingerprint [sha256.Size]byte) (submission developerplatform.CredentialSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-credential-create|"+command.TenantID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveDeveloperCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.CreateCredentialOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		id, err := newUUID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO developer_credentials(id,tenant_id,display_name,external_reference,audience,scopes,status,version,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'active',1,$7,$8,$8)`, id, command.TenantID, command.DisplayName, command.ExternalReference, command.Audience, command.Scopes, command.ExpiresAt, now)
		if err != nil {
			return classifyDeveloperCredentialError(err)
		}
		submission.Credential = developerplatform.Credential{ID: id, DisplayName: command.DisplayName, ExternalReference: command.ExternalReference, Audience: command.Audience, Scopes: append([]string(nil), command.Scopes...), Status: "active", Version: "1", ExpiresAt: command.ExpiresAt, CreatedAt: now, UpdatedAt: now}
		if err := insertDeveloperCredentialEvent(ctx, tx, command.TenantID, id, "created", 1, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"audience": command.Audience, "scopes": command.Scopes}); err != nil {
			return err
		}
		return completeDeveloperCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.CreateCredentialOperation, command.IdempotencyKey, submission.Credential, now)
	})
	return submission, err
}

func (r *DeveloperCredentialRepository) RotateCredential(ctx context.Context, command developerplatform.RotateCredentialCommand, fingerprint [sha256.Size]byte) (submission developerplatform.CredentialSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-credential-rotate|"+command.TenantID+"|"+command.CredentialID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveDeveloperCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RotateCredentialOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		current, err := lockDeveloperCredential(ctx, tx, command.TenantID, command.CredentialID, now)
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
		previousReference, previousLastUsedAt := current.ExternalReference, current.LastUsedAt
		result, err := tx.ExecContext(ctx, `UPDATE developer_credentials SET external_reference=$4,audience=$5,scopes=$6,expires_at=$7,last_used_at=NULL,version=$8,updated_at=$9 WHERE tenant_id=$1 AND id=$2 AND version=$3 AND status='active'`, command.TenantID, command.CredentialID, version, command.ExternalReference, command.Audience, command.Scopes, command.ExpiresAt, newVersion, now)
		if err != nil {
			return classifyDeveloperCredentialError(err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return developerplatform.ErrVersionConflict
		}
		current.ExternalReference, current.Audience, current.Scopes, current.ExpiresAt, current.LastUsedAt, current.Version, current.UpdatedAt = command.ExternalReference, command.Audience, append([]string(nil), command.Scopes...), command.ExpiresAt, nil, strconv.FormatInt(newVersion, 10), now
		submission.Credential = current
		if err := insertDeveloperCredentialEvent(ctx, tx, command.TenantID, command.CredentialID, "rotated", newVersion, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"audience": command.Audience, "scopes": command.Scopes, "previous_reference": previousReference, "previous_last_used_at": previousLastUsedAt}); err != nil {
			return err
		}
		return completeDeveloperCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RotateCredentialOperation, command.IdempotencyKey, submission.Credential, now)
	})
	return submission, err
}

func (r *DeveloperCredentialRepository) RevokeCredential(ctx context.Context, command developerplatform.RevokeCredentialCommand, fingerprint [sha256.Size]byte) (submission developerplatform.CredentialSubmission, err error) {
	now := r.clock().UTC()
	err = WithSerializableSequence(ctx, r.database, "developer-credential-revoke|"+command.TenantID+"|"+command.CredentialID, 5, func(tx *sql.Tx) error {
		if err := authorizeTenantActor(ctx, tx, command.TenantID, command.ActorSubjectID); err != nil {
			return err
		}
		replayed, err := reserveDeveloperCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RevokeCredentialOperation, command.IdempotencyKey, fingerprint, now, &submission)
		if err != nil || replayed {
			return err
		}
		current, err := lockDeveloperCredential(ctx, tx, command.TenantID, command.CredentialID, now)
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
		result, err := tx.ExecContext(ctx, `UPDATE developer_credentials SET status='revoked',version=$4,revoked_at=$5,updated_at=$5 WHERE tenant_id=$1 AND id=$2 AND version=$3 AND status='active'`, command.TenantID, command.CredentialID, version, newVersion, now)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return developerplatform.ErrVersionConflict
		}
		current.Status, current.Version, current.UpdatedAt, current.RevokedAt = "revoked", strconv.FormatInt(newVersion, 10), now, &now
		submission.Credential = current
		if err := insertDeveloperCredentialEvent(ctx, tx, command.TenantID, command.CredentialID, "revoked", newVersion, command.ActorSubjectID, command.CorrelationID, now, map[string]any{"reason": command.Reason}); err != nil {
			return err
		}
		return completeDeveloperCommand(ctx, tx, command.TenantID, command.ActorSubjectID, developerplatform.RevokeCredentialOperation, command.IdempotencyKey, submission.Credential, now)
	})
	return submission, err
}

func (r *DeveloperCredentialRepository) GetCredential(ctx context.Context, tenantID, credentialID string) (developerplatform.Credential, error) {
	row := r.database.QueryRowContext(ctx, credentialSelect+` WHERE tenant_id=$1 AND id=$2`, tenantID, credentialID)
	return scanDeveloperCredential(row, r.clock().UTC())
}

func (r *DeveloperCredentialRepository) ListCredentials(ctx context.Context, tenantID string, query developerplatform.CredentialQuery) (developerplatform.CredentialPage, error) {
	cursorTime, cursorID, err := decodeDeveloperCredentialCursor(query.Cursor)
	if err != nil {
		return developerplatform.CredentialPage{}, developerplatform.ErrInvalidCommand
	}
	now := r.clock().UTC()
	rows, err := r.database.QueryContext(ctx, credentialSelect+` WHERE tenant_id=$1
AND ($2='' OR $2='active' AND status='active' AND expires_at>$3 OR $2='expired' AND status='active' AND expires_at<=$3 OR $2='revoked' AND status='revoked')
AND ($4::timestamptz IS NULL OR (updated_at,id)<($4,$5::uuid)) ORDER BY updated_at DESC,id DESC LIMIT $6`, tenantID, query.Status, now, nullableTime(cursorTime), nullableString(cursorID), query.Limit+1)
	if err != nil {
		return developerplatform.CredentialPage{}, err
	}
	defer rows.Close()
	items := make([]developerplatform.Credential, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanDeveloperCredential(rows, now)
		if scanErr != nil {
			return developerplatform.CredentialPage{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return developerplatform.CredentialPage{}, err
	}
	page := developerplatform.CredentialPage{Items: items}
	if len(items) > query.Limit {
		last := items[query.Limit-1]
		page.Items = items[:query.Limit]
		page.NextCursor = encodeDeveloperCredentialCursor(last.UpdatedAt, last.ID)
	}
	return page, nil
}

// RecordUsage advances last-used evidence at most once per minute per
// credential. Unknown external references are intentionally ignored so identity
// provider rollout can precede metadata registration without leaking existence.
func (r *DeveloperCredentialRepository) RecordUsage(ctx context.Context, tenantID, externalReference string, at time.Time) error {
	at = at.UTC()
	_, err := r.database.ExecContext(ctx, `UPDATE developer_credentials SET last_used_at=$3 WHERE tenant_id=$1 AND external_reference=$2 AND status='active' AND expires_at>$3 AND (last_used_at IS NULL OR last_used_at<$3-interval '1 minute')`, tenantID, externalReference, at)
	return err
}

const credentialSelect = `SELECT id,display_name,external_reference,audience,array_to_json(scopes)::text,status,version,expires_at,last_used_at,created_at,updated_at,revoked_at FROM developer_credentials`

type credentialScanner interface{ Scan(...any) error }

func scanDeveloperCredential(scanner credentialScanner, now time.Time) (developerplatform.Credential, error) {
	var item developerplatform.Credential
	var scopesJSON string
	var version int64
	if err := scanner.Scan(&item.ID, &item.DisplayName, &item.ExternalReference, &item.Audience, &scopesJSON, &item.Status, &version, &item.ExpiresAt, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt, &item.RevokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, developerplatform.ErrNotFound
		}
		return item, err
	}
	if err := json.Unmarshal([]byte(scopesJSON), &item.Scopes); err != nil {
		return item, err
	}
	item.Version = strconv.FormatInt(version, 10)
	if item.Status == "active" && !item.ExpiresAt.After(now) {
		item.Status = "expired"
	}
	return item, nil
}

func lockDeveloperCredential(ctx context.Context, tx *sql.Tx, tenantID, credentialID string, now time.Time) (developerplatform.Credential, error) {
	return scanDeveloperCredential(tx.QueryRowContext(ctx, credentialSelect+` WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, credentialID), now)
}

func reserveDeveloperCommand(ctx context.Context, tx *sql.Tx, tenantID, actorID, operation, key string, fingerprint [sha256.Size]byte, now time.Time, submission *developerplatform.CredentialSubmission) (bool, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO developer_command_idempotency(tenant_id,actor_subject_id,operation,idempotency_key,request_fingerprint,state,created_at) VALUES($1,$2,$3,$4,$5,'in_progress',$6) ON CONFLICT DO NOTHING`, tenantID, actorID, operation, key, fingerprint[:], now)
	if err != nil {
		return false, err
	}
	if affected, _ := result.RowsAffected(); affected == 1 {
		return false, nil
	}
	var stored []byte
	var state string
	var body []byte
	if err := tx.QueryRowContext(ctx, `SELECT request_fingerprint,state,response_body FROM developer_command_idempotency WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 FOR UPDATE`, tenantID, actorID, operation, key).Scan(&stored, &state, &body); err != nil {
		return false, err
	}
	if !bytes.Equal(stored, fingerprint[:]) {
		return false, developerplatform.ErrIdempotencyConflict
	}
	if state != "completed" || len(body) == 0 {
		return false, developerplatform.ErrConflict
	}
	if err := json.Unmarshal(body, &submission.Credential); err != nil {
		return false, err
	}
	submission.Replayed = true
	return true, nil
}

func completeDeveloperCommand(ctx context.Context, tx *sql.Tx, tenantID, actorID, operation, key string, credential developerplatform.Credential, at time.Time) error {
	body, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE developer_command_idempotency SET state='completed',response_body=$5::jsonb,completed_at=$6 WHERE tenant_id=$1 AND actor_subject_id=$2 AND operation=$3 AND idempotency_key=$4 AND state='in_progress'`, tenantID, actorID, operation, key, string(body), at)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return developerplatform.ErrConflict
	}
	return nil
}

func insertDeveloperCredentialEvent(ctx context.Context, tx *sql.Tx, tenantID, credentialID, action string, version int64, actorID, correlationID string, at time.Time, details map[string]any) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO developer_credential_events(id,tenant_id,credential_id,action,version,actor_subject_id,correlation_id,sanitized_details,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, tenantID, credentialID, action, version, actorID, correlationID, encoded, at)
	return err
}

func classifyDeveloperCredentialError(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return developerplatform.ErrConflict
	}
	return err
}

func encodeDeveloperCredentialCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeDeveloperCredentialCursor(cursor string) (time.Time, string, error) {
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
