package statementreconciliation

import (
	"context"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
)

type Service struct{ repository Repository }

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, ErrInvalid
	}
	return &Service{repository}, nil
}
func validID(ctx context.Context, id string) bool {
	canonical, err := identifier.Canonicalize(ctx, identifier.KindReconciliation, id)
	return err == nil && canonical == id && id != "00000000-0000-0000-0000-000000000000"
}
func validScope(ctx context.Context, scope Scope) bool {
	return validID(ctx, scope.TenantID) && scope.ActorSubjectID != "" && len(scope.ActorSubjectID) <= 256 && utf8.ValidString(scope.ActorSubjectID) && strings.TrimSpace(scope.ActorSubjectID) == scope.ActorSubjectID && !strings.ContainsFunc(scope.ActorSubjectID, unicode.IsControl)
}
func validMeta(ctx context.Context, meta Meta) bool {
	return validScope(ctx, meta.Scope) && validID(ctx, meta.CorrelationID) && idempotency.ValidateKey(meta.IdempotencyKey) == nil && strings.TrimSpace(meta.IdempotencyKey) == meta.IdempotencyKey
}
func (s *Service) CreateMapping(ctx context.Context, c MappingCommand) (Receipt, error) {
	if !validMeta(ctx, c.Meta) || !validID(ctx, c.AccountID) || !token.MatchString(c.SourceAccountRef) {
		return Receipt{}, ErrInvalid
	}
	return s.repository.CreateMapping(ctx, c)
}
func (s *Service) Preview(source []byte) (Preview, error) {
	p, err := Parse(source)
	if err != nil {
		return Preview{}, err
	}
	return Preview{SHA256: hex.EncodeToString(p.SHA256[:]), SourceAccountRef: p.SourceAccountRef, PeriodStart: p.PeriodStart, PeriodEnd: p.PeriodEnd, OpeningMinor: p.OpeningMinor, ClosingMinor: p.ClosingMinor, RowCount: len(p.Rows)}, nil
}
func (s *Service) Import(ctx context.Context, c ImportCommand) (Receipt, error) {
	if !validMeta(ctx, c.Meta) || !validID(ctx, c.MappingID) {
		return Receipt{}, ErrInvalid
	}
	parsed, err := Parse(c.Source)
	if err != nil {
		return Receipt{}, err
	}
	c.Source = parsed.Source
	return s.repository.AcceptImport(ctx, c)
}
func (s *Service) Run(ctx context.Context, c RunCommand) (Receipt, error) {
	if !validMeta(ctx, c.Meta) || !validID(ctx, c.ImportID) {
		return Receipt{}, ErrInvalid
	}
	prepared, err := s.repository.PrepareRun(ctx, c)
	if err != nil {
		return Receipt{}, err
	}
	snapshot, err := s.repository.Snapshot(ctx, c.Scope, prepared.ID)
	if err != nil {
		return Receipt{}, err
	}
	if snapshot.Header.State == "completed" {
		return prepared, nil
	}
	if len(snapshot.Rows) > MaxRows || len(snapshot.Items) > MaxSnapshotItems {
		return Receipt{}, ErrLimit
	}
	completed, err := s.repository.CompleteRun(ctx, c.Scope, prepared.ID, MatchExact(snapshot.Rows, snapshot.Items))
	if err != nil {
		return Receipt{}, err
	}
	completed.Replayed = prepared.Replayed || completed.Replayed
	return completed, nil
}
func (s *Service) Review(ctx context.Context, c ReviewCommand) (Receipt, error) {
	if !validMeta(ctx, c.Meta) || !validID(ctx, c.RunID) || !validReview(ctx, c.Decision) {
		return Receipt{}, ErrInvalid
	}
	return s.repository.Review(ctx, c)
}
func validReview(ctx context.Context, d ReviewInput) bool {
	if (d.Kind != "statement" && d.Kind != "ledger") || !token.MatchString(d.ItemID) || !signed.MatchString(d.ExpectedVersion) || strings.HasPrefix(d.ExpectedVersion, "-") {
		return false
	}
	if d.Status != "open" && d.Status != "reviewed_exception" && d.Status != "matched" {
		return false
	}
	if !utf8.ValidString(d.Comment) || len(d.Comment) > 512 || !utf8.ValidString(d.Assignee) || len(d.Assignee) > 256 || strings.ContainsFunc(d.Assignee, unicode.IsControl) {
		return false
	}
	if !reviewReason.MatchString(d.Reason) || len(d.Evidence) > 256 || (d.Evidence != "" && !reviewEvidence.MatchString(d.Evidence)) {
		return false
	}
	if d.PostingID != "" && !validID(ctx, d.PostingID) {
		return false
	}
	if d.Status == "matched" {
		return d.Kind == "statement" && d.PostingID != ""
	}
	return d.PostingID == ""
}
func (s *Service) Workspace(ctx context.Context, scope Scope) (Workspace, error) {
	if !validScope(ctx, scope) {
		return Workspace{}, ErrInvalid
	}
	return s.repository.Workspace(ctx, scope)
}
func (s *Service) Page(ctx context.Context, scope Scope, id, cursor string) (RunPage, error) {
	if !validScope(ctx, scope) || !validID(ctx, id) {
		return RunPage{}, ErrInvalid
	}
	return s.repository.Page(ctx, scope, id, cursor)
}
func (s *Service) Source(ctx context.Context, scope Scope, id string) (SourceFile, error) {
	if !validScope(ctx, scope) || !validID(ctx, id) {
		return SourceFile{}, ErrInvalid
	}
	return s.repository.Source(ctx, scope, id)
}
func (s *Service) Export(ctx context.Context, scope Scope, id string) ([]byte, error) {
	if !validScope(ctx, scope) || !validID(ctx, id) {
		return nil, ErrInvalid
	}
	snapshot, err := s.repository.Snapshot(ctx, scope, id)
	if err != nil {
		return nil, err
	}
	return ExportSnapshot(snapshot)
}
