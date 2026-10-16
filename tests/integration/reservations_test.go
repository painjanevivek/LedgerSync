package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
)

func reservationFixture(t *testing.T) (*sql.DB, *sql.DB, *hold.Service) {
	t.Helper()
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api").db
	repo, err := db.NewReservationRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	service, err := hold.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	return admin, api, service
}
func reservationMeta() hold.Meta {
	return hold.Meta{Scope: hold.Scope{TenantID: testTenantID, ActorSubjectID: testActorID}, IdempotencyKey: uuid.NewString(), CorrelationID: uuid.NewString()}
}
func reservationCreate(amount string, ttl int) hold.Create {
	return hold.Create{Meta: reservationMeta(), DebitAccountID: testSourceID, CreditAccountID: testDestinationID, AmountMinor: amount, Currency: "USD", TTLSeconds: ttl}
}
func reservationBalance(t *testing.T, admin *sql.DB) (ledger, available, version int64) {
	t.Helper()
	if err := admin.QueryRow(`SELECT ledger_minor,available_minor,balance_version FROM account_balance_projections WHERE account_id=$1`, testSourceID).Scan(&ledger, &available, &version); err != nil {
		t.Fatal(err)
	}
	return
}
func TestInternalReservations(t *testing.T) {
	t.Run("audited obligation exports fail closed", checkReservationExport)
	t.Run("capture once, immutable receipt and current state", func(t *testing.T) {
		admin, _, service := reservationFixture(t)
		ctx := context.Background()
		command := reservationCreate("700", 3600)
		created, err := service.Create(ctx, command)
		if err != nil {
			t.Fatal(err)
		}
		l, a, v := reservationBalance(t, admin)
		if l != 10000 || a != 9300 || v != 1 {
			t.Fatalf("reserve %d %d %d", l, a, v)
		}
		if countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 0 {
			t.Fatal("reserve posted a journal")
		}
		decision := hold.Decide{Meta: reservationMeta(), HoldID: created.ID, Decision: "capture"}
		captured, err := service.Decide(ctx, decision)
		if err != nil || captured.State != "captured" || captured.TransferID == "" {
			t.Fatalf("capture %+v %v", captured, err)
		}
		l, a, v = reservationBalance(t, admin)
		if l != 9300 || a != 9300 || v != 2 {
			t.Fatalf("capture versions %d %d %d", l, a, v)
		}
		replay, err := service.Decide(ctx, decision)
		if err != nil || !replay.Replayed || replay.TransferID != captured.TransferID {
			t.Fatal("capture replay changed", err)
		}
		original, err := service.Create(ctx, command)
		if err != nil || !original.Replayed || original.State != "active" {
			t.Fatal("creation receipt was relabelled", err)
		}
		current, err := service.Read(ctx, command.Scope, created.ID)
		if err != nil || current.State != "captured" || current.Version != "2" {
			t.Fatal("current state unavailable", err)
		}
		command.AmountMinor = "701"
		if _, err := service.Create(ctx, command); !errors.Is(err, hold.ErrConflict) {
			t.Fatalf("changed intent: %v", err)
		}
		if countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 1 || countRows(t, admin, `SELECT count(*) FROM ledger_postings`) != 2 {
			t.Fatal("capture duplicated posting")
		}
	})
	t.Run("concurrent terminal decisions consume once", func(t *testing.T) {
		admin, _, service := reservationFixture(t)
		created, err := service.Create(context.Background(), reservationCreate("200", 3600))
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan error, 2)
		var wait sync.WaitGroup
		for _, action := range []string{"capture", "release"} {
			wait.Add(1)
			go func(action string) {
				defer wait.Done()
				_, err := service.Decide(context.Background(), hold.Decide{Meta: reservationMeta(), HoldID: created.ID, Decision: action})
				results <- err
			}(action)
		}
		wait.Wait()
		close(results)
		success, conflict := 0, 0
		for err := range results {
			if err == nil {
				success++
			} else if errors.Is(err, hold.ErrConflict) {
				conflict++
			} else {
				t.Fatal(err)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatalf("terminal winners %d conflicts %d", success, conflict)
		}
		l, a, v := reservationBalance(t, admin)
		if l != a || v != 2 {
			t.Fatal("reservation was lost or released twice")
		}
	})
	t.Run("frozen capture denial is durable and release remains permitted", func(t *testing.T) {
		admin, _, service := reservationFixture(t)
		created, err := service.Create(context.Background(), reservationCreate("300", 3600))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(`UPDATE accounts SET status='frozen' WHERE id=$1`, testSourceID); err != nil {
			t.Fatal(err)
		}
		decision := hold.Decide{Meta: reservationMeta(), HoldID: created.ID, Decision: "capture"}
		denied, err := service.Decide(context.Background(), decision)
		if err != nil || denied.RejectionCode == "" || denied.State != "active" {
			t.Fatalf("capture denial %+v %v", denied, err)
		}
		if _, err := admin.Exec(`UPDATE accounts SET status='active' WHERE id=$1`, testSourceID); err != nil {
			t.Fatal(err)
		}
		replay, err := service.Decide(context.Background(), decision)
		if err != nil || !replay.Replayed || replay.RejectionCode != denied.RejectionCode {
			t.Fatal("denied key re-evaluated", err)
		}
		l, a, v := reservationBalance(t, admin)
		if l != 10000 || a != 9700 || v != 1 {
			t.Fatal("failed capture lost reservation")
		}
		if _, err := admin.Exec(`UPDATE accounts SET status='frozen' WHERE id=$1`, testSourceID); err != nil {
			t.Fatal(err)
		}
		released, err := service.Decide(context.Background(), hold.Decide{Meta: reservationMeta(), HoldID: created.ID, Decision: "release"})
		if err != nil || released.State != "released" {
			t.Fatal(err)
		}
		l, a, v = reservationBalance(t, admin)
		if l != a || v != 2 || countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 0 {
			t.Fatal("release posted or changed twice")
		}
	})
	t.Run("audit failure rolls back and original create retries", func(t *testing.T) {
		admin, _, service := reservationFixture(t)
		if _, err := admin.Exec(`CREATE FUNCTION r16_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type LIKE 'hold.%' THEN RAISE EXCEPTION 'injected audit failure';END IF;RETURN NEW;END $$;CREATE TRIGGER r16_fail_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION r16_fail_audit()`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = admin.Exec(`DROP TRIGGER IF EXISTS r16_fail_audit ON audit_events;DROP FUNCTION IF EXISTS r16_fail_audit()`)
		})
		command := reservationCreate("100", 3600)
		if _, err := service.Create(context.Background(), command); err == nil {
			t.Fatal("audit failure ignored")
		}
		l, a, v := reservationBalance(t, admin)
		if l != a || v != 0 || countRows(t, admin, `SELECT count(*) FROM account_holds`) != 0 {
			t.Fatal("failed audit left financial change")
		}
		if _, err := admin.Exec(`DROP TRIGGER r16_fail_audit ON audit_events;DROP FUNCTION r16_fail_audit()`); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Create(context.Background(), command); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("expiry is worker only, leased and frozen-safe", func(t *testing.T) {
		admin, api, service := reservationFixture(t)
		created, err := service.Create(context.Background(), reservationCreate("500", 1))
		if err != nil {
			t.Fatal(err)
		}
		worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker").db
		if _, err := api.Exec(`SELECT controlled_claim_hold_expiry_v1(1,'api-denied',30)`); err == nil {
			t.Fatal("API discovered global expiry work")
		}
		time.Sleep(1100 * time.Millisecond)
		var id, tenant string
		var generation int64
		if err := worker.QueryRow(`SELECT hold_id,tenant_id,generation FROM controlled_claim_hold_expiry_v1(1,'expiry-test',30)`).Scan(&id, &tenant, &generation); err != nil || id != created.ID {
			t.Fatalf("claim %s %v", id, err)
		}
		if _, err := admin.Exec(`UPDATE accounts SET status='frozen' WHERE id=$1`, testSourceID); err != nil {
			t.Fatal(err)
		}
		complete := func(gen int64) error {
			return db.WithTenantSerializableSequence(context.Background(), worker, db.TenantScope{TenantID: tenant, ActorID: "system:hold-expiry"}, "transfer-policy|"+tenant, 5, func(tx *sql.Tx) error {
				var changed bool
				return tx.QueryRow(`SELECT controlled_expire_hold_v1($1,$2,$3,$4)`, tenant, id, "expiry-test", gen).Scan(&changed)
			})
		}
		if err := complete(generation - 1); err == nil {
			t.Fatal("stale expiry generation accepted")
		}
		if err := complete(generation); err != nil {
			t.Fatal(err)
		}
		if err := complete(generation); err != nil {
			t.Fatal("completed expiry replay", err)
		}
		current, err := service.Read(context.Background(), hold.Scope{TenantID: testTenantID, ActorSubjectID: testActorID}, id)
		if err != nil || current.State != "expired" {
			t.Fatal(err)
		}
		l, a, v := reservationBalance(t, admin)
		if l != a || v != 2 || countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 0 {
			t.Fatal("expiry changed money")
		}
	})
	t.Run("capture and due expiry race without posting or double release", func(t *testing.T) {
		admin, _, service := reservationFixture(t)
		ctx := context.Background()
		created, err := service.Create(ctx, reservationCreate("400", 1))
		if err != nil {
			t.Fatal(err)
		}
		worker := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_worker").db
		// Keep the real shared deadline. Never alter the immutable hold to force expiry.
		time.Sleep(1100 * time.Millisecond)
		expiry, err := db.NewReservationExpiryWorker(worker, "capture-expiry-race", 1)
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			receipt, err := service.Decide(ctx, hold.Decide{Meta: reservationMeta(), HoldID: created.ID, Decision: "capture"})
			if err == nil && (receipt.State != "active" || receipt.RejectionCode != "expired_capture") {
				err = fmt.Errorf("due capture returned an invalid outcome: %+v", receipt)
			}
			if errors.Is(err, hold.ErrConflict) {
				err = nil // Expiry acquired the shared account lock first.
			}
			results <- err
		}()
		go func() {
			<-start
			count, err := expiry.RunOnce(ctx)
			if err == nil && count != 1 {
				err = fmt.Errorf("expiry transitions=%d, want 1", count)
			}
			results <- err
		}()
		close(start)
		for range 2 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		current, err := service.Read(ctx, hold.Scope{TenantID: testTenantID, ActorSubjectID: testActorID}, created.ID)
		if err != nil || current.State != "expired" || current.Version != "2" {
			t.Fatal("race lost its single terminal outcome", err)
		}
		l, a, v := reservationBalance(t, admin)
		if l != 10000 || a != 10000 || v != 2 || countRows(t, admin, `SELECT count(*) FROM journal_transactions`) != 0 {
			t.Fatal("race posted or released more than once")
		}
	})
	t.Run("tenant actor raw DML and private helpers denied", func(t *testing.T) {
		admin, api, service := reservationFixture(t)
		command := reservationCreate("100", 3600)
		created, err := service.Create(context.Background(), command)
		if err != nil {
			t.Fatal(err)
		}
		command.ActorSubjectID = "forged-actor"
		command.IdempotencyKey = uuid.NewString()
		if _, err := service.Create(context.Background(), command); !errors.Is(err, hold.ErrForbidden) {
			t.Fatalf("actor substitution: %v", err)
		}
		if _, err := api.Exec(`UPDATE account_holds SET state='released' WHERE id=$1`, created.ID); err == nil {
			t.Fatal("API raw hold DML allowed")
		}
		var allowed bool
		for _, sig := range []string{"hold_command_existing_v1(uuid,text,text,text,uuid,bytea)", "hold_command_store_v1(uuid,text,text,text,bytea,jsonb)", "hold_record_change_v1(uuid,text,uuid,uuid,text,boolean)"} {
			if err := admin.QueryRow(`SELECT has_function_privilege('ledgersync_api',$1,'EXECUTE')`, sig).Scan(&allowed); err != nil || allowed {
				t.Fatal("private helper exposed", sig, err)
			}
		}
		if _, err := admin.Exec(`UPDATE account_holds SET amount_minor=101 WHERE id=$1`, created.ID); err == nil {
			t.Fatal("hold intent mutable")
		}
	})
}

// This independent oracle holds its own posted/held totals and source versions.
// It does not read production SQL or derive an expectation from a receipt.
func TestReservationGeneratedModel(t *testing.T) {
	admin, _, service := reservationFixture(t)
	random := rand.New(rand.NewPCG(16, 20261016))
	posted, reserved, version := int64(10000), int64(0), int64(0)
	type modelHold struct {
		id     string
		amount int64
		active bool
	}
	var records []modelHold
	for step := 0; step < 128; step++ {
		action := random.IntN(3)
		if action == 0 || len(records) == 0 {
			amount := int64(1 + random.IntN(37))
			if posted-reserved < amount {
				continue
			}
			receipt, err := service.Create(context.Background(), reservationCreate(strconv.FormatInt(amount, 10), 3600))
			if err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
			records = append(records, modelHold{receipt.ID, amount, true})
			reserved += amount
			version++
		} else {
			index := random.IntN(len(records))
			record := &records[index]
			if !record.active {
				continue
			}
			decision := "release"
			if action == 2 {
				decision = "capture"
			}
			command := hold.Decide{Meta: reservationMeta(), HoldID: record.id, Decision: decision}
			receipt, err := service.Decide(context.Background(), command)
			if err != nil || receipt.RejectionCode != "" {
				t.Fatalf("step %d: %+v %v", step, receipt, err)
			}
			reserved -= record.amount
			if decision == "capture" {
				posted -= record.amount
			}
			record.active = false
			version++
			again, err := service.Decide(context.Background(), command)
			if err != nil || !again.Replayed {
				t.Fatal("model replay changed state", err)
			}
		}
		ledger, available, actualVersion := reservationBalance(t, admin)
		if ledger != posted || available != posted-reserved || actualVersion != version {
			t.Fatalf("step %d actual %d/%d/v%d expected %d/%d/v%d", step, ledger, available, actualVersion, posted, posted-reserved, version)
		}
		var active string
		if err := admin.QueryRow(`SELECT COALESCE(sum(amount_minor),0)::text FROM account_holds WHERE tenant_id=$1 AND state='active'`, testTenantID).Scan(&active); err != nil || active != strconv.FormatInt(reserved, 10) {
			t.Fatal("obligation mismatch", err)
		}
	}
	snapshot := map[string]any{"steps": 128, "seed": "16:20261016", "posted_minor": strconv.FormatInt(posted, 10), "reserved_minor": strconv.FormatInt(reserved, 10), "source_version": strconv.FormatInt(version, 10)}
	raw, _ := json.Marshal(snapshot)
	t.Logf("reservation model: %s", raw)
}

func TestReservationExactIntegerBoundary(t *testing.T) {
	admin, _, service := reservationFixture(t)
	if err := seedTransferFixture(context.Background(), admin, 9223372036854775807); err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), reservationCreate("9223372036854775807", 3600))
	if err != nil {
		t.Fatal(err)
	}
	ledger, available, version := reservationBalance(t, admin)
	if ledger != 9223372036854775807 || available != 0 || version != 1 {
		t.Fatal("exact reserve overflow")
	}
	released, err := service.Decide(context.Background(), hold.Decide{Meta: reservationMeta(), HoldID: created.ID, Decision: "release"})
	if err != nil || released.State != "released" {
		t.Fatal(err)
	}
	ledger, available, version = reservationBalance(t, admin)
	if ledger != available || version != 2 {
		t.Fatal("exact release overflow")
	}
	if _, err := service.Create(context.Background(), reservationCreate("9223372036854775808", 3600)); !errors.Is(err, hold.ErrInvalid) {
		t.Fatal("out-of-range reserve accepted", err)
	}
}

func TestReservationsPreserveFundingAndCorrectionBoundary(t *testing.T) {
	suite := newFinancialModelSuite(t)
	ctx := context.Background()
	if err := suite.reset(ctx); err != nil {
		t.Fatal(err)
	}
	original, err := suite.transfers.Submit(ctx, transferCommand(t, uuid.NewString(), "1.00"))
	if err != nil || original.Result.Status != "posted" {
		t.Fatal(err)
	}
	repository, err := db.NewReservationRepository(suite.api)
	if err != nil {
		t.Fatal(err)
	}
	holds, err := hold.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	command := reservationCreate("2050", 3600)
	command.DebitAccountID = testDestinationID
	command.CreditAccountID = testSourceID
	created, err := holds.Create(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	requested, err := suite.corrections.Request(ctx, appcorrections.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.Result.TransferID, ReasonCode: "operational_error", OperatorNote: "Synthetic reserved-funds boundary", IdempotencyKey: uuid.NewString(), CorrelationID: uuid.NewString(), StepUpAuthenticatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := suite.corrections.Approve(ctx, appcorrections.DecisionCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, CorrectionID: requested.Event.CorrectionID, Reason: "Independent synthetic evidence", CorrelationID: uuid.NewString(), StepUpAuthenticatedAt: now}); err != nil {
		t.Fatal(err)
	}
	post := appcorrections.PostCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, CorrectionID: requested.Event.CorrectionID, IdempotencyKey: uuid.NewString(), CorrelationID: uuid.NewString(), StepUpAuthenticatedAt: now}
	before := statementFinancialHash(t, suite.admin)
	if _, err := suite.corrections.Post(ctx, post); !errors.Is(err, appcorrections.ErrConflict) {
		t.Fatalf("correction spent reserved funds: %v", err)
	}
	if before != statementFinancialHash(t, suite.admin) {
		t.Fatal("denied correction changed money")
	}
	amount, err := money.New("USD", 100)
	if err != nil {
		t.Fatal(err)
	}
	funded, err := suite.funding.Request(ctx, appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: testDestinationID, Amount: amount, ExternalReference: "synthetic-reservation-funding", EvidenceReference: "synthetic://reservation/funding", IdempotencyKey: uuid.NewString(), CorrelationID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := suite.funding.Approve(ctx, appfunding.DecisionCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, FundingEventID: funded.Event.FundingEventID, Reason: "Independent synthetic funding", CorrelationID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if _, err := suite.funding.Post(ctx, appfunding.ActionCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, FundingEventID: funded.Event.FundingEventID, IdempotencyKey: uuid.NewString(), CorrelationID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	var ledger, available int64
	if err := suite.admin.QueryRow(`SELECT ledger_minor,available_minor FROM account_balance_projections WHERE account_id=$1`, testDestinationID).Scan(&ledger, &available); err != nil || ledger != 2200 || available != 150 {
		t.Fatal("funding released a hold or changed exact values", err)
	}
	if _, err := suite.corrections.Post(ctx, post); err != nil {
		t.Fatal("funded correction did not use available funds", err)
	}
	if err := suite.admin.QueryRow(`SELECT ledger_minor,available_minor FROM account_balance_projections WHERE account_id=$1`, testDestinationID).Scan(&ledger, &available); err != nil || ledger != 2100 || available != 50 {
		t.Fatal("correction consumed another obligation", err)
	}
	current, err := holds.Read(ctx, command.Scope, created.ID)
	if err != nil || current.State != "active" {
		t.Fatal("financial command changed hold state", err)
	}
	recon := provisionWorkloadSession(t, suite.admin, testDatabaseURL(t), "ledgersync_reconciliation").db
	reconciler, err := db.NewReconciliationRepository(recon)
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Reconcile(ctx, testTenantID, time.Now())
	if err != nil || result.MismatchCount != 0 {
		t.Fatal("active hold reconciliation differs", err)
	}
}
