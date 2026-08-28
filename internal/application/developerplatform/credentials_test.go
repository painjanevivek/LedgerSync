package developerplatform

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

type credentialRepositoryStub struct {
	create CreateCredentialCommand
	rotate RotateCredentialCommand
	revoke RevokeCredentialCommand
	result CredentialSubmission
}

func (r *credentialRepositoryStub) CreateCredential(_ context.Context, command CreateCredentialCommand, _ [sha256.Size]byte) (CredentialSubmission, error) {
	r.create = command
	return r.result, nil
}
func (r *credentialRepositoryStub) RotateCredential(_ context.Context, command RotateCredentialCommand, _ [sha256.Size]byte) (CredentialSubmission, error) {
	r.rotate = command
	return r.result, nil
}
func (r *credentialRepositoryStub) RevokeCredential(_ context.Context, command RevokeCredentialCommand, _ [sha256.Size]byte) (CredentialSubmission, error) {
	r.revoke = command
	return r.result, nil
}
func (*credentialRepositoryStub) GetCredential(context.Context, string, string) (Credential, error) {
	return Credential{}, ErrNotFound
}
func (*credentialRepositoryStub) ListCredentials(context.Context, string, CredentialQuery) (CredentialPage, error) {
	return CredentialPage{}, nil
}

func TestCredentialServiceNormalizesMetadataWithoutSecretMaterial(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository := &credentialRepositoryStub{}
	service, err := NewCredentialService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Create(context.Background(), CreateCredentialCommand{
		TenantID: " tenant ", ActorSubjectID: " actor ", CorrelationID: " correlation ", IdempotencyKey: " credential-key-0001 ",
		DisplayName: " Partner production ", ExternalReference: " cognito/client-001 ", Audience: " ledgersync-api ",
		Scopes: []string{"transfers:write", "accounts:read", "transfers:write"}, ExpiresAt: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.create.DisplayName != "Partner production" || repository.create.ExternalReference != "cognito/client-001" || len(repository.create.Scopes) != 2 || repository.create.Scopes[0] != "accounts:read" {
		t.Fatalf("unexpected normalized command: %#v", repository.create)
	}
}

func TestCredentialServiceRejectsUnsafeOrUnboundedMetadata(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository := &credentialRepositoryStub{}
	service, _ := NewCredentialService(repository, func() time.Time { return now })
	base := CreateCredentialCommand{TenantID: "tenant", ActorSubjectID: "actor", CorrelationID: "correlation", IdempotencyKey: "credential-key-0001", DisplayName: "Partner", ExternalReference: "cognito/client-001", Audience: "ledgersync-api", Scopes: []string{"accounts:read"}, ExpiresAt: now.Add(time.Hour)}
	for name, mutate := range map[string]func(*CreateCredentialCommand){
		"secret-like whitespace": func(command *CreateCredentialCommand) { command.ExternalReference = "raw secret value" },
		"unknown scope":          func(command *CreateCredentialCommand) { command.Scopes = []string{"admin:*"} },
		"expired":                func(command *CreateCredentialCommand) { command.ExpiresAt = now },
		"too distant":            func(command *CreateCredentialCommand) { command.ExpiresAt = now.Add(367 * 24 * time.Hour) },
		"short retry key":        func(command *CreateCredentialCommand) { command.IdempotencyKey = "short" },
	} {
		t.Run(name, func(t *testing.T) {
			command := base
			mutate(&command)
			if _, err := service.Create(context.Background(), command); !errors.Is(err, ErrInvalidCommand) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
