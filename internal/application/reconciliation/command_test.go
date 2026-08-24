package reconciliation

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

type commandRepositoryStub struct {
	command     RunCommand
	fingerprint [sha256.Size]byte
	submission  CommandSubmission
}

func (r *commandRepositoryStub) RunCommand(_ context.Context, command RunCommand, fingerprint [sha256.Size]byte) (CommandSubmission, error) {
	r.command, r.fingerprint = command, fingerprint
	return r.submission, nil
}

func TestCommandServiceNormalizesAndNamespacesRetryIntent(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	first, second := &commandRepositoryStub{}, &commandRepositoryStub{}
	for _, repository := range []*commandRepositoryStub{first, second} {
		service, err := NewCommandService(repository, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Run(context.Background(), RunCommand{TenantID: " tenant ", ActorSubjectID: " operator ", CorrelationID: " correlation ", IdempotencyKey: " reconciliation-key-0001 "}); err != nil {
			t.Fatal(err)
		}
	}
	if first.command.TenantID != "tenant" || first.command.ActorSubjectID != "operator" || first.command.OccurredAt != now {
		t.Fatalf("unexpected normalized command: %#v", first.command)
	}
	if first.fingerprint != second.fingerprint {
		t.Fatal("identical retry intent produced a different fingerprint")
	}
}

func TestCommandServiceRejectsIncompleteEnvelopeAndInvalidKey(t *testing.T) {
	service, err := NewCommandService(&commandRepositoryStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Run(context.Background(), RunCommand{}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("empty command error = %v", err)
	}
	if _, err := service.Run(context.Background(), RunCommand{TenantID: "tenant", ActorSubjectID: "actor", CorrelationID: "correlation", IdempotencyKey: "short"}); err == nil {
		t.Fatal("invalid idempotency key accepted")
	}
}
