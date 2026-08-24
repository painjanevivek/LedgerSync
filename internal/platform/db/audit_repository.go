package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
)

// AuditEvent is deliberately metadata-minimal. It must contain identifiers and
// outcome context, never credentials, money amounts, raw payloads, or PII.
type AuditEvent struct {
	TenantID, ActorSubjectID, EventType, TargetType, TargetID, Outcome, CorrelationID string
	Metadata                                                                          map[string]string
	OccurredAt                                                                        time.Time
}

type AuditRepository struct{ database *sql.DB }

func NewAuditRepository(database *sql.DB) (*AuditRepository, error) {
	if database == nil {
		return nil, errors.New("audit database is required")
	}
	return &AuditRepository{database: database}, nil
}

func (r *AuditRepository) Record(ctx context.Context, event AuditEvent) error {
	if r == nil || strings.TrimSpace(event.TenantID) == "" || strings.TrimSpace(event.EventType) == "" || strings.TrimSpace(event.TargetType) == "" || strings.TrimSpace(event.Outcome) == "" || strings.TrimSpace(event.CorrelationID) == "" {
		return errors.New("audit event has required fields missing")
	}
	metadata, err := json.Marshal(sanitizeAuditMetadata(event.Metadata))
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	when := event.OccurredAt
	if when.IsZero() {
		when = time.Now().UTC()
	}
	id, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate audit event ID: %w", err)
	}
	_, err = r.database.ExecContext(ctx, `INSERT INTO audit_events (id,tenant_id,actor_subject_id,event_type,target_type,target_id,outcome,correlation_id,sanitized_metadata,occurred_at) VALUES ($1,$2,NULLIF($3,''),$4,$5,NULLIF($6,''),$7,$8,$9,$10)`, id, event.TenantID, event.ActorSubjectID, event.EventType, event.TargetType, event.TargetID, event.Outcome, event.CorrelationID, metadata, when.UTC())
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

func sanitizeAuditMetadata(metadata map[string]string) map[string]string {
	clean := make(map[string]string)
	for key, value := range metadata {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" || len(key) > 64 || !allowedAuditMetadataValue(key, value) || sensitiveAuditKey(key) {
			continue
		}
		clean[key] = value
	}
	return clean
}

func allowedAuditMetadataValue(key, value string) bool {
	if key != "reason" {
		return len(value) <= 256
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > accounts.MaxLifecycleReasonRunes {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func sensitiveAuditKey(key string) bool {
	for _, word := range []string{"secret", "token", "authorization", "cookie", "session", "csrf", "consistency", "credential", "database_url", "connection_string", "dsn", "private_key", "api_key", "access_key", "amount", "balance", "email", "phone", "address", "ip_address", "payload"} {
		if strings.Contains(key, word) {
			return true
		}
	}
	return false
}
