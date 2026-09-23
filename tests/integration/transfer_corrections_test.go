package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

const controlledCorrectionFunction = `public.controlled_post_transfer_correction_v1(uuid,text,uuid,text,uuid,timestamptz,timestamptz)`

func TestControlledCorrectionFunctionUsesFixedDefinerBoundary(t *testing.T) {
	_, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)

	var owner, searchPath string
	var securityDefiner, apiCanExecute, supportCanExecute, ownerCanUpdateCorrection, ownerCanWriteLedger, publicCanExecute bool
	err := database.QueryRowContext(context.Background(), `
SELECT owner.rolname,
       procedure.prosecdef,
       COALESCE((SELECT setting FROM unnest(procedure.proconfig) setting WHERE setting LIKE 'search_path=%'),''),
       has_function_privilege('ledgersync_api',$1,'EXECUTE'),
       has_function_privilege('ledgersync_support_readonly',$1,'EXECUTE'),
       has_table_privilege('ledgersync_migration_owner','public.transfer_corrections','UPDATE'),
       has_table_privilege('ledgersync_migration_owner','public.ledger_postings','INSERT'),
       EXISTS(
         SELECT 1 FROM aclexplode(procedure.proacl) acl
         WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'
       )
FROM pg_proc procedure
JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
JOIN pg_roles owner ON owner.oid=procedure.proowner
WHERE namespace.nspname='public' AND procedure.proname='controlled_post_transfer_correction_v1'`, controlledCorrectionFunction).
		Scan(&owner, &securityDefiner, &searchPath, &apiCanExecute, &supportCanExecute, &ownerCanUpdateCorrection, &ownerCanWriteLedger, &publicCanExecute)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ledgersync_migration_owner" || !securityDefiner || searchPath != "search_path=pg_catalog, public" || !apiCanExecute || supportCanExecute || !ownerCanUpdateCorrection || !ownerCanWriteLedger || publicCanExecute {
		t.Fatalf("unsafe controlled correction metadata owner=%q definer=%t search_path=%q api=%t support=%t owner_correction_update=%t owner_ledger_write=%t public=%t", owner, securityDefiner, searchPath, apiCanExecute, supportCanExecute, ownerCanUpdateCorrection, ownerCanWriteLedger, publicCanExecute)
	}
}

func TestControlledCorrectionFunctionExecutesAsAPIAndRejectsSpoofing(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	requireWorkloadRoles(t, database)
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
	const approver = "controlled-correction-approver"
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance')`, testTenantID, approver); err != nil {
		t.Fatal(err)
	}
	original, err := transferService.Submit(ctx, transferCommand(t, "controlled-correction-original-0001", "10.00"))
	if err != nil {
		t.Fatal(err)
	}
	repository, err := db.NewTransferCorrectionRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service, err := appcorrections.NewService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Request(ctx, appcorrections.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID,
		ReasonCode: "operational_error", OperatorNote: "Controlled function boundary evidence.",
		IdempotencyKey: "controlled-correction-request-0001", CorrelationID: "00000000-0000-4000-8000-000000008601",
		StepUpAuthenticatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Approve(ctx, appcorrections.DecisionCommand{
		TenantID: testTenantID, ActorSubjectID: approver, CorrectionID: created.Event.CorrectionID,
		Reason: "Independent evidence review completed.", CorrelationID: "00000000-0000-4000-8000-000000008602",
		StepUpAuthenticatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	api := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_api")
	support := provisionWorkloadSession(t, database, testDatabaseURL(t), "ledgersync_support_readonly")
	const postKey = "controlled-correction-post-0001"
	const correlationID = "00000000-0000-4000-8000-000000008603"
	var replayed bool
	if err = api.db.QueryRowContext(ctx, `SELECT replayed FROM public.controlled_post_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7)`,
		testTenantID, approver, created.Event.CorrectionID, postKey, correlationID, now, now).Scan(&replayed); err != nil || replayed {
		t.Fatalf("API controlled correction post replayed=%t error=%v", replayed, err)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers WHERE compensation_of_transfer_id=$1 AND status='posted'`, original.Result.TransferID) != 1 ||
		countRows(t, database, `SELECT count(*) FROM ledger_postings posting JOIN journal_transactions journal ON journal.id=posting.journal_transaction_id JOIN transfers transfer ON transfer.id=journal.transfer_id WHERE transfer.compensation_of_transfer_id=$1`, original.Result.TransferID) != 2 {
		t.Fatal("controlled correction post did not commit one exact compensation journal")
	}

	if _, err = api.db.ExecContext(ctx, `SELECT * FROM public.controlled_post_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7)`,
		testTenantID, "spoofed-finance-actor", created.Event.CorrectionID, postKey, correlationID, now, now); sqlState(err) != "42501" {
		t.Fatalf("spoofed actor SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}
	if _, err = support.db.ExecContext(ctx, `SELECT * FROM public.controlled_post_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7)`,
		testTenantID, approver, created.Event.CorrectionID, postKey, correlationID, now, now); sqlState(err) != "42501" {
		t.Fatalf("support execution SQLSTATE=%s error=%v, want 42501", sqlState(err), err)
	}

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`CREATE TEMP TABLE transfer_corrections(id uuid); SET LOCAL search_path=pg_temp,public`); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(`SELECT replayed FROM public.controlled_post_transfer_correction_v1($1,$2,$3,$4,$5,$6,$7)`,
		testTenantID, approver, created.Event.CorrectionID, postKey, correlationID, now, now).Scan(&replayed); err != nil || !replayed {
		t.Fatalf("fixed search path replayed=%t error=%v", replayed, err)
	}
}

func TestTransferCorrectionIsExactAdditiveApprovedAndReplaySafe(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	if _, err := database.ExecContext(ctx, `UPDATE tenant_transfer_policies SET control_mode='local_demo_single_operator',requires_step_up=false,approval_ttl_minutes=60 WHERE tenant_id=$1`, testTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,'correction-finance','finance')`, testTenantID); err != nil {
		t.Fatal(err)
	}
	original, err := transferService.Submit(ctx, transferCommand(t, "correction-original-0001", "25.00"))
	if err != nil || original.Result.Status != "posted" {
		t.Fatalf("original=%#v err=%v", original, err)
	}
	repository, err := db.NewTransferCorrectionRepository(database, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	service, err := appcorrections.NewService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	request := appcorrections.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID,
		ReasonCode: "wrong_destination", OperatorNote: "Customer supplied the wrong destination account.",
		IdempotencyKey: "correction-request-0001", CorrelationID: "00000000-0000-0000-0000-000000000901",
	}
	created, err := service.Request(ctx, request)
	if err != nil || created.Replayed || created.Event.Status != "requested" || created.Event.PolicyVersion == "" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	replayed, err := service.Request(ctx, request)
	if err != nil || !replayed.Replayed || replayed.Event.CorrectionID != created.Event.CorrectionID {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	approved, err := service.Approve(ctx, appcorrections.DecisionCommand{TenantID: testTenantID, ActorSubjectID: "correction-finance", CorrectionID: created.Event.CorrectionID, Reason: "Evidence reviewed and exact reversal approved.", CorrelationID: "00000000-0000-0000-0000-000000000902"})
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approved=%#v err=%v", approved, err)
	}
	posted, err := service.Post(ctx, appcorrections.PostCommand{TenantID: testTenantID, ActorSubjectID: "correction-finance", CorrectionID: created.Event.CorrectionID, IdempotencyKey: "correction-post-0000903", CorrelationID: "00000000-0000-0000-0000-000000000903"})
	if err != nil || posted.Replayed || posted.Event.Status != "posted" || posted.Event.OriginalJournalID == "" || posted.Event.CompensationJournalID == "" || posted.Event.OriginalJournalID == posted.Event.CompensationJournalID {
		t.Fatalf("posted=%#v err=%v", posted, err)
	}
	postReplay, err := service.Post(ctx, appcorrections.PostCommand{TenantID: testTenantID, ActorSubjectID: "correction-finance", CorrectionID: created.Event.CorrectionID, IdempotencyKey: "correction-post-0000903", CorrelationID: "00000000-0000-0000-0000-000000000904"})
	if err != nil || !postReplay.Replayed || postReplay.Event.CompensationTransferID != posted.Event.CompensationTransferID {
		t.Fatalf("post replay=%#v err=%v", postReplay, err)
	}
	if _, err = service.Post(ctx, appcorrections.PostCommand{TenantID: testTenantID, ActorSubjectID: "correction-finance", CorrectionID: created.Event.CorrectionID, IdempotencyKey: "correction-post-different-0903", CorrelationID: "00000000-0000-0000-0000-000000000904"}); !errors.Is(err, appcorrections.ErrConflict) {
		t.Fatalf("post with mismatched idempotency key error=%v", err)
	}
	var source, destination int64
	if err = database.QueryRowContext(ctx, `SELECT ledger_minor FROM account_balance_projections WHERE account_id=$1`, testSourceID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRowContext(ctx, `SELECT ledger_minor FROM account_balance_projections WHERE account_id=$1`, testDestinationID).Scan(&destination); err != nil {
		t.Fatal(err)
	}
	if source != 10_000 || destination != 2_000 {
		t.Fatalf("reversal drifted balances source=%d destination=%d", source, destination)
	}
	if countRows(t, database, `SELECT count(*) FROM transfers WHERE id IN ($1,$2) AND status='posted'`, original.Result.TransferID, posted.Event.CompensationTransferID) != 2 ||
		countRows(t, database, `SELECT count(*) FROM journal_transactions WHERE transfer_id IN ($1,$2)`, original.Result.TransferID, posted.Event.CompensationTransferID) != 2 ||
		countRows(t, database, `SELECT count(*) FROM ledger_postings posting JOIN journal_transactions journal ON journal.id=posting.journal_transaction_id WHERE journal.transfer_id IN ($1,$2)`, original.Result.TransferID, posted.Event.CompensationTransferID) != 4 ||
		countRows(t, database, `SELECT count(*) FROM transfer_corrections WHERE id=$1 AND status='posted'`, created.Event.CorrectionID) != 1 {
		t.Fatal("correction evidence was partial or duplicated")
	}
	investigationRepository, err := db.NewInvestigationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := investigationRepository.GetTransfer(ctx, testTenantID, original.Result.TransferID)
	if err != nil || detail.CorrectionRole != "original" || detail.CorrectionStatus != "posted" || detail.OriginalJournalID != posted.Event.OriginalJournalID || detail.CompensationJournalID != posted.Event.CompensationJournalID {
		t.Fatalf("original correction detail=%#v err=%v", detail, err)
	}
	compensationDetail, err := investigationRepository.GetTransfer(ctx, testTenantID, posted.Event.CompensationTransferID)
	if err != nil || compensationDetail.CorrectionRole != "compensation" || compensationDetail.OriginalTransferID != original.Result.TransferID {
		t.Fatalf("compensation detail=%#v err=%v", compensationDetail, err)
	}
	historyRepository, err := db.NewTransactionHistoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	history, _, err := historyRepository.ListAccountHistory(ctx, testTenantID, testActorID, testSourceID, "", 10)
	if err != nil || len(history) < 2 || history[0].CorrectionRole != "compensation" || history[1].CorrectionRole != "original" {
		t.Fatalf("compensated account history=%#v err=%v", history, err)
	}
	_, err = service.Request(ctx, appcorrections.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID, ReasonCode: "duplicate", OperatorNote: "A second correction must never be created.", IdempotencyKey: "correction-request-0002", CorrelationID: "00000000-0000-0000-0000-000000000905"})
	if !errors.Is(err, appcorrections.ErrConflict) {
		t.Fatalf("second correction error=%v", err)
	}
}

func TestProductionCorrectionRequiresRecentStepUpAndIndependentApproval(t *testing.T) {
	transferService, database := requireTransferService(t, 10_000)
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,'correction-finance','finance')`, testTenantID); err != nil {
		t.Fatal(err)
	}
	original, err := transferService.Submit(ctx, transferCommand(t, "correction-production-0001", "10.00"))
	if err != nil {
		t.Fatal(err)
	}
	repository, _ := db.NewTransferCorrectionRepository(database, func() time.Time { return now })
	service, _ := appcorrections.NewService(repository, func() time.Time { return now })
	request := appcorrections.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID, ReasonCode: "operational_error", OperatorNote: "Reviewed operational correction evidence.", IdempotencyKey: "correction-production-0002", CorrelationID: "00000000-0000-0000-0000-000000000911"}
	if _, err = service.Request(ctx, request); !errors.Is(err, appcorrections.ErrStepUpRequired) {
		t.Fatalf("missing step-up error=%v", err)
	}
	request.StepUpAuthenticatedAt = now.Add(-time.Minute)
	created, err := service.Request(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Approve(ctx, appcorrections.DecisionCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrectionID: created.Event.CorrectionID, Reason: "self approval", CorrelationID: "00000000-0000-0000-0000-000000000912", StepUpAuthenticatedAt: now})
	if !errors.Is(err, appcorrections.ErrForbidden) {
		t.Fatalf("self approval error=%v", err)
	}
	_, err = service.Reject(ctx, appcorrections.DecisionCommand{TenantID: testTenantID, ActorSubjectID: "correction-finance", CorrectionID: created.Event.CorrectionID, Reason: "independent rejection without step-up", CorrelationID: "00000000-0000-0000-0000-000000000913"})
	if !errors.Is(err, appcorrections.ErrStepUpRequired) {
		t.Fatalf("rejection without step-up error=%v", err)
	}
	approved, err := service.Approve(ctx, appcorrections.DecisionCommand{TenantID: testTenantID, ActorSubjectID: "correction-finance", CorrectionID: created.Event.CorrectionID, Reason: "independent evidence review", CorrelationID: "00000000-0000-0000-0000-000000000913", StepUpAuthenticatedAt: now})
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approval=%#v err=%v", approved, err)
	}
}
