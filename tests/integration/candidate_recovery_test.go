package integration_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/bootstrap"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
	"github.com/redis/go-redis/v9"
)

type recoveryTable struct {
	Name   string `json:"name"`
	Count  int64  `json:"count"`
	Digest string `json:"sha256"`
}
type candidateRecoveryReference struct {
	Tables                                                      []recoveryTable           `json:"tables"`
	Accounts                                                    map[string]oracle.Account `json:"accounts"`
	Bindings                                                    map[string]string         `json:"bindings"`
	Journals                                                    int                       `json:"journals"`
	Immutable                                                   map[string]string         `json:"immutable"`
	TransferID, ReceiptDigest, DeadEventID, ApprovalCorrelation string
}

func recoveryControl(t *testing.T) string {
	t.Helper()
	if os.Getenv("LEDGERSYNC_RUN_RECOVERY") != "true" {
		t.Skip("requires guarded candidate recovery coordinator")
	}
	return portfolioControlDirectory(t)
}
func recoveryTables(t *testing.T, ctx context.Context, database *sql.DB) []recoveryTable {
	t.Helper()
	rows, err := database.QueryContext(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(name) {
			t.Fatal("unexpected candidate table name")
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	result := []recoveryTable{}
	for _, name := range names {
		var encoded string
		var count int64
		if err := database.QueryRowContext(ctx, `SELECT count(*),COALESCE(jsonb_agg(to_jsonb(record) ORDER BY to_jsonb(record)::text)::text,'[]') FROM public.`+name+` record`).Scan(&count, &encoded); err != nil {
			t.Fatal(err)
		}
		result = append(result, recoveryTable{name, count, fmt.Sprintf("%x", sha256.Sum256([]byte(encoded)))})
	}
	return result
}

func TestPortfolioRecoverySeed(t *testing.T) {
	root := recoveryControl(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	suite := newFinancialModelSuite(t)
	if err := suite.reset(ctx); err != nil {
		t.Fatal(err)
	}
	for index, op := range []oracle.Operation{{Kind: "fund", Minor: 1000}, {Kind: "transfer", Minor: 500}, {Kind: "correct", Reference: 1}, {Kind: "transfer", Minor: 100}, {Kind: "lost-response", Minor: 200}} {
		if err := suite.step(ctx, 9010, index, op); err != nil {
			t.Fatal(err)
		}
	}
	amount, _ := money.New("USD", 25)
	if _, err := suite.funding.Request(ctx, appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testSourceID, Amount: amount, ExternalReference: "synthetic-pending-recovery", EvidenceReference: "synthetic://recovery/pending", IdempotencyKey: "recovery-pending-funding-request", CorrelationID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	repo, _ := db.NewTransferRepository(suite.api, time.Now)
	command := transferCommand(t, "candidate-recovery-original-key", "1.00")
	command.RequestReference = "50000000-0000-4000-8000-000000009101"
	if err := repo.PrepareRecovery(ctx, command); err != nil {
		t.Fatal(err)
	}
	result, err := suite.transfers.Submit(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	suite.ledger.Move(oracle.Intent{Tenant: testTenantID, Actor: testActorID, Operation: "transfer", Key: command.IdempotencyKey, From: "a", To: "b", Currency: "USD", Minor: "100"})
	pending := transferCommand(t, "candidate-recovery-unresolved-key", "0.01")
	pending.RequestReference = "50000000-0000-4000-8000-000000009102"
	if err := repo.PrepareRecovery(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err := suite.check(ctx); err != nil {
		t.Fatal(err)
	}
	worker := provisionWorkloadSession(t, suite.admin, testDatabaseURL(t), "ledgersync_worker")
	queue, _ := db.NewOutboxRepository(worker.db, time.Now)
	events, err := queue.Claim(ctx, "candidate-recovery-fixture", 100, time.Minute)
	if err != nil || len(events) < 1 {
		t.Fatal("recovery fixture has no committed delivery work", err)
	}
	for i, event := range events {
		if i == 0 {
			err = queue.MarkDead(ctx, "candidate-recovery-fixture", event.ID, time.Now(), "publish_failed", event.ClaimGeneration)
		} else {
			err = queue.Reschedule(ctx, "candidate-recovery-fixture", event.ID, time.Now(), "publish_failed", event.ClaimGeneration)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	replay, _ := db.NewOutboxReplayRepository(suite.api, time.Now)
	correlation := uuid.NewString()
	if err := replay.Approve(ctx, recovery.Approval{TenantID: testTenantID, ActorSubjectID: testActorID, EventID: events[0].ID, ReasonCode: "dependency_restored", CorrelationID: correlation}); err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(result.Result)
	reference := candidateRecoveryReference{Tables: recoveryTables(t, ctx, suite.admin), Accounts: suite.ledger.Accounts, Bindings: suite.bindings, Journals: suite.ledger.Journals, Immutable: suite.immutable, TransferID: result.Result.TransferID, ReceiptDigest: fmt.Sprintf("%x", sha256.Sum256(receipt)), DeadEventID: events[0].ID, ApprovalCorrelation: correlation}
	encoded, err := json.Marshal(reference)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(root, "recovery-reference.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(encoded)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal("recovery reference write failed")
	}
	t.Logf("RECOVERY_FIXTURE tables=%d journals=%d pending_delivery=%d", len(reference.Tables), reference.Journals, len(events))
}

func TestPortfolioRecoveryRestored(t *testing.T) {
	root := recoveryControl(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var expected candidateRecoveryReference
	encoded, err := os.ReadFile(filepath.Join(root, "recovery-reference.json"))
	if err != nil || json.Unmarshal(encoded, &expected) != nil {
		t.Fatal("complete recovery reference required")
	}
	admin, err := sql.Open("pgx", testDatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	admin.SetMaxOpenConns(2)
	actual, err := json.Marshal(recoveryTables(t, ctx, admin))
	if err != nil {
		t.Fatal(err)
	}
	wanted, _ := json.Marshal(expected.Tables)
	if string(actual) != string(wanted) {
		t.Fatal("restored complete table counts or row hashes differ")
	}
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	api.db.SetMaxOpenConns(2)
	if _, err := api.db.ExecContext(ctx, `UPDATE account_balance_projections SET ledger_minor=0`); sqlState(err) != "42501" {
		t.Fatal("restored API can write balances directly", err)
	}
	if _, err := api.db.QueryContext(ctx, `SELECT * FROM controlled_claim_outbox_v1(1,'forbidden-api',30)`); sqlState(err) != "42501" {
		t.Fatal("restored API can claim global work", err)
	}
	repo, _ := db.NewTransferRepository(api.db, time.Now)
	service, _ := transfers.NewService(repo, time.Now)
	original := transferCommand(t, "candidate-recovery-original-key", "1.00")
	result, err := service.Submit(ctx, original)
	if err != nil || !result.Replayed || result.Result.TransferID != expected.TransferID {
		t.Fatal("restored retry did not return original outcome", err)
	}
	receipt, _ := json.Marshal(result.Result)
	if fmt.Sprintf("%x", sha256.Sum256(receipt)) != expected.ReceiptDigest {
		t.Fatal("restored immutable receipt differs")
	}
	saved, err := repo.GetRecovery(ctx, testTenantID, testActorID, "50000000-0000-4000-8000-000000009102")
	if err != nil || saved.State != "unresolved" {
		t.Fatal("pending recovery intent changed", err)
	}
	replay, _ := db.NewOutboxReplayRepository(api.db, time.Now)
	if err := replay.Replay(ctx, recovery.Replay{TenantID: testTenantID, ActorSubjectID: modelApprover, EventID: expected.DeadEventID, CorrelationID: expected.ApprovalCorrelation}); err != nil {
		t.Fatal(err)
	}
	ledger := oracle.New(expected.Accounts)
	ledger.Journals = expected.Journals
	suite := &financialModelSuite{admin: admin, api: api.db, transfers: service, ledger: ledger, bindings: expected.Bindings, immutable: expected.Immutable}
	if err := suite.step(ctx, 9011, 0, oracle.Operation{Kind: "transfer", Minor: 1}); err != nil {
		t.Fatal(err)
	}
	if err := suite.check(ctx); err != nil {
		t.Fatal(err)
	}
	recon := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_reconciliation")
	reconciler, _ := db.NewReconciliationRepository(recon.db)
	check, err := reconciler.Reconcile(ctx, testTenantID, time.Now())
	if err != nil || check.MismatchCount != 0 {
		t.Fatal("restored reconciliation failed", err)
	}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")})
	defer client.Close()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	balances, _ := db.NewBalanceRepository(recon.db)
	current, err := balances.ListCurrentForTenant(ctx, testTenantID)
	if err != nil {
		t.Fatal(err)
	}
	projection, _ := cache.NewBalanceCache(client, "", time.Minute)
	adapter, _ := cache.NewAccountAdapter(projection)
	for _, balance := range current {
		if _, err := adapter.Put(ctx, balance); err != nil {
			t.Fatal(err)
		}
	}
	worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker")
	worker.db.SetMaxOpenConns(2)
	runner, err := bootstrap.NewWorkerRunnerWithOptions(ctx, config.Config{Environment: "development", NodeID: "candidate-recovery", OutboxBatchSize: 10}, worker.db, client, nil, bootstrap.WorkerRunnerOptions{NodeRole: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if _, err := runner.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	if countRows(t, admin, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL OR dead_at IS NOT NULL`) != 0 {
		t.Fatal("restored delivery work did not drain")
	}
	if err := suite.check(ctx); err != nil {
		t.Fatal(err)
	}
	for alias, accountID := range expected.Bindings {
		if suite.ledger.Accounts[alias].System {
			continue
		}
		cached, err := projection.Get(ctx, testTenantID, accountID)
		if err != nil || fmt.Sprint(cached.LedgerMinor) != suite.ledger.Accounts[alias].Minor {
			t.Fatal("restored cache did not retain authoritative balances", err)
		}
	}
	t.Logf("RECOVERY_RESTORED tables=%d journals=%d row_hashes=equal restricted_commands=passed replay=original reconciliation=matched cache=rebuilt pending_work=drained", len(expected.Tables), suite.ledger.Journals)
}
