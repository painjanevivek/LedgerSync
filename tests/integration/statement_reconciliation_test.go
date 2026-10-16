package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	statement "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/statementreconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

type statementReceipt struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}

func statementCall(database *sql.DB, tenant, actor, query string, args ...any) (statementReceipt, error) {
	return statementCallContext(context.Background(), database, tenant, actor, query, args...)
}

func statementCallContext(ctx context.Context, database *sql.DB, tenant, actor, query string, args ...any) (statementReceipt, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return statementReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.actor_id',$2,true)`, tenant, actor); err != nil {
		return statementReceipt{}, err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		return statementReceipt{}, err
	}
	var result statementReceipt
	if err = json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, tx.Commit()
}
func statementMustCall(t *testing.T, database *sql.DB, query string, args ...any) statementReceipt {
	t.Helper()
	r, err := statementCall(database, testTenantID, testActorID, query, args...)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == "" {
		t.Fatal("receipt has no identity")
	}
	return r
}

const statementMapSQL = `SELECT controlled_statement_mapping_v1($1,$2,$3,$4,$5,$6)`
const statementImportSQL = `SELECT controlled_statement_import_v1($1,$2,$3,$4,$5,$6)`
const statementPrepareSQL = `SELECT controlled_statement_prepare_v1($1,$2,$3,$4,$5)`
const statementCompleteSQL = `SELECT controlled_statement_complete_v1($1,$2,$3,$4)`
const statementReviewSQL = `SELECT controlled_statement_review_v1($1,$2,$3,$4,$5,$6)`

func statementFinancialHash(t *testing.T, database *sql.DB) string {
	t.Helper()
	var result string
	err := database.QueryRow(`SELECT md5(jsonb_build_object('balances',(SELECT jsonb_agg(to_jsonb(b) ORDER BY account_id) FROM account_balance_projections b),'postings',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM ledger_postings p),'journals',(SELECT jsonb_agg(to_jsonb(j) ORDER BY id) FROM journal_transactions j),'transfers',(SELECT jsonb_agg(to_jsonb(tr) ORDER BY id) FROM transfers tr))::text)`).Scan(&result)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func statementFixture(t *testing.T) (*sql.DB, *sql.DB, string, string, string) {
	t.Helper()
	_, admin := requireTransferService(t, 1000)
	requireWorkloadRoles(t, admin)
	if err := seedTransferFixtureCurrency(context.Background(), admin, 1000, "INR"); err != nil {
		t.Fatal(err)
	}
	source, destination := uuid.NewString(), uuid.NewString()
	_, err := admin.Exec(`INSERT INTO accounts(id,tenant_id,currency,status) VALUES($1,$3,'INR','active'),($2,$3,'INR','active');`, source, destination, testTenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(`INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version) VALUES($1,1000,1000,0),($2,0,0,0);`, source, destination)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(`INSERT INTO account_opening_balances(account_id,opening_ledger_minor) VALUES($1,1000),($2,0)`, source, destination)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(`INSERT INTO account_owners(tenant_id,account_id,subject_id,permission) VALUES($1,$2,$3,'debit'),($1,$4,$3,'read')`, testTenantID, source, testActorID, destination)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(`INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id) VALUES($1,$2,$3)`, testTenantID, destination, testActorID)
	if err != nil {
		t.Fatal(err)
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api").db
	repository, err := db.NewTransferRepository(api, nil)
	if err != nil {
		t.Fatal(err)
	}
	repository.WithPilotCurrency("INR")
	service, err := transfers.NewService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	amount, err := money.Parse("INR", "1.00")
	if err != nil {
		t.Fatal(err)
	}
	command := transferCommand(t, uuid.NewString(), "1.00")
	command.DebitAccountID = identifier.UUID(source)
	command.CreditAccountID = identifier.UUID(destination)
	command.Amount = amount
	command.OccurredAt = time.Now().UTC().Add(-time.Minute)
	submitted, err := service.Submit(context.Background(), command)
	if err != nil || submitted.Result.Status != "posted" {
		t.Fatalf("fixture posting: %+v %v", submitted, err)
	}
	var posting string
	if err = admin.QueryRow(`SELECT p.id FROM ledger_postings p JOIN journal_transactions j ON j.id=p.journal_transaction_id WHERE p.account_id=$1 AND j.transfer_id=$2`, source, submitted.Result.TransferID).Scan(&posting); err != nil {
		t.Fatal(err)
	}
	command.IdempotencyKey = uuid.NewString()
	if second, err := service.Submit(context.Background(), command); err != nil || second.Result.Status != "posted" {
		t.Fatalf("second fixture posting: %+v %v", second, err)
	}
	return admin, api, source, submitted.Result.TransferID, posting
}

func statementSnapshot(t *testing.T, database *sql.DB, run string) []statement.LedgerItem {
	t.Helper()
	rows, err := database.Query(`SELECT posting_id,external_reference,direction,amount_minor,currency FROM statement_snapshot_items WHERE run_id=$1 ORDER BY posting_id`, run)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	result := []statement.LedgerItem{}
	for rows.Next() {
		var item statement.LedgerItem
		if err := rows.Scan(&item.ID, &item.Reference, &item.Direction, &item.AmountMinor, &item.Currency); err != nil {
			t.Fatal(err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestStatementReconciliation(t *testing.T) {
	admin, api, account, reference, posting := statementFixture(t)
	before := statementFinancialHash(t, admin)
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	source := []byte(statement.Header + "\n" + fmt.Sprintf("row-1,synthetic-clearing,%s,%s,%s,%s,debit,100,INR,1000,900\n", start.Format(time.RFC3339), end.Format(time.RFC3339), now.Format(time.RFC3339), reference))
	key, correlation := uuid.NewString(), uuid.NewString()
	mapping := statementMustCall(t, api, statementMapSQL, testTenantID, testActorID, key, correlation, account, "synthetic-clearing")
	importKey := uuid.NewString()
	accepted := statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, importKey, correlation, mapping.ID, source)
	t.Run("original-key and byte replay", func(t *testing.T) {
		replay := statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, importKey, correlation, mapping.ID, source)
		if !replay.Replayed || replay.ID != accepted.ID {
			t.Fatal("original import not recovered")
		}
		repeated := statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, source)
		if !repeated.Replayed || repeated.ID != accepted.ID {
			t.Fatal("duplicate file accepted twice")
		}
		altered := []byte(strings.Replace(string(source), "1000,900", "1001,901", 1))
		if _, err := statementCall(api, testTenantID, testActorID, statementImportSQL, testTenantID, testActorID, importKey, correlation, mapping.ID, altered); err == nil {
			t.Fatal("changed original intent accepted")
		}
	})
	t.Run("invalid file is atomic and original immutable", func(t *testing.T) {
		count := countRows(t, admin, `SELECT count(*) FROM statement_imports`)
		invalid := []byte(strings.Replace(string(source), ",100,INR", ",0100,INR", 1))
		if _, err := statementCall(api, testTenantID, testActorID, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, invalid); err == nil {
			t.Fatal("raw database boundary accepted malformed CSV")
		}
		repeatedRow := []byte(strings.Replace(string(source), reference, "different-reference", 1))
		if _, err := statementCall(api, testTenantID, testActorID, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, repeatedRow); err == nil {
			t.Fatal("source row reused across files")
		}
		if countRows(t, admin, `SELECT count(*) FROM statement_imports`) != count {
			t.Fatal("failed import left partial evidence")
		}
		var original []byte
		if err := admin.QueryRow(`SELECT source_bytes FROM statement_imports WHERE id=$1`, accepted.ID).Scan(&original); err != nil || string(original) != string(source) {
			t.Fatal("source changed")
		}
		if _, err := admin.Exec(`UPDATE statement_imports SET source_bytes='altered' WHERE id=$1`, accepted.ID); err == nil {
			t.Fatal("source evidence mutable")
		}
	})
	prepared := statementMustCall(t, api, statementPrepareSQL, testTenantID, testActorID, uuid.NewString(), correlation, accepted.ID)
	parsed, err := statement.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	matches := statement.MatchExact(parsed.Rows, []statement.LedgerItem{{ID: posting, Reference: reference, Direction: "debit", AmountMinor: 100, Currency: "INR"}})
	payload, _ := json.Marshal(matches)
	complete := statementMustCall(t, api, statementCompleteSQL, testTenantID, testActorID, prepared.ID, payload)
	if complete.ID != prepared.ID || countRows(t, admin, `SELECT count(*) FROM statement_matches WHERE run_id=$1`, prepared.ID) != 1 {
		t.Fatal("exact one-to-one result missing")
	}
	replay := statementMustCall(t, api, statementCompleteSQL, testTenantID, testActorID, prepared.ID, payload)
	if !replay.Replayed {
		t.Fatal("completion is not idempotent")
	}
	t.Run("review version and concurrent reviewers", func(t *testing.T) {
		review := map[string]any{"kind": "statement", "item_id": "row-1", "expected_version": "0", "status": "matched", "assignee": testActorID, "reason": "confirmed", "comment": "=untrusted spreadsheet text", "evidence": "/transfers/" + reference, "posting_id": posting}
		raw, _ := json.Marshal(review)
		errors := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := statementCall(api, testTenantID, testActorID, statementReviewSQL, testTenantID, testActorID, uuid.NewString(), correlation, prepared.ID, raw)
				errors <- err
			}()
		}
		wg.Wait()
		close(errors)
		success := 0
		for err := range errors {
			if err == nil {
				success++
			}
		}
		if success != 1 {
			t.Fatalf("concurrent successes=%d", success)
		}
	})
	t.Run("cancelled import rolls back and original identity retries", func(t *testing.T) {
		locked, err := admin.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = locked.Exec(`SELECT id FROM statement_mappings WHERE id=$1 FOR UPDATE`, mapping.ID); err != nil {
			_ = locked.Rollback()
			t.Fatal(err)
		}
		cancelledSource := []byte(strings.Replace(string(source), "row-1,", "row-cancel,", 1))
		count := countRows(t, admin, `SELECT count(*) FROM statement_imports`)
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		cancelKey := uuid.NewString()
		_, callErr := statementCallContext(ctx, api, testTenantID, testActorID, statementImportSQL, testTenantID, testActorID, cancelKey, correlation, mapping.ID, cancelledSource)
		cancel()
		_ = locked.Rollback()
		if callErr == nil || countRows(t, admin, `SELECT count(*) FROM statement_imports`) != count {
			t.Fatal("cancelled import left accepted evidence")
		}
		statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, cancelKey, correlation, mapping.ID, cancelledSource)
		if countRows(t, admin, `SELECT count(*) FROM statement_imports`) != count+1 {
			t.Fatal("original retry did not accept exactly once")
		}
	})
	t.Run("unequal money cannot be manually matched", func(t *testing.T) {
		var secondReference string
		if err := admin.QueryRow(`SELECT external_reference FROM statement_snapshot_items WHERE run_id=$1 AND posting_id<>$2`, prepared.ID, posting).Scan(&secondReference); err != nil {
			t.Fatal(err)
		}
		file := []byte(strings.Replace(strings.Replace(strings.Replace(string(source), "row-1,", "row-difference,", 1), reference, secondReference, 1), ",100,INR,1000,900", ",99,INR,900,801", 1))
		acceptedDifference := statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, file)
		differenceRun := statementMustCall(t, api, statementPrepareSQL, testTenantID, testActorID, uuid.NewString(), correlation, acceptedDifference.ID)
		parsedDifference, err := statement.Parse(file)
		if err != nil {
			t.Fatal(err)
		}
		items := statementSnapshot(t, admin, differenceRun.ID)
		if len(items) != 1 {
			t.Fatal("expected remaining eligible posting")
		}
		result, _ := json.Marshal(statement.MatchExact(parsedDifference.Rows, items))
		statementMustCall(t, api, statementCompleteSQL, testTenantID, testActorID, differenceRun.ID, result)
		decision, _ := json.Marshal(map[string]any{"kind": "statement", "item_id": "row-difference", "expected_version": "0", "status": "matched", "assignee": testActorID, "reason": "reference_override", "comment": "", "evidence": "", "posting_id": items[0].ID})
		if _, err := statementCall(api, testTenantID, testActorID, statementReviewSQL, testTenantID, testActorID, uuid.NewString(), correlation, differenceRun.ID, decision); err == nil {
			t.Fatal("unequal manual match accepted")
		}
		manualSource := []byte(strings.Replace(strings.Replace(string(source), "row-1,", "row-manual,", 1), reference, "provider-alias", 1))
		manualImport := statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, manualSource)
		manualRun := statementMustCall(t, api, statementPrepareSQL, testTenantID, testActorID, uuid.NewString(), correlation, manualImport.ID)
		manualParsed, err := statement.Parse(manualSource)
		if err != nil {
			t.Fatal(err)
		}
		manualResult, _ := json.Marshal(statement.MatchExact(manualParsed.Rows, statementSnapshot(t, admin, manualRun.ID)))
		statementMustCall(t, api, statementCompleteSQL, testTenantID, testActorID, manualRun.ID, manualResult)
		manualDecision, _ := json.Marshal(map[string]any{"kind": "statement", "item_id": "row-manual", "expected_version": "0", "status": "matched", "assignee": testActorID, "reason": "reference_override", "comment": "Reviewed reference only", "evidence": "/transfers/" + secondReference, "posting_id": items[0].ID})
		manualKey := uuid.NewString()
		statementMustCall(t, api, statementReviewSQL, testTenantID, testActorID, manualKey, correlation, manualRun.ID, manualDecision)
		if !statementMustCall(t, api, statementReviewSQL, testTenantID, testActorID, manualKey, correlation, manualRun.ID, manualDecision).Replayed {
			t.Fatal("manual decision not replayed")
		}
		if countRows(t, admin, `SELECT count(*) FROM statement_matches WHERE posting_id=$1`, items[0].ID) != 1 {
			t.Fatal("posting consumed more than once")
		}
	})
	t.Run("tenant actor and direct write denials", func(t *testing.T) {
		if _, err := statementCall(api, testTenantID, "wrong-actor", statementMapSQL, testTenantID, testActorID, uuid.NewString(), correlation, account, "synthetic-clearing"); err == nil {
			t.Fatal("actor forgery accepted")
		}
		if _, err := statementCall(api, uuid.NewString(), testActorID, statementPrepareSQL, testTenantID, testActorID, uuid.NewString(), correlation, accepted.ID); err == nil {
			t.Fatal("cross tenant command accepted")
		}
		if _, err := api.Exec(`DELETE FROM statement_imports`); err == nil {
			t.Fatal("raw API DML allowed")
		}
		var count int
		if err := api.QueryRow(`SELECT count(*) FROM statement_imports`).Scan(&count); err != nil || count != 0 {
			t.Fatal("unscoped source disclosed")
		}
		for _, table := range []string{"accounts", "account_balance_projections", "ledger_postings", "journal_transactions", "transfers"} {
			var canWrite bool
			if err := admin.QueryRow(`SELECT has_table_privilege('ledgersync_statement_owner',$1,'INSERT,UPDATE,DELETE')`, table).Scan(&canWrite); err != nil || canWrite {
				t.Fatalf("statement owner has financial authority: %s %v", table, err)
			}
		}
	})
	t.Run("real HTTP authority and bounded export", func(t *testing.T) {
		repository, err := db.NewStatementReconciliationRepository(api)
		if err != nil {
			t.Fatal(err)
		}
		service, err := statement.NewService(repository)
		if err != nil {
			t.Fatal(err)
		}
		audit, err := db.NewAuditRepository(api)
		if err != nil {
			t.Fatal(err)
		}
		provider := identity.DevelopmentProvider{TenantID: testTenantID, SubjectID: testActorID, Scopes: []string{"reconciliation:read", "reconciliation:write", "exports:read"}}
		h := handlers.NewStatementReconciliationHandler(service, provider).WithAuditRecorder(audit)
		router := http.NewServeMux()
		router.HandleFunc("GET /api/statement-reconciliation", h.Workspace)
		router.HandleFunc("POST /api/statement-reconciliation/preview", h.Preview)
		router.HandleFunc("POST /api/statement-reconciliation/runs", h.Run)
		router.HandleFunc("GET /api/statement-reconciliation/runs/{runID}", h.RunDetail)
		router.HandleFunc("GET /api/statement-reconciliation/runs/{runID}/comparison.csv", h.Export)
		router.HandleFunc("GET /api/statement-reconciliation/imports/{importID}/source.csv", h.Source)
		server := middleware.Correlation(router)
		call := func(method, path string, body any) *httptest.ResponseRecorder {
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(method, path, bytes.NewReader(data))
			r.Header.Set("Authorization", "Bearer development-local-only")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", uuid.NewString())
			w := httptest.NewRecorder()
			server.ServeHTTP(w, r)
			return w
		}
		workspace := call("GET", "/api/statement-reconciliation", nil)
		if workspace.Code != 200 || strings.Contains(workspace.Body.String(), "source_base64") {
			t.Fatalf("workspace status=%d", workspace.Code)
		}
		preview := call("POST", "/api/statement-reconciliation/preview", map[string]any{"source_base64": source})
		if preview.Code != 200 {
			t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
		}
		forged := call("POST", "/api/statement-reconciliation/preview", map[string]any{"source_base64": source, "tenant_id": uuid.NewString()})
		if forged.Code != 400 {
			t.Fatal("body authority accepted")
		}
		continued := call("POST", "/api/statement-reconciliation/runs", map[string]any{"import_id": accepted.ID})
		if continued.Code != 200 {
			t.Fatalf("run status=%d body=%s", continued.Code, continued.Body.String())
		}
		detail := call("GET", "/api/statement-reconciliation/runs/"+prepared.ID, nil)
		if detail.Code != 200 {
			t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
		}
		invalidCursor := call("GET", "/api/statement-reconciliation/runs/"+prepared.ID+"?cursor=invalid", nil)
		if invalidCursor.Code != 400 {
			t.Fatal("invalid cursor accepted")
		}
		invalidQuery := call("GET", "/api/statement-reconciliation?tenant_id=forged", nil)
		if invalidQuery.Code != 400 {
			t.Fatal("unrecognized query accepted")
		}
		export := call("GET", "/api/statement-reconciliation/runs/"+prepared.ID+"/comparison.csv", nil)
		if export.Code != 200 || export.Header().Get("X-Audit-Reference") == "" {
			t.Fatalf("export status=%d", export.Code)
		}
		records, err := csv.NewReader(strings.NewReader(export.Body.String())).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, record := range records[1:] {
			if record[1] == "row-1" {
				found = true
				if record[4] != "100" || !strings.HasPrefix(record[10], "'=untrusted") {
					t.Fatal("numeric or formula export is unsafe")
				}
			}
		}
		if !found {
			t.Fatal("row missing from export")
		}
		download := call("GET", "/api/statement-reconciliation/imports/"+accepted.ID+"/source.csv", nil)
		if download.Code != 200 || !bytes.Equal(download.Body.Bytes(), source) || download.Header().Get("X-Statement-SHA256") == "" {
			t.Fatal("protected original source changed")
		}
		noExport := provider
		noExport.Scopes = []string{"reconciliation:read"}
		denied := handlers.NewStatementReconciliationHandler(service, noExport).WithAuditRecorder(audit)
		r := httptest.NewRequest("GET", "/api/source", nil)
		r.Header.Set("Authorization", "Bearer development-local-only")
		r.SetPathValue("importID", accepted.ID)
		w := httptest.NewRecorder()
		middleware.Correlation(http.HandlerFunc(denied.Source)).ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("source download did not require export scope")
		}
	})
	if after := statementFinancialHash(t, admin); before != after {
		t.Fatal("statement comparison changed financial state")
	}
}

// This test creates business records only in the guarded disposable database.
// Comparison calls between each financial checkpoint must leave money unchanged.
func TestStatementSnapshotIsolationAndConsumption(t *testing.T) {
	admin, api, account, reference, _ := statementFixture(t)
	correlation := uuid.NewString()
	mapping := statementMustCall(t, api, statementMapSQL, testTenantID, testActorID, uuid.NewString(), correlation, account, "snapshot-source")
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	row := func(id, closing string) string {
		return fmt.Sprintf("%s,snapshot-source,%s,%s,%s,%s,debit,100,INR,1000,%s\n", id, start.Format(time.RFC3339), end.Format(time.RFC3339), now.Format(time.RFC3339), reference, closing)
	}
	prepare := func(raw []byte) string {
		imp := statementMustCall(t, api, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, raw)
		return statementMustCall(t, api, statementPrepareSQL, testTenantID, testActorID, uuid.NewString(), correlation, imp.ID).ID
	}
	complete := func(run string, raw []byte) error {
		parsed, err := statement.Parse(raw)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(statement.MatchExact(parsed.Rows, statementSnapshot(t, admin, run)))
		if err != nil {
			return err
		}
		_, err = statementCall(api, testTenantID, testActorID, statementCompleteSQL, testTenantID, testActorID, run, payload)
		return err
	}
	ambiguous := []byte(statement.Header + "\n" + row("ambiguous-a", "800") + row("ambiguous-b", "800"))
	runAmbiguous := prepare(ambiguous)
	if err := complete(runAmbiguous, ambiguous); err != nil {
		t.Fatal(err)
	}
	if countRows(t, admin, `SELECT count(*) FROM statement_matches WHERE run_id=$1`, runAmbiguous) != 0 || countRows(t, admin, `SELECT count(*) FROM statement_review_items WHERE run_id=$1 AND classification='ambiguous'`, runAmbiguous) != 2 {
		t.Fatal("shared candidate was selected arbitrarily")
	}
	first := []byte(statement.Header + "\n" + row("consume-a", "900"))
	second := []byte(statement.Header + "\n" + row("consume-b", "900"))
	runFirst, runSecond := prepare(first), prepare(second)
	frozen := statementSnapshot(t, admin, runFirst)
	var destination string
	if err := admin.QueryRow(`SELECT credit_account_id FROM transfers WHERE id=$1`, reference).Scan(&destination); err != nil {
		t.Fatal(err)
	}
	repo, err := db.NewTransferRepository(api, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo.WithPilotCurrency("INR")
	service, err := transfers.NewService(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	amount, err := money.Parse("INR", "1.00")
	if err != nil {
		t.Fatal(err)
	}
	command := transferCommand(t, uuid.NewString(), "1.00")
	command.DebitAccountID = identifier.UUID(account)
	command.CreditAccountID = identifier.UUID(destination)
	command.Amount = amount
	command.OccurredAt = now
	if result, err := service.Submit(context.Background(), command); err != nil || result.Result.Status != "posted" {
		t.Fatalf("late backdated fixture: %+v %v", result, err)
	}
	if after := statementSnapshot(t, admin, runFirst); len(after) != len(frozen) {
		t.Fatal("later backdated posting entered frozen run")
	}
	financial := statementFinancialHash(t, admin)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for _, job := range []struct {
		id  string
		raw []byte
	}{{runFirst, first}, {runSecond, second}} {
		wait.Add(1)
		go func(id string, raw []byte) { defer wait.Done(); errors <- complete(id, raw) }(job.id, job.raw)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if countRows(t, admin, `SELECT count(*) FROM statement_matches WHERE mapping_id=$1`, mapping.ID) != 1 {
		t.Fatal("posting was consumed twice")
	}
	if countRows(t, admin, `SELECT count(*) FROM statement_review_items WHERE run_id IN ($1,$2) AND kind='statement' AND classification='already_consumed'`, runFirst, runSecond) != 1 {
		t.Fatal("concurrent consumption was not classified")
	}
	if financial != statementFinancialHash(t, admin) {
		t.Fatal("comparison changed money")
	}
	for _, helper := range []string{"statement_existing_command_v1(uuid,text,text,text,uuid,bytea)", "statement_store_command_v1(uuid,text,text,text,uuid,bytea,uuid)"} {
		var allowed bool
		if err := admin.QueryRow(`SELECT has_function_privilege('ledgersync_api',$1,'EXECUTE')`, helper).Scan(&allowed); err != nil || allowed {
			t.Fatalf("private helper callable: %s %v", helper, err)
		}
	}
	var unsafe int
	if err := admin.QueryRow(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname LIKE 'statement_%' AND c.relkind='r' AND (NOT c.relrowsecurity OR NOT c.relforcerowsecurity)`).Scan(&unsafe); err != nil || unsafe != 0 {
		t.Fatal("statement table lacks forced tenant scope", err)
	}
}

func TestStatementRowBoundsAndPaging(t *testing.T) {
	admin, api, account, _, _ := statementFixture(t)
	correlation := uuid.NewString()
	mapping := statementMustCall(t, api, statementMapSQL, testTenantID, testActorID, uuid.NewString(), correlation, account, "row-limit")
	now := time.Now().UTC().Truncate(time.Second)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	source := func(rows int) []byte {
		var result strings.Builder
		result.WriteString(statement.Header + "\n")
		for n := 0; n < rows; n++ {
			fmt.Fprintf(&result, "bulk-%04d,row-limit,%s,%s,%s,provider-reference,debit,1,INR,%d,0\n", n, start.Format(time.RFC3339), end.Format(time.RFC3339), now.Format(time.RFC3339), rows)
		}
		return []byte(result.String())
	}
	before := statementFinancialHash(t, admin)
	count := countRows(t, admin, `SELECT count(*) FROM statement_imports`)
	if _, err := statementCall(api, testTenantID, testActorID, statementImportSQL, testTenantID, testActorID, uuid.NewString(), correlation, mapping.ID, source(1001)); err == nil {
		t.Fatal("raw SQL accepted more than 1000 rows")
	}
	if countRows(t, admin, `SELECT count(*) FROM statement_imports`) != count {
		t.Fatal("row-limit failure left an import")
	}
	repository, err := db.NewStatementReconciliationRepository(api)
	if err != nil {
		t.Fatal(err)
	}
	service, err := statement.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	meta := statement.Meta{Scope: statement.Scope{TenantID: testTenantID, ActorSubjectID: testActorID}, IdempotencyKey: uuid.NewString(), CorrelationID: correlation}
	accepted, err := service.Import(context.Background(), statement.ImportCommand{Meta: meta, MappingID: mapping.ID, Source: source(1000)})
	if err != nil {
		t.Fatal("valid maximum-size row set rejected", err)
	}
	meta.IdempotencyKey = uuid.NewString()
	run, err := service.Run(context.Background(), statement.RunCommand{Meta: meta, ImportID: accepted.ID})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Page(context.Background(), meta.Scope, run.ID, "")
	if err != nil || len(first.Items) != 100 || first.NextCursor == "" {
		t.Fatal("first bounded page differs", err)
	}
	next, err := service.Page(context.Background(), meta.Scope, run.ID, first.NextCursor)
	if err != nil || len(next.Items) != 100 {
		t.Fatal("next bounded page differs", err)
	}
	ids := map[string]bool{}
	for _, item := range first.Items {
		ids[item.Kind+":"+item.ItemID] = true
	}
	for _, item := range next.Items {
		if ids[item.Kind+":"+item.ItemID] {
			t.Fatal("cursor repeated an item")
		}
	}
	if before != statementFinancialHash(t, admin) {
		t.Fatal("bounded comparison changed money")
	}
}
