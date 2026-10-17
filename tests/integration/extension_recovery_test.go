package integration_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
	statement "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/statementreconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/redis/go-redis/v9"
)

type extensionRecoveryReference struct {
	ExpiryOwner      string
	ExpiryGeneration int64
	Tables           []recoveryTable
	ActiveCreate     hold.Create
	ActiveID         string
	CapturedCreate   hold.Create
	CaptureCommand   hold.Decide
	CaptureReceipt   hold.Receipt
	ImportCommand    statement.ImportCommand
	ImportID         string
	SourceSHA256     string
	PreparedCommand  statement.RunCommand
	PreparedID       string
	FinancialHash    string
	SourceVersion    int64
	Journals         int
}

func TestPortfolioExtensionRecoverySeed(t *testing.T) {
	root := recoveryControl(t)
	admin, api, account, reference, posting := statementFixture(t)
	ctx := context.Background()
	holdRepo, err := db.NewReservationRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	holds, err := hold.NewService(holdRepo)
	if err != nil {
		t.Fatal(err)
	}
	var destination string
	if err := admin.QueryRow(`SELECT credit_account_id FROM transfers WHERE id=$1`, reference).Scan(&destination); err != nil {
		t.Fatal(err)
	}
	create := func(key string, ttl int) hold.Create {
		return hold.Create{Meta: hold.Meta{Scope: hold.Scope{TenantID: testTenantID, ActorSubjectID: testActorID}, IdempotencyKey: key, CorrelationID: uuid.NewString()}, DebitAccountID: account, CreditAccountID: destination, AmountMinor: "100", Currency: "INR", TTLSeconds: ttl}
	}
	activeCommand, captureCreate := create("extension-recovery-active-hold", 1), create("extension-recovery-capture-hold", 3600)
	active, err := holds.Create(ctx, activeCommand)
	if err != nil {
		t.Fatal(err)
	}
	captureHold, err := holds.Create(ctx, captureCreate)
	if err != nil {
		t.Fatal(err)
	}
	captureCommand := hold.Decide{Meta: hold.Meta{Scope: activeCommand.Scope, IdempotencyKey: "extension-recovery-original-capture", CorrelationID: uuid.NewString()}, HoldID: captureHold.ID, Decision: "capture"}
	captured, err := holds.Decide(ctx, captureCommand)
	if err != nil || captured.State != "captured" {
		t.Fatal("seed capture", err)
	}
	mapping := statementMustCall(t, api, statementMapSQL, testTenantID, testActorID, uuid.NewString(), uuid.NewString(), account, "recovery-source")
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	raw := []byte(statement.Header + "\n" + fmt.Sprintf("recovery-row,recovery-source,%s,%s,%s,%s,debit,100,INR,1000,900\n", start.Format(time.RFC3339), end.Format(time.RFC3339), now.Format(time.RFC3339), reference))
	sr, err := db.NewStatementReconciliationRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := statement.NewService(sr)
	if err != nil {
		t.Fatal(err)
	}
	meta := statement.Meta{Scope: statement.Scope{TenantID: testTenantID, ActorSubjectID: testActorID}, IdempotencyKey: "extension-recovery-original-import", CorrelationID: uuid.NewString()}
	importCommand := statement.ImportCommand{Meta: meta, MappingID: mapping.ID, Source: raw}
	accepted, err := statements.Import(ctx, importCommand)
	if err != nil {
		t.Fatal(err)
	}
	run, err := statements.Run(ctx, statement.RunCommand{Meta: statement.Meta{Scope: meta.Scope, IdempotencyKey: "extension-recovery-completed-run", CorrelationID: uuid.NewString()}, ImportID: accepted.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = statements.Review(ctx, statement.ReviewCommand{Meta: statement.Meta{Scope: meta.Scope, IdempotencyKey: "extension-recovery-reviewed-row", CorrelationID: uuid.NewString()}, RunID: run.ID, Decision: statement.ReviewInput{Kind: "statement", ItemID: "recovery-row", ExpectedVersion: "0", Status: "matched", Assignee: testActorID, Reason: "confirmed", Comment: "Synthetic retained recovery decision", Evidence: "/transfers/" + reference, PostingID: posting}})
	if err != nil {
		t.Fatal(err)
	}
	secondMeta := meta
	secondMeta.IdempotencyKey = "extension-recovery-prepared-source"
	secondImport, err := statements.Import(ctx, statement.ImportCommand{Meta: secondMeta, MappingID: mapping.ID, Source: []byte(strings.Replace(string(raw), "recovery-row,", "recovery-prepared,", 1))})
	if err != nil {
		t.Fatal(err)
	}
	preparedCommand := statement.RunCommand{Meta: statement.Meta{Scope: meta.Scope, IdempotencyKey: "extension-recovery-prepared-run", CorrelationID: uuid.NewString()}, ImportID: secondImport.ID}
	prepared, err := sr.PrepareRun(ctx, preparedCommand)
	if err != nil {
		t.Fatal(err)
	}
	var version int64
	if err := admin.QueryRow(`SELECT balance_version FROM account_balance_projections WHERE account_id=$1`, account).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if active.ExpiresAt == nil {
		t.Fatal("active hold deadline missing")
	}
	if delay := time.Until(*active.ExpiresAt) + 100*time.Millisecond; delay > 0 {
		if delay > 2*time.Second {
			t.Fatal("unexpected source expiry lifetime")
		}
		time.Sleep(delay)
	}
	sourceWorker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker").db
	var claimedID, claimedTenant string
	var claimedGeneration int64
	if err := sourceWorker.QueryRow(`SELECT hold_id,tenant_id,generation FROM controlled_claim_hold_expiry_v1(1,'extension-pre-backup',1)`).Scan(&claimedID, &claimedTenant, &claimedGeneration); err != nil || claimedID != active.ID || claimedTenant != testTenantID {
		t.Fatal("source expiry claim differs", err)
	}
	referenceData := extensionRecoveryReference{ExpiryOwner: "extension-pre-backup", ExpiryGeneration: claimedGeneration, Tables: recoveryTables(t, ctx, admin), ActiveCreate: activeCommand, ActiveID: active.ID, CapturedCreate: captureCreate, CaptureCommand: captureCommand, CaptureReceipt: captured, ImportCommand: importCommand, ImportID: accepted.ID, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), PreparedCommand: preparedCommand, PreparedID: prepared.ID, FinancialHash: statementFinancialHash(t, admin), SourceVersion: version, Journals: countRows(t, admin, `SELECT count(*) FROM journal_transactions`)}
	encoded, err := json.Marshal(referenceData)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(root, "extension-recovery-reference.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("EXTENSION_RECOVERY_FIXTURE tables=%d source=immutable prepared_run=retained review=retained active_hold=reserved captured_hold=terminal", len(referenceData.Tables))
}
func TestPortfolioExtensionRecoveryRestored(t *testing.T) {
	root := recoveryControl(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var expected extensionRecoveryReference
	raw, err := os.ReadFile(filepath.Join(root, "extension-recovery-reference.json"))
	if err != nil || json.Unmarshal(raw, &expected) != nil {
		t.Fatal("extension reference required", err)
	}
	admin, err := sql.Open("pgx", testDatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	// Role-session cleanup must finish before its administrative connection closes.
	t.Cleanup(func() { _ = admin.Close() })
	actual, _ := json.Marshal(recoveryTables(t, ctx, admin))
	wanted, _ := json.Marshal(expected.Tables)
	if string(actual) != string(wanted) {
		t.Fatal("complete extension restore row hashes differ")
	}
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api").db
	hr, err := db.NewReservationRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	holds, err := hold.NewService(hr)
	if err != nil {
		t.Fatal(err)
	}
	active, err := holds.Create(ctx, expected.ActiveCreate)
	if err != nil || !active.Replayed || active.ID != expected.ActiveID {
		t.Fatal("restored active original key differs", err)
	}
	captured, err := holds.Decide(ctx, expected.CaptureCommand)
	if err != nil || !captured.Replayed || captured.TransferID != expected.CaptureReceipt.TransferID {
		t.Fatal("restored capture posted again", err)
	}
	sr, err := db.NewStatementReconciliationRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := statement.NewService(sr)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := statements.Import(ctx, expected.ImportCommand)
	if err != nil || !imported.Replayed || imported.ID != expected.ImportID {
		t.Fatal("restored source key differs", err)
	}
	source, err := statements.Source(ctx, expected.ImportCommand.Scope, expected.ImportID)
	if err != nil || source.SHA256 != expected.SourceSHA256 || string(source.Bytes) != string(expected.ImportCommand.Source) {
		t.Fatal("restored immutable source differs", err)
	}
	resumed, err := statements.Run(ctx, expected.PreparedCommand)
	if err != nil || resumed.ID != expected.PreparedID || !resumed.Replayed {
		t.Fatal("restored fixed snapshot did not resume", err)
	}
	if expected.FinancialHash != statementFinancialHash(t, admin) {
		t.Fatal("extension original retries changed money")
	}
	if countRows(t, admin, `SELECT count(*) FROM statement_review_events WHERE decision->>'comment'='Synthetic retained recovery decision'`) != 1 {
		t.Fatal("restored review decision lost")
	}
	current, err := holds.Read(ctx, expected.ActiveCreate.Scope, expected.ActiveID)
	if err != nil || current.State != "active" {
		t.Fatal("active obligation was inferred terminal", err)
	}
	if delay := time.Until(current.ExpiresAt) + 100*time.Millisecond; delay > 0 {
		if delay > 2*time.Second {
			t.Fatal("unexpected expiry lifetime")
		}
		time.Sleep(delay)
	}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")})
	defer func() { _ = client.Close() }()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	projection, err := cache.NewBalanceCache(client, "", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cacheReader := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_reconciliation").db
	balanceReader, err := db.NewBalanceRepository(cacheReader)
	if err != nil {
		t.Fatal(err)
	}
	balances, err := balanceReader.ListCurrentForTenant(ctx, testTenantID)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := cache.NewAccountAdapter(projection)
	if err != nil {
		t.Fatal(err)
	}
	for _, balance := range balances {
		if _, err := adapter.Put(ctx, balance); err != nil {
			t.Fatal(err)
		}
	}
	rebuilt, err := projection.Get(ctx, testTenantID, expected.ActiveCreate.DebitAccountID)
	if err != nil || rebuilt.Version != expected.SourceVersion || rebuilt.LedgerMinor-rebuilt.AvailableMinor != 100 {
		t.Fatal("active reserved cache rebuild differs", err)
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker").db
	var expiredLease bool
	for n := 0; n < 40; n++ {
		if err := admin.QueryRow(`SELECT claimed_until<=clock_timestamp() FROM hold_expiry_jobs WHERE hold_id=$1`, expected.ActiveID).Scan(&expiredLease); err != nil {
			t.Fatal(err)
		}
		if expiredLease {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !expiredLease {
		t.Fatal("source lease did not expire within its real bounded lifetime")
	}
	staleErr := db.WithTenantSerializableSequence(ctx, worker, db.TenantScope{TenantID: testTenantID, ActorID: "system:hold-expiry"}, "transfer-policy|"+testTenantID, 5, func(tx *sql.Tx) error {
		var changed bool
		return tx.QueryRow(`SELECT controlled_expire_hold_v1($1,$2,$3,$4)`, testTenantID, expected.ActiveID, expected.ExpiryOwner, expected.ExpiryGeneration).Scan(&changed)
	})
	if sqlState(staleErr) != "55000" {
		t.Fatal("restored stale lease was not fenced", staleErr)
	}
	expiry, err := db.NewReservationExpiryWorker(worker, "extension-recovery-worker", 10)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := expiry.RunOnce(ctx); err != nil || count != 1 {
		t.Fatal("restored obligation did not expire", count, err)
	}
	var restoredGeneration int64
	if err := admin.QueryRow(`SELECT claim_generation FROM hold_expiry_jobs WHERE hold_id=$1`, expected.ActiveID).Scan(&restoredGeneration); err != nil || restoredGeneration <= expected.ExpiryGeneration {
		t.Fatal("restored claim generation did not advance", err)
	}
	current, err = holds.Read(ctx, expected.ActiveCreate.Scope, expected.ActiveID)
	if err != nil || current.State != "expired" {
		t.Fatal("expiry state missing", err)
	}
	if countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != expected.Journals {
		t.Fatal("expiry or replay posted another journal")
	}
	var version int64
	if err := admin.QueryRow(`SELECT balance_version FROM account_balance_projections WHERE account_id=$1`, expected.ActiveCreate.DebitAccountID).Scan(&version); err != nil || version != expected.SourceVersion+1 {
		t.Fatal("expiry balance version differs", err)
	}
	recon := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_reconciliation").db
	reconciler, err := db.NewReconciliationRepository(recon)
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Reconcile(ctx, testTenantID, time.Now())
	if err != nil || result.MismatchCount != 0 {
		t.Fatal("restored reservation reconciliation differs", err)
	}
	runner, err := bootstrap.NewWorkerRunnerWithOptions(ctx, config.Config{Environment: "development", NodeID: "extension-recovery", OutboxBatchSize: 10}, worker, client, nil, bootstrap.WorkerRunnerOptions{NodeRole: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 100; n++ {
		if _, err := runner.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0 {
			break
		}
	}
	cached, err := projection.Get(ctx, testTenantID, expected.ActiveCreate.DebitAccountID)
	if err != nil || cached.Version != version || cached.AvailableMinor != cached.LedgerMinor {
		t.Fatal("mixed event restore projection differs", err)
	}
	t.Logf("EXTENSION_RECOVERY_RESTORED tables=%d source=exact snapshots=resumed reviews=retained original_keys=replayed active_obligation=expired_once journals=unchanged cache=mixed_v1_v2", len(expected.Tables))
}
