// Package guidance builds truthful first-use and explainability read models
// from persisted evidence. Missing records are never promoted to success.
package guidance

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
)

var (
	ErrEvidenceUnavailable = errors.New("guided evidence unavailable")
	ErrTransferNotFound    = errors.New("guided transfer not found")
	canonicalUUID          = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type DurableReference struct {
	ID         string
	OccurredAt time.Time
}

type OrientationFacts struct {
	AuthorizedAccount  *DurableReference
	CreatedAccount     *DurableReference
	PostedTransfer     *DurableReference
	AuthorizedTransfer *DurableReference
	ReconciliationRun  *DurableReference
	DeliveryAttempt    *DurableReference
}

type EvidenceItem struct {
	EvidenceType   string     `json:"evidence_type"`
	EvidenceID     string     `json:"evidence_id,omitempty"`
	RelatedID      string     `json:"related_id,omitempty"`
	Status         string     `json:"status,omitempty"`
	EventType      string     `json:"event_type,omitempty"`
	AccountID      string     `json:"account_id,omitempty"`
	Direction      string     `json:"direction,omitempty"`
	AmountMinor    string     `json:"amount_minor,omitempty"`
	Currency       string     `json:"currency,omitempty"`
	BalanceVersion string     `json:"balance_version,omitempty"`
	AttemptNumber  string     `json:"attempt_number,omitempty"`
	OccurredAt     *time.Time `json:"occurred_at,omitempty"`
}

type EvidenceLink struct {
	Items       []EvidenceItem
	Unavailable bool
	Truncated   bool
}

type TransferFacts struct {
	TransferID      string
	Request         EvidenceLink
	Transfer        EvidenceLink
	JournalPostings EvidenceLink
	BalanceVersions EvidenceLink
	Outbox          EvidenceLink
	Delivery        EvidenceLink
	Reconciliation  EvidenceLink
}

type Repository interface {
	Orientation(context.Context, string, string) (OrientationFacts, error)
	ExplainTransfer(context.Context, string, string, string) (TransferFacts, error)
}

type RecoveryIndex interface {
	Snapshot(context.Context) (apprecovery.ManifestSnapshot, error)
}

type Service struct {
	repository Repository
	recovery   RecoveryIndex
	clock      func() time.Time
}

func NewService(repository Repository, recovery RecoveryIndex, clock func() time.Time) (*Service, error) {
	if repository == nil || recovery == nil {
		return nil, errors.New("guidance repositories are required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository: repository, recovery: recovery, clock: clock}, nil
}

type OrientationStep struct {
	ID           string     `json:"id"`
	State        string     `json:"state"`
	EvidenceType string     `json:"evidence_type"`
	EvidenceID   string     `json:"evidence_id,omitempty"`
	OccurredAt   *time.Time `json:"occurred_at,omitempty"`
	ReasonCode   string     `json:"reason_code,omitempty"`
}

type OrientationSummary struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	EvidenceState string            `json:"evidence_state"`
	Steps         []OrientationStep `json:"steps"`
}

func (s *Service) Orientation(ctx context.Context, tenantID, actorID string) (OrientationSummary, error) {
	if s == nil || ctx == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return OrientationSummary{}, ErrEvidenceUnavailable
	}
	facts, err := s.repository.Orientation(ctx, tenantID, actorID)
	if err != nil {
		return OrientationSummary{}, errors.Join(ErrEvidenceUnavailable, err)
	}
	steps := []OrientationStep{
		orientationStep("inspect_account", "account_record", facts.AuthorizedAccount, false, "no_authorized_account", "browser_action_not_recorded"),
		orientationStep("create_account", "account_created_audit", facts.CreatedAccount, true, "no_account_creation_evidence", ""),
		orientationStep("fund_account", "posted_transfer", facts.PostedTransfer, true, "no_posted_transfer", ""),
		orientationStep("inspect_transfer", "transfer_record", facts.AuthorizedTransfer, false, "no_authorized_transfer", "browser_action_not_recorded"),
		orientationStep("run_reconciliation", "reconciliation_run", facts.ReconciliationRun, true, "no_reconciliation_run", ""),
		orientationStep("inspect_delivery", "delivery_attempt", facts.DeliveryAttempt, false, "no_delivery_attempt", "browser_action_not_recorded"),
	}
	backupStep := OrientationStep{ID: "create_backup", State: "missing", EvidenceType: "recovery_backup", ReasonCode: "no_validated_backup"}
	snapshot, recoveryErr := s.recovery.Snapshot(ctx)
	if recoveryErr != nil {
		backupStep.State, backupStep.ReasonCode = "unavailable", "recovery_evidence_unavailable"
	} else if snapshot.LatestBackup != nil {
		occurredAt, parseErr := time.Parse(time.RFC3339Nano, snapshot.LatestBackup.FinalizedAtUTC)
		if parseErr != nil {
			backupStep.State, backupStep.ReasonCode = "unavailable", "recovery_evidence_unavailable"
		} else {
			backupStep.State, backupStep.EvidenceID, backupStep.OccurredAt, backupStep.ReasonCode = "completed", snapshot.LatestBackup.BackupID, utcPointer(occurredAt), ""
		}
	}
	steps = append(steps, backupStep)
	state := "complete"
	for _, step := range steps {
		// evidence_available means a record can be inspected, not that the
		// browser inspection action happened. Keep the aggregate partial rather
		// than manufacturing checklist completion from record existence.
		if step.State != "completed" {
			state = "partial"
			break
		}
	}
	return OrientationSummary{GeneratedAt: s.clock().UTC(), EvidenceState: state, Steps: steps}, nil
}

func orientationStep(id, evidenceType string, reference *DurableReference, actionProvable bool, missingReason, availableReason string) OrientationStep {
	step := OrientationStep{ID: id, State: "missing", EvidenceType: evidenceType, ReasonCode: missingReason}
	if reference == nil || !canonicalUUID.MatchString(strings.ToLower(reference.ID)) || reference.OccurredAt.IsZero() {
		return step
	}
	step.State, step.EvidenceID, step.OccurredAt = "evidence_available", strings.ToLower(reference.ID), utcPointer(reference.OccurredAt)
	step.ReasonCode = availableReason
	if actionProvable {
		step.State, step.ReasonCode = "completed", ""
	}
	return step
}

type TimelineStage struct {
	Sequence   int            `json:"sequence"`
	Kind       string         `json:"kind"`
	State      string         `json:"state"`
	ReasonCode string         `json:"reason_code,omitempty"`
	Truncated  bool           `json:"truncated"`
	Evidence   []EvidenceItem `json:"evidence"`
}

type ExplainabilityTimeline struct {
	TransferID    string          `json:"transfer_id"`
	GeneratedAt   time.Time       `json:"generated_at"`
	EvidenceState string          `json:"evidence_state"`
	Stages        []TimelineStage `json:"stages"`
}

func (s *Service) ExplainTransfer(ctx context.Context, tenantID, actorID, transferID string) (ExplainabilityTimeline, error) {
	if s == nil || ctx == nil || !canonicalUUID.MatchString(strings.ToLower(transferID)) || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return ExplainabilityTimeline{}, ErrTransferNotFound
	}
	facts, err := s.repository.ExplainTransfer(ctx, tenantID, actorID, strings.ToLower(transferID))
	if err != nil {
		return ExplainabilityTimeline{}, err
	}
	stages := []TimelineStage{
		stage(1, "request", facts.Request, "no_retained_idempotency_outcome"),
		stage(2, "transfer", facts.Transfer, "dependency_unavailable"),
		stage(3, "journal_postings", facts.JournalPostings, "no_journal"),
		stage(4, "balance_versions", facts.BalanceVersions, "no_balance_version_evidence"),
		stage(5, "outbox", facts.Outbox, "no_outbox_events"),
		stage(6, "delivery", facts.Delivery, "no_delivery_attempts"),
		stage(7, "reconciliation", facts.Reconciliation, "coverage_not_provable"),
	}
	if len(stages[2].Evidence) == 1 && stages[2].Evidence[0].EvidenceType == "journal" {
		stages[2].State, stages[2].ReasonCode = "missing", "no_postings"
	}
	state := "complete"
	for _, item := range stages {
		if item.State != "available" || item.Truncated {
			state = "partial"
			break
		}
	}
	return ExplainabilityTimeline{TransferID: strings.ToLower(transferID), GeneratedAt: s.clock().UTC(), EvidenceState: state, Stages: stages}, nil
}

func stage(sequence int, kind string, link EvidenceLink, missingReason string) TimelineStage {
	result := TimelineStage{Sequence: sequence, Kind: kind, State: "missing", ReasonCode: missingReason, Truncated: link.Truncated, Evidence: []EvidenceItem{}}
	if link.Unavailable {
		result.State, result.ReasonCode = "unavailable", "dependency_unavailable"
		return result
	}
	if len(link.Items) == 0 {
		return result
	}
	result.State, result.ReasonCode = "available", ""
	result.Evidence = append(result.Evidence, link.Items...)
	sort.SliceStable(result.Evidence, func(left, right int) bool {
		if result.Evidence[left].OccurredAt == nil {
			return false
		}
		if result.Evidence[right].OccurredAt == nil {
			return true
		}
		if result.Evidence[left].OccurredAt.Equal(*result.Evidence[right].OccurredAt) {
			return result.Evidence[left].EvidenceID < result.Evidence[right].EvidenceID
		}
		return result.Evidence[left].OccurredAt.Before(*result.Evidence[right].OccurredAt)
	})
	if link.Truncated {
		result.ReasonCode = "evidence_truncated"
	}
	return result
}

func utcPointer(value time.Time) *time.Time {
	result := value.UTC()
	return &result
}
