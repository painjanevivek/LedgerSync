// Package developerplatform defines the public integration-control lifecycle.
// Credential records contain external identity-provider references only; raw
// client secrets, private keys, bearer tokens, and refresh tokens are forbidden.
package developerplatform

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidCommand      = errors.New("invalid developer platform command")
	ErrNotFound            = errors.New("developer platform record not found")
	ErrConflict            = errors.New("developer platform record conflict")
	ErrVersionConflict     = errors.New("developer platform version conflict")
	ErrIdempotencyConflict = errors.New("developer platform idempotency conflict")
)

const (
	CreateCredentialOperation = "developer_credential_create"
	RotateCredentialOperation = "developer_credential_rotate"
	RevokeCredentialOperation = "developer_credential_revoke"
)

var (
	credentialReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{2,199}$`)
	safeAudience        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{2,199}$`)
	canonicalUUID       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

var AllowedCredentialScopes = []string{
	"accounts:read", "accounts:write", "events:read", "funding:read", "funding:write",
	"reconciliation:read", "reconciliation:write", "transactions:read", "transfers:read", "transfers:write",
	"webhooks:read", "webhooks:write", "webhooks:replay",
}

type Credential struct {
	ID                string     `json:"credential_id"`
	DisplayName       string     `json:"display_name"`
	ExternalReference string     `json:"external_reference"`
	Audience          string     `json:"audience"`
	Scopes            []string   `json:"scopes"`
	Status            string     `json:"status"`
	Version           string     `json:"version"`
	ExpiresAt         time.Time  `json:"expires_at"`
	LastUsedAt        *time.Time `json:"last_used_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	RevokedAt         *time.Time `json:"revoked_at"`
}

type CredentialPage struct {
	Items      []Credential `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type CreateCredentialCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey string
	DisplayName, ExternalReference, Audience                string
	Scopes                                                  []string
	ExpiresAt                                               time.Time
}

type RotateCredentialCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey string
	CredentialID, ExternalReference, Audience               string
	Scopes                                                  []string
	ExpiresAt                                               time.Time
	ExpectedVersion                                         int64
}

type RevokeCredentialCommand struct {
	TenantID, ActorSubjectID, CorrelationID, IdempotencyKey string
	CredentialID, Reason                                    string
	ExpectedVersion                                         int64
}

type CredentialQuery struct {
	Status, Cursor string
	Limit          int
}

type CredentialSubmission struct {
	Credential Credential
	Replayed   bool
}

type CredentialRepository interface {
	CreateCredential(context.Context, CreateCredentialCommand, [sha256.Size]byte) (CredentialSubmission, error)
	RotateCredential(context.Context, RotateCredentialCommand, [sha256.Size]byte) (CredentialSubmission, error)
	RevokeCredential(context.Context, RevokeCredentialCommand, [sha256.Size]byte) (CredentialSubmission, error)
	GetCredential(context.Context, string, string) (Credential, error)
	ListCredentials(context.Context, string, CredentialQuery) (CredentialPage, error)
}

type CredentialService struct {
	repository CredentialRepository
	clock      func() time.Time
}

func NewCredentialService(repository CredentialRepository, clock func() time.Time) (*CredentialService, error) {
	if repository == nil {
		return nil, errors.New("credential repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &CredentialService{repository: repository, clock: clock}, nil
}

func (s *CredentialService) Create(ctx context.Context, command CreateCredentialCommand) (CredentialSubmission, error) {
	normalizeCreate(&command)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || !validCredentialFields(command.DisplayName, command.ExternalReference, command.Audience, command.Scopes, command.ExpiresAt, s.clock().UTC()) {
		return CredentialSubmission{}, ErrInvalidCommand
	}
	return s.repository.CreateCredential(ctx, command, fingerprint(CreateCredentialOperation, command.DisplayName, command.ExternalReference, command.Audience, command.Scopes, command.ExpiresAt.UTC().Format(time.RFC3339)))
}

func (s *CredentialService) Rotate(ctx context.Context, command RotateCredentialCommand) (CredentialSubmission, error) {
	normalizeRotate(&command)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || !canonicalUUID.MatchString(command.CredentialID) || command.ExpectedVersion < 1 || !validCredentialFields("rotation", command.ExternalReference, command.Audience, command.Scopes, command.ExpiresAt, s.clock().UTC()) {
		return CredentialSubmission{}, ErrInvalidCommand
	}
	return s.repository.RotateCredential(ctx, command, fingerprint(RotateCredentialOperation, command.CredentialID, command.ExpectedVersion, command.ExternalReference, command.Audience, command.Scopes, command.ExpiresAt.UTC().Format(time.RFC3339)))
}

func (s *CredentialService) Revoke(ctx context.Context, command RevokeCredentialCommand) (CredentialSubmission, error) {
	command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID), strings.TrimSpace(command.CorrelationID), strings.TrimSpace(command.IdempotencyKey)
	command.CredentialID, command.Reason = strings.ToLower(strings.TrimSpace(command.CredentialID)), strings.TrimSpace(command.Reason)
	if !validEnvelope(command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey) || !canonicalUUID.MatchString(command.CredentialID) || command.ExpectedVersion < 1 || len(command.Reason) < 3 || len(command.Reason) > 500 {
		return CredentialSubmission{}, ErrInvalidCommand
	}
	return s.repository.RevokeCredential(ctx, command, fingerprint(RevokeCredentialOperation, command.CredentialID, command.ExpectedVersion, command.Reason))
}

func (s *CredentialService) Get(ctx context.Context, tenantID, credentialID string) (Credential, error) {
	tenantID, credentialID = strings.TrimSpace(tenantID), strings.ToLower(strings.TrimSpace(credentialID))
	if tenantID == "" || !canonicalUUID.MatchString(credentialID) {
		return Credential{}, ErrInvalidCommand
	}
	return s.repository.GetCredential(ctx, tenantID, credentialID)
}

func (s *CredentialService) List(ctx context.Context, tenantID string, query CredentialQuery) (CredentialPage, error) {
	tenantID, query.Status, query.Cursor = strings.TrimSpace(tenantID), strings.ToLower(strings.TrimSpace(query.Status)), strings.TrimSpace(query.Cursor)
	if query.Limit == 0 {
		query.Limit = 50
	}
	if tenantID == "" || query.Limit < 1 || query.Limit > 100 || query.Status != "" && !slices.Contains([]string{"active", "expired", "revoked"}, query.Status) || len(query.Cursor) > 512 {
		return CredentialPage{}, ErrInvalidCommand
	}
	return s.repository.ListCredentials(ctx, tenantID, query)
}

func normalizeCreate(command *CreateCredentialCommand) {
	command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID), strings.TrimSpace(command.CorrelationID), strings.TrimSpace(command.IdempotencyKey)
	command.DisplayName, command.ExternalReference, command.Audience = strings.TrimSpace(command.DisplayName), strings.TrimSpace(command.ExternalReference), strings.TrimSpace(command.Audience)
	command.Scopes = normalizedScopes(command.Scopes)
	command.ExpiresAt = command.ExpiresAt.UTC()
}

func normalizeRotate(command *RotateCredentialCommand) {
	command.TenantID, command.ActorSubjectID, command.CorrelationID, command.IdempotencyKey = strings.TrimSpace(command.TenantID), strings.TrimSpace(command.ActorSubjectID), strings.TrimSpace(command.CorrelationID), strings.TrimSpace(command.IdempotencyKey)
	command.CredentialID, command.ExternalReference, command.Audience = strings.ToLower(strings.TrimSpace(command.CredentialID)), strings.TrimSpace(command.ExternalReference), strings.TrimSpace(command.Audience)
	command.Scopes = normalizedScopes(command.Scopes)
	command.ExpiresAt = command.ExpiresAt.UTC()
}

func normalizedScopes(scopes []string) []string {
	result := make([]string, 0, len(scopes))
	seen := map[string]struct{}{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

func validEnvelope(tenantID, actorID, correlationID, key string) bool {
	return tenantID != "" && actorID != "" && correlationID != "" && len(key) >= 16 && len(key) <= 255
}

func validCredentialFields(displayName, reference, audience string, scopes []string, expiresAt, now time.Time) bool {
	if len(displayName) < 1 || len(displayName) > 100 || !credentialReference.MatchString(reference) || !safeAudience.MatchString(audience) || len(scopes) < 1 || len(scopes) > 16 || !expiresAt.After(now.Add(5*time.Minute)) || expiresAt.After(now.Add(366*24*time.Hour)) {
		return false
	}
	for _, scope := range scopes {
		if !slices.Contains(AllowedCredentialScopes, scope) {
			return false
		}
	}
	return true
}

func fingerprint(parts ...any) [sha256.Size]byte {
	encoded, _ := json.Marshal(parts)
	return sha256.Sum256(encoded)
}
