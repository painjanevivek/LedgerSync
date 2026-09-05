package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRecord struct {
	SubjectID               string
	TenantID                string
	CSRFToken               string
	ExpiresAt               time.Time
	AuthenticatedAt         *time.Time
	Roles                   []string
	Scopes                  []string
	ConsistencyRequirements map[string]string
}

type SessionRepository struct {
	database *sql.DB
	now      func() time.Time
}

func NewSessionRepository(database *sql.DB) (*SessionRepository, error) {
	if database == nil {
		return nil, errors.New("session database is required")
	}
	return &SessionRepository{database: database, now: time.Now}, nil
}

func (r *SessionRepository) Create(ctx context.Context, record SessionRecord, rotateToken string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate opaque session: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	digest := sha256.Sum256(tokenBytes)
	roles, _ := json.Marshal(record.Roles)
	scopes, _ := json.Marshal(record.Scopes)
	requirements, _ := json.Marshal(record.ConsistencyRequirements)
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "", fmt.Errorf("begin session creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if rotateToken != "" {
		oldDigest, digestErr := sessionDigest(rotateToken)
		if digestErr != nil {
			return "", ErrSessionNotFound
		}
		result, revokeErr := tx.ExecContext(ctx, `UPDATE bff_sessions SET revoked_at=now(),updated_at=now() WHERE token_digest=$1 AND revoked_at IS NULL`, oldDigest)
		if revokeErr != nil {
			return "", fmt.Errorf("revoke rotated session: %w", revokeErr)
		}
		if count, _ := result.RowsAffected(); count != 1 {
			return "", ErrSessionNotFound
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bff_sessions(token_digest,tenant_id,subject_id,csrf_token,roles,scopes,consistency_requirements,authenticated_at,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::jsonb,$8,$9,now(),now())`, digest[:], record.TenantID, record.SubjectID, record.CSRFToken, string(roles), string(scopes), string(requirements), record.AuthenticatedAt, record.ExpiresAt.UTC())
	if err != nil {
		return "", fmt.Errorf("persist opaque session: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM bff_sessions WHERE token_digest IN (SELECT token_digest FROM bff_sessions WHERE expires_at<=now() OR revoked_at<=now()-interval '1 hour' ORDER BY expires_at LIMIT 100)`); err != nil {
		return "", fmt.Errorf("retain opaque sessions: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("commit opaque session: %w", err)
	}
	return token, nil
}

func (r *SessionRepository) Resolve(ctx context.Context, token string) (SessionRecord, error) {
	digest, err := sessionDigest(token)
	if err != nil {
		return SessionRecord{}, ErrSessionNotFound
	}
	var record SessionRecord
	var roles, scopes, requirements []byte
	var authenticatedAt sql.NullTime
	err = r.database.QueryRowContext(ctx, `SELECT tenant_id::text,subject_id,csrf_token,roles,scopes,consistency_requirements,authenticated_at,expires_at FROM bff_sessions WHERE token_digest=$1 AND revoked_at IS NULL AND expires_at>now()`, digest).
		Scan(&record.TenantID, &record.SubjectID, &record.CSRFToken, &roles, &scopes, &requirements, &authenticatedAt, &record.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRecord{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionRecord{}, fmt.Errorf("resolve opaque session: %w", err)
	}
	if json.Unmarshal(roles, &record.Roles) != nil || json.Unmarshal(scopes, &record.Scopes) != nil || json.Unmarshal(requirements, &record.ConsistencyRequirements) != nil {
		return SessionRecord{}, errors.New("decode opaque session")
	}
	if authenticatedAt.Valid {
		value := authenticatedAt.Time.UTC()
		record.AuthenticatedAt = &value
	}
	record.ExpiresAt = record.ExpiresAt.UTC()
	return record, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, token string) error {
	digest, err := sessionDigest(token)
	if err != nil {
		return ErrSessionNotFound
	}
	result, err := r.database.ExecContext(ctx, `UPDATE bff_sessions SET revoked_at=now(),updated_at=now() WHERE token_digest=$1 AND revoked_at IS NULL`, digest)
	if err != nil {
		return fmt.Errorf("revoke opaque session: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) UpdateConsistency(ctx context.Context, token string, updates map[string]string) error {
	digest, err := sessionDigest(token)
	if err != nil {
		return ErrSessionNotFound
	}
	payload, err := json.Marshal(updates)
	if err != nil {
		return errors.New("encode consistency requirements")
	}
	result, err := r.database.ExecContext(ctx, `UPDATE bff_sessions SET consistency_requirements=consistency_requirements||$2::jsonb,updated_at=now() WHERE token_digest=$1 AND revoked_at IS NULL AND expires_at>now() AND jsonb_array_length(jsonb_path_query_array(consistency_requirements||$2::jsonb, '$.*'))<=10`, digest, string(payload))
	if err != nil {
		return fmt.Errorf("update session consistency: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrSessionNotFound
	}
	return nil
}

func sessionDigest(token string) ([]byte, error) {
	if strings.TrimSpace(token) != token || len(token) != 43 {
		return nil, ErrSessionNotFound
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, ErrSessionNotFound
	}
	digest := sha256.Sum256(raw)
	return digest[:], nil
}
