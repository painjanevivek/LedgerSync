package guidance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
)

const guidanceTransferID = "70000000-0000-4000-8000-000000000001"

type guidanceRepositoryStub struct {
	orientation OrientationFacts
	preference  OrientationPreference
	transfer    TransferFacts
	err         error
	update      func(PreferenceUpdate) (OrientationPreference, error)
}

func (s guidanceRepositoryStub) Orientation(context.Context, string, string) (OrientationFacts, error) {
	return s.orientation, s.err
}

func (s guidanceRepositoryStub) OrientationPreference(context.Context, string, string) (OrientationPreference, error) {
	return s.preference, s.err
}

func (s guidanceRepositoryStub) UpdateOrientationPreference(_ context.Context, _, _ string, update PreferenceUpdate) (OrientationPreference, error) {
	if s.update != nil {
		return s.update(update)
	}
	return s.preference, s.err
}

func (s guidanceRepositoryStub) ExplainTransfer(context.Context, string, string, string) (TransferFacts, error) {
	return s.transfer, s.err
}

type guidanceRecoveryStub struct {
	snapshot apprecovery.ManifestSnapshot
	err      error
}

func (s guidanceRecoveryStub) Snapshot(context.Context) (apprecovery.ManifestSnapshot, error) {
	return s.snapshot, s.err
}

func TestOrientationDistinguishesCompletedActionsFromAvailableInspectionEvidence(t *testing.T) {
	when := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	reference := func(suffix string) *DurableReference {
		return &DurableReference{ID: "70000000-0000-4000-8000-0000000000" + suffix, OccurredAt: when}
	}
	service, err := NewService(guidanceRepositoryStub{orientation: OrientationFacts{
		AuthorizedAccount: reference("01"), CreatedAccount: reference("02"), PostedTransfer: reference("03"), AuthorizedTransfer: reference("04"), ReconciliationRun: reference("05"), DeliveryAttempt: reference("06"),
	}}, guidanceRecoveryStub{snapshot: apprecovery.ManifestSnapshot{LatestBackup: &apprecovery.BackupManifestEvidence{BackupID: "backup-20260825T120000Z-abcdef0", FinalizedAtUTC: when.Format(time.RFC3339)}}}, func() time.Time { return when })
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Orientation(context.Background(), "tenant", "actor")
	if err != nil || summary.EvidenceState != "partial" || len(summary.Steps) != 12 {
		t.Fatalf("summary=%+v error=%v", summary, err)
	}
	for _, index := range []int{2, 6, 7, 9, 10} {
		if summary.Steps[index].State != "evidence_available" || summary.Steps[index].ReasonCode != "operator_confirmation_required" {
			t.Fatalf("inspection step fabricated completion: %+v", summary.Steps[index])
		}
	}
	for _, index := range []int{3, 5, 8, 11} {
		if summary.Steps[index].State != "completed" || summary.Steps[index].OccurredAt == nil {
			t.Fatalf("durable action not completed: %+v", summary.Steps[index])
		}
	}
	if summary.Steps[4].State != "unavailable" || summary.Steps[4].ReasonCode != "funding_workflow_unavailable" {
		t.Fatalf("funding availability was overstated: %+v", summary.Steps[4])
	}
}

func TestOrientationKeepsMissingAndUnavailableEvidenceExplicit(t *testing.T) {
	service, _ := NewService(guidanceRepositoryStub{}, guidanceRecoveryStub{err: errors.New("index unavailable")}, nil)
	summary, err := service.Orientation(context.Background(), "tenant", "actor")
	if err != nil || summary.EvidenceState != "partial" || len(summary.Steps) != 12 {
		t.Fatalf("summary=%+v error=%v", summary, err)
	}
	if summary.Steps[2].State != "missing" || summary.Steps[2].ReasonCode != "no_authorized_account" || summary.Steps[11].State != "unavailable" || summary.Steps[11].ReasonCode != "recovery_evidence_unavailable" {
		t.Fatalf("missing/unavailable truth drifted: %+v", summary.Steps)
	}
}

func TestOrientationPreferencesAreVersionedServerStateAndCannotBypassEvidence(t *testing.T) {
	when := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	reference := &DurableReference{ID: guidanceTransferID, OccurredAt: when}
	var captured PreferenceUpdate
	repository := guidanceRepositoryStub{
		orientation: OrientationFacts{AuthorizedAccount: reference, PostedTransfer: reference, DeliveryAttempt: reference},
		update: func(update PreferenceUpdate) (OrientationPreference, error) {
			captured = update
			return OrientationPreference{Dismissed: update.Dismissed, CompletedStepIDs: update.CompletedStepIDs, Version: 4, UpdatedAt: &when}, nil
		},
	}
	service, _ := NewService(repository, guidanceRecoveryStub{}, func() time.Time { return when })
	summary, err := service.UpdateOrientationPreferences(context.Background(), "tenant", "actor", "3", true, []string{"retry_transfer", "confirm_health", "inspect_accounts"})
	if err != nil || !summary.Dismissed || summary.PreferenceVersion != "4" || len(summary.OperatorCompletedSteps) != 3 {
		t.Fatalf("summary=%+v error=%v", summary, err)
	}
	if captured.ExpectedVersion != 3 || strings.Join(captured.CompletedStepIDs, ",") != "confirm_health,inspect_accounts,retry_transfer" {
		t.Fatalf("update was not normalized: %+v", captured)
	}
	for _, index := range []int{0, 2, 6} {
		if summary.Steps[index].State != "operator_confirmed" {
			t.Fatalf("preference completion missing at %d: %+v", index, summary.Steps[index])
		}
	}

	withoutEvidence, _ := NewService(guidanceRepositoryStub{}, guidanceRecoveryStub{}, nil)
	if _, err := withoutEvidence.UpdateOrientationPreferences(context.Background(), "tenant", "actor", "0", false, []string{"inspect_postings"}); !errors.Is(err, ErrInvalidPreference) {
		t.Fatalf("missing evidence preference error=%v", err)
	}
	if _, err := withoutEvidence.UpdateOrientationPreferences(context.Background(), "tenant", "actor", "0", false, []string{"fund_account"}); !errors.Is(err, ErrInvalidPreference) {
		t.Fatalf("automated/unavailable preference error=%v", err)
	}
}

func TestTimelineUsesSemanticStageOrderAndSortsOutOfOrderEvidence(t *testing.T) {
	earlier := time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Minute)
	facts := TransferFacts{TransferID: guidanceTransferID,
		Request:  EvidenceLink{Items: []EvidenceItem{{EvidenceType: "idempotency_outcome", Status: "completed", OccurredAt: &earlier}}},
		Transfer: EvidenceLink{Items: []EvidenceItem{{EvidenceType: "transfer", EvidenceID: guidanceTransferID, Status: "posted", OccurredAt: &earlier}}},
		JournalPostings: EvidenceLink{Items: []EvidenceItem{
			{EvidenceType: "posting", EvidenceID: "70000000-0000-4000-8000-000000000003", OccurredAt: &later},
			{EvidenceType: "journal", EvidenceID: "70000000-0000-4000-8000-000000000002", OccurredAt: &earlier},
		}},
		BalanceVersions: EvidenceLink{Items: []EvidenceItem{{EvidenceType: "balance_version", BalanceVersion: "1", OccurredAt: &earlier}}},
		Outbox:          EvidenceLink{Items: []EvidenceItem{{EvidenceType: "outbox_event", Status: "published", OccurredAt: &earlier}}},
		Delivery:        EvidenceLink{Unavailable: true},
		Reconciliation:  EvidenceLink{},
	}
	service, _ := NewService(guidanceRepositoryStub{transfer: facts}, guidanceRecoveryStub{}, func() time.Time { return later })
	timeline, err := service.ExplainTransfer(context.Background(), "tenant", "actor", guidanceTransferID)
	if err != nil || len(timeline.Stages) != 7 || timeline.EvidenceState != "partial" {
		t.Fatalf("timeline=%+v error=%v", timeline, err)
	}
	wantKinds := []string{"request", "transfer", "journal_postings", "balance_versions", "outbox", "delivery", "reconciliation"}
	for index, want := range wantKinds {
		if timeline.Stages[index].Sequence != index+1 || timeline.Stages[index].Kind != want {
			t.Fatalf("stage[%d]=%+v", index, timeline.Stages[index])
		}
	}
	if timeline.Stages[2].Evidence[0].EvidenceType != "journal" || timeline.Stages[5].State != "unavailable" || timeline.Stages[5].ReasonCode != "dependency_unavailable" || timeline.Stages[6].State != "missing" || timeline.Stages[6].ReasonCode != "coverage_not_provable" {
		t.Fatalf("out-of-order/partial evidence drifted: %+v", timeline.Stages)
	}
}

func TestTimelineCallsJournalWithoutPostingsMissingAndNeverAcceptsArbitraryID(t *testing.T) {
	when := time.Now().UTC()
	facts := TransferFacts{TransferID: guidanceTransferID, Transfer: EvidenceLink{Items: []EvidenceItem{{EvidenceType: "transfer", OccurredAt: &when}}}, JournalPostings: EvidenceLink{Items: []EvidenceItem{{EvidenceType: "journal", OccurredAt: &when}}}}
	service, _ := NewService(guidanceRepositoryStub{transfer: facts}, guidanceRecoveryStub{}, nil)
	timeline, err := service.ExplainTransfer(context.Background(), "tenant", "actor", guidanceTransferID)
	if err != nil || timeline.Stages[2].State != "missing" || timeline.Stages[2].ReasonCode != "no_postings" {
		t.Fatalf("journal-only stage=%+v error=%v", timeline.Stages[2], err)
	}
	if _, err := service.ExplainTransfer(context.Background(), "tenant", "actor", "../../transfer"); !errors.Is(err, ErrTransferNotFound) {
		t.Fatalf("invalid transfer ID error=%v", err)
	}
}
