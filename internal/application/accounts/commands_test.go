package accounts

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"
)

type capturingCommandRepository struct {
	createFingerprint [sha256.Size]byte
	updateFingerprint [sha256.Size]byte
	statusFingerprint [sha256.Size]byte
	createCommand     CreateAccountCommand
}

func (r *capturingCommandRepository) Create(_ context.Context, command CreateAccountCommand, fingerprint [sha256.Size]byte) (CommandSubmission, error) {
	r.createCommand, r.createFingerprint = command, fingerprint
	return CommandSubmission{Result: CommandResult{AccountID: "account"}}, nil
}
func (r *capturingCommandRepository) UpdateMetadata(_ context.Context, _ UpdateAccountMetadataCommand, fingerprint [sha256.Size]byte) (CommandSubmission, error) {
	r.updateFingerprint = fingerprint
	return CommandSubmission{}, nil
}
func (r *capturingCommandRepository) ChangeStatus(_ context.Context, _ ChangeAccountStatusCommand, fingerprint [sha256.Size]byte) (CommandSubmission, error) {
	r.statusFingerprint = fingerprint
	return CommandSubmission{}, nil
}

func TestCreateCommandNormalizesOnlySemanticCaseInsensitiveFields(t *testing.T) {
	clock := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
	firstRepository := &capturingCommandRepository{}
	first, _ := NewCommandService(firstRepository, func() time.Time { return clock })
	command := CreateAccountCommand{TenantID: " tenant ", ActorSubjectID: " actor ", CorrelationID: " correlation ", IdempotencyKey: "0123456789abcdef", DisplayName: "  भारत Reserve  ", Reference: " OPS-INR ", Category: " Reserve ", Currency: " inr "}
	if _, err := first.Create(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	secondRepository := &capturingCommandRepository{}
	second, _ := NewCommandService(secondRepository, func() time.Time { return clock.Add(time.Hour) })
	command.TenantID, command.ActorSubjectID, command.CorrelationID = "tenant", "actor", "other-correlation"
	command.DisplayName, command.Reference, command.Category, command.Currency = "भारत Reserve", "ops-inr", "reserve", "INR"
	if _, err := second.Create(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if firstRepository.createFingerprint != secondRepository.createFingerprint {
		t.Fatal("semantically identical create commands produced different fingerprints")
	}
	if firstRepository.createCommand.OccurredAt != clock || firstRepository.createCommand.Reference != "ops-inr" || firstRepository.createCommand.DisplayName != "भारत Reserve" {
		t.Fatalf("unexpected normalized command: %#v", firstRepository.createCommand)
	}
}

func TestAccountCommandFingerprintsConflictOnChangedIntent(t *testing.T) {
	repository := &capturingCommandRepository{}
	service, _ := NewCommandService(repository, time.Now)
	base := UpdateAccountMetadataCommand{TenantID: "tenant", ActorSubjectID: "actor", CorrelationID: "correlation", IdempotencyKey: "0123456789abcdef", AccountID: "account", ExpectedVersion: 2, DisplayName: "Payroll", Reference: "payroll-inr", Category: "payroll"}
	if _, err := service.UpdateMetadata(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	original := repository.updateFingerprint
	base.DisplayName = "Payroll changed"
	if _, err := service.UpdateMetadata(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	if original == repository.updateFingerprint {
		t.Fatal("changed metadata did not change the fingerprint")
	}
	status := ChangeAccountStatusCommand{TenantID: "tenant", ActorSubjectID: "actor", CorrelationID: "correlation", IdempotencyKey: "0123456789abcdef", AccountID: "account", ExpectedVersion: 2, TargetStatus: "frozen"}
	if _, err := service.ChangeStatus(context.Background(), status); err != nil {
		t.Fatal(err)
	}
	frozen := repository.statusFingerprint
	status.TargetStatus = "closed"
	if _, err := service.ChangeStatus(context.Background(), status); err != nil {
		t.Fatal(err)
	}
	if frozen == repository.statusFingerprint {
		t.Fatal("changed lifecycle target did not change the fingerprint")
	}
}
