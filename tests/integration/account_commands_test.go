package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

var accountCommandTime = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func requireAccountCommandService(t *testing.T) (*accounts.CommandService, *sql.DB) {
	t.Helper()
	_, database := requireTransferService(t, 100)
	repository, err := db.NewAccountCommandRepository(database, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	service, err := accounts.NewCommandService(repository, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	return service, database
}

func createAccountCommand(key, reference string) accounts.CreateAccountCommand {
	return accounts.CreateAccountCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099",
		IdempotencyKey: key, DisplayName: "Operations भारत", Reference: reference, Category: "operating", Currency: "INR",
	}
}

func TestAccountCreateIsAtomicZeroAndRetrySafe(t *testing.T) {
	service, database := requireAccountCommandService(t)
	command := createAccountCommand("account-create-0001", "ops-india")
	created, err := service.Create(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if created.Replayed || created.Result.AvailableMinor != "0" || created.Result.LedgerMinor != "0" || created.Result.Currency != "INR" || created.Result.Version != "1" {
		t.Fatalf("unexpected create result: %#v", created)
	}
	replayed, err := service.Create(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || !reflect.DeepEqual(replayed.Result, created.Result) {
		t.Fatalf("replay=%#v, want original %#v", replayed, created)
	}

	changed := command
	changed.DisplayName = "Changed intent"
	if _, err := service.Create(context.Background(), changed); !errors.Is(err, accounts.ErrIdempotencyConflict) {
		t.Fatalf("changed-intent error=%v, want idempotency conflict", err)
	}
	duplicate := createAccountCommand("account-create-0002", "OPS-INDIA")
	if _, err := service.Create(context.Background(), duplicate); !errors.Is(err, accounts.ErrAccountConflict) {
		t.Fatalf("duplicate-reference error=%v, want account conflict", err)
	}

	checks := []struct {
		name  string
		query string
		want  int
	}{
		{name: "account", query: `SELECT count(*) FROM accounts WHERE id='` + created.Result.AccountID + `' AND tenant_id='` + testTenantID + `' AND currency='INR' AND status='active' AND version=1`, want: 1},
		{name: "zero projection", query: `SELECT count(*) FROM account_balance_projections WHERE account_id='` + created.Result.AccountID + `' AND available_minor=0 AND ledger_minor=0 AND balance_version=0`, want: 1},
		{name: "zero reconciliation baseline", query: `SELECT count(*) FROM account_opening_balances WHERE account_id='` + created.Result.AccountID + `' AND opening_ledger_minor=0`, want: 1},
		{name: "debit owner", query: `SELECT count(*) FROM account_owners WHERE account_id='` + created.Result.AccountID + `' AND subject_id='` + testActorID + `' AND permission='debit'`, want: 1},
		{name: "credit permission", query: `SELECT count(*) FROM account_credit_permissions WHERE account_id='` + created.Result.AccountID + `' AND subject_id='` + testActorID + `'`, want: 1},
		{name: "idempotency outcome", query: `SELECT count(*) FROM idempotency_requests WHERE operation='accounts.create.v1' AND idempotency_key='account-create-0001' AND state='completed' AND transfer_id IS NULL`, want: 1},
		{name: "duplicate failed outcome", query: `SELECT count(*) FROM idempotency_requests WHERE operation='accounts.create.v1' AND idempotency_key='account-create-0002' AND state='failed' AND response_body->>'error_code'='reference_conflict'`, want: 1},
		{name: "one normalized reference", query: `SELECT count(*) FROM accounts WHERE tenant_id='` + testTenantID + `' AND lower(external_reference)='ops-india'`, want: 1},
		{name: "audit", query: `SELECT count(*) FROM audit_events WHERE target_id='` + created.Result.AccountID + `' AND event_type='account.created' AND outcome='succeeded'`, want: 1},
		{name: "generic outbox", query: `SELECT count(*) FROM outbox_events WHERE aggregate_type='account' AND aggregate_id='` + created.Result.AccountID + `' AND transfer_id IS NULL AND event_type='account.created.v1'`, want: 1},
	}
	for _, check := range checks {
		if got := countRows(t, database, check.query); got != check.want {
			t.Fatalf("%s rows=%d, want %d", check.name, got, check.want)
		}
	}
}

func TestAccountCreateHTTPRetryAfterLostResponseUsesRealServiceAndDatabaseControls(t *testing.T) {
	service, database := requireAccountCommandService(t)
	rateLimiter, err := db.NewRateLimitRepository(database, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	commandHandler := handlers.NewAccountCommandHandler(service, identity.DevelopmentProvider{
		SubjectID: testActorID, TenantID: testTenantID, Scopes: []string{"accounts:write"},
	}).WithRateLimiter(rateLimiter, 100).WithCapacityLimit(rateLimiter, 100)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/accounts", commandHandler.Create)
	handler := middleware.Correlation(router)
	body := `{"display_name":"HTTP Operations","external_reference":"http-operations","category":"operating","currency":"INR"}`

	// Treat this committed response as lost at the caller boundary.
	original := executeAccountHTTPCreate(handler, body, "account-http-lost-001")
	if original.Code != http.StatusCreated || original.Header().Get("Idempotent-Replay") != "" {
		t.Fatalf("original status=%d headers=%v body=%s", original.Code, original.Header(), original.Body.String())
	}
	replayed := executeAccountHTTPCreate(handler, body, "account-http-lost-001")
	if replayed.Code != http.StatusCreated || replayed.Header().Get("Idempotent-Replay") != "true" || replayed.Body.String() != original.Body.String() {
		t.Fatalf("replay status=%d headers=%v body=%s original=%s", replayed.Code, replayed.Header(), replayed.Body.String(), original.Body.String())
	}
	if got := countRows(t, database, `SELECT count(*) FROM accounts WHERE tenant_id=$1 AND external_reference='http-operations'`, testTenantID); got != 1 {
		t.Fatalf("created account rows=%d, want one", got)
	}
	if got := countRows(t, database, `SELECT count(*) FROM audit_events WHERE event_type='account.created' AND sanitized_metadata->>'external_reference' IS NULL`); got != 1 {
		t.Fatalf("sanitized create audit rows=%d, want one", got)
	}
	if got := countRows(t, database, `SELECT count(*) FROM outbox_events WHERE event_type='account.created.v1' AND aggregate_type='account'`); got != 1 {
		t.Fatalf("account outbox rows=%d, want one", got)
	}
}

func TestAuthorizedAccountReadsExposeConfigurationAndBalanceVersionsSeparately(t *testing.T) {
	commandService, database := requireAccountCommandService(t)
	created, err := commandService.Create(context.Background(), createAccountCommand("account-version-read-01", "version-read"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = commandService.UpdateMetadata(context.Background(), accounts.UpdateAccountMetadataCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099",
		IdempotencyKey: "account-version-update-01", AccountID: created.Result.AccountID, ExpectedVersion: 1,
		DisplayName: "Versioned account", Reference: "version-read", Category: "operating",
	})
	if err != nil {
		t.Fatal(err)
	}
	readRepository, err := db.NewAccountRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	readService, err := accounts.NewService(readRepository)
	if err != nil {
		t.Fatal(err)
	}
	readHandler := handlers.NewAccountsHandler(readService, identity.DevelopmentProvider{SubjectID: testActorID, TenantID: testTenantID, Scopes: []string{"accounts:read"}})
	router := http.NewServeMux()
	router.Handle("GET /api/me/accounts", readHandler)
	router.Handle("GET /api/accounts/{accountID}", readHandler)
	handler := middleware.Correlation(router)

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/accounts/"+created.Result.AccountID, nil)
	detailRequest.Header.Set("Authorization", "Bearer development-local-only")
	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail map[string]any
	if err := json.NewDecoder(detailResponse.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail["account_version"] != "2" || detail["version"] != "0" {
		t.Fatalf("detail versions=%#v, want account_version=2 and balance version=0", detail)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/me/accounts", nil)
	listRequest.Header.Set("Authorization", "Bearer development-local-only")
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var list struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range list.Accounts {
		if item["account_id"] == created.Result.AccountID {
			found = true
			if item["account_version"] != "2" || item["version"] != "0" {
				t.Fatalf("list versions=%#v, want account_version=2 and balance version=0", item)
			}
		}
	}
	if !found {
		t.Fatalf("created account %s absent from authorized list", created.Result.AccountID)
	}
}

func executeAccountHTTPCreate(handler http.Handler, body, key string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer development-local-only")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestStableAccountDenialIsRecordedAndReplayedOnce(t *testing.T) {
	service, database := requireAccountCommandService(t)
	command := accounts.ChangeAccountStatusCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099",
		IdempotencyKey: "stable-denial-001", AccountID: testSourceID, ExpectedVersion: 1, TargetStatus: accountdomain.StatusClosed, Reason: "Quarter-end closure review",
	}
	original, err := service.ChangeStatus(context.Background(), command)
	if !errors.Is(err, accounts.ErrNonZeroClose) || original.Replayed {
		t.Fatalf("original submission=%#v error=%v, want original non-zero denial", original, err)
	}
	assertStableDenialEvidence(t, database, command.IdempotencyKey, testSourceID)

	replayed, err := service.ChangeStatus(context.Background(), command)
	if !errors.Is(err, accounts.ErrNonZeroClose) || !replayed.Replayed {
		t.Fatalf("replayed submission=%#v error=%v, want replayed non-zero denial", replayed, err)
	}
	assertStableDenialEvidence(t, database, command.IdempotencyKey, testSourceID)

	changed := command
	changed.TargetStatus = accountdomain.StatusFrozen
	if _, err := service.ChangeStatus(context.Background(), changed); !errors.Is(err, accounts.ErrIdempotencyConflict) {
		t.Fatalf("changed intent error=%v, want idempotency conflict", err)
	}
	changed = command
	changed.Reason = "Different closure intent"
	if _, err := service.ChangeStatus(context.Background(), changed); !errors.Is(err, accounts.ErrIdempotencyConflict) {
		t.Fatalf("changed reason error=%v, want idempotency conflict", err)
	}
	assertStableDenialEvidence(t, database, command.IdempotencyKey, testSourceID)
}

func TestUnknownAccountDependencyFailureRollsBackAndSameKeyCanRetry(t *testing.T) {
	service, database := requireAccountCommandService(t)
	if _, err := database.Exec(`
CREATE FUNCTION fail_account_create_for_test() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.external_reference='dependency-failure' THEN
    RAISE EXCEPTION 'injected account storage failure' USING ERRCODE='58000';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER fail_account_create_for_test BEFORE INSERT ON accounts
FOR EACH ROW EXECUTE FUNCTION fail_account_create_for_test()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DROP TRIGGER IF EXISTS fail_account_create_for_test ON accounts`)
		_, _ = database.Exec(`DROP FUNCTION IF EXISTS fail_account_create_for_test()`)
	})
	command := createAccountCommand("dependency-fail-01", "dependency-failure")
	if _, err := service.Create(context.Background(), command); !errors.Is(err, accounts.ErrCommandUnavailable) {
		t.Fatalf("injected dependency error=%v, want unavailable", err)
	}
	for _, check := range []struct {
		name  string
		query string
	}{
		{name: "idempotency", query: `SELECT count(*) FROM idempotency_requests WHERE operation='accounts.create.v1' AND idempotency_key='dependency-fail-01'`},
		{name: "account", query: `SELECT count(*) FROM accounts WHERE external_reference='dependency-failure'`},
		{name: "audit", query: `SELECT count(*) FROM audit_events WHERE correlation_id='00000000-0000-0000-0000-000000000099' AND sanitized_metadata->>'operation'='account_create'`},
	} {
		if got := countRows(t, database, check.query); got != 0 {
			t.Fatalf("rolled-back %s rows=%d, want zero", check.name, got)
		}
	}
	if _, err := database.Exec(`DROP TRIGGER fail_account_create_for_test ON accounts; DROP FUNCTION fail_account_create_for_test()`); err != nil {
		t.Fatal(err)
	}
	retried, err := service.Create(context.Background(), command)
	if err != nil || retried.Replayed {
		t.Fatalf("retry after dependency recovery=%#v error=%v", retried, err)
	}
}

func assertStableDenialEvidence(t *testing.T, database *sql.DB, key, accountID string) {
	t.Helper()
	if got := countRows(t, database, `SELECT count(*) FROM idempotency_requests WHERE operation='accounts.update.v1' AND idempotency_key=$1 AND state='failed' AND response_status=422 AND response_body->>'error_code'='non_zero_balance'`, key); got != 1 {
		t.Fatalf("failed idempotency outcomes=%d, want one", got)
	}
	if got := countRows(t, database, `SELECT count(*) FROM audit_events WHERE target_id=$1 AND event_type='account.command_denied' AND outcome='denied' AND sanitized_metadata->>'denial_code'='non_zero_balance' AND sanitized_metadata->>'reason'='Quarter-end closure review'`, accountID); got != 1 {
		t.Fatalf("non-zero denial audit rows=%d, want exactly one", got)
	}
}

func TestAccountLifecycleCloseRulesVersionAndTenantBoundary(t *testing.T) {
	service, database := requireAccountCommandService(t)
	created, err := service.Create(context.Background(), createAccountCommand("account-create-0010", "close-zero"))
	if err != nil {
		t.Fatal(err)
	}
	closeCommand := accounts.ChangeAccountStatusCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", IdempotencyKey: "account-close-00001", AccountID: created.Result.AccountID, ExpectedVersion: 1, TargetStatus: accountdomain.StatusClosed, Reason: "Account no longer required"}
	closed, err := service.ChangeStatus(context.Background(), closeCommand)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Result.Status != "closed" || closed.Result.Version != "2" {
		t.Fatalf("closed result=%#v", closed.Result)
	}
	if got := countRows(t, database, `SELECT count(*) FROM audit_events WHERE target_id=$1 AND event_type='account.status_changed' AND sanitized_metadata->>'reason'='Account no longer required'`, created.Result.AccountID); got != 1 {
		t.Fatalf("lifecycle reason audit rows=%d, want one", got)
	}
	readRepository, err := db.NewAccountRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := readRepository.GetOwned(context.Background(), testTenantID, testActorID, created.Result.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.AuditContext) < 1 || detail.AuditContext[0].EventType != "account.status_changed" || detail.AuditContext[0].Reason != "Account no longer required" {
		t.Fatalf("account detail lifecycle audit=%+v", detail.AuditContext)
	}
	if got := countRows(t, database, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='account.status.changed.v1' AND payload->>'reason'='Account no longer required'`, created.Result.AccountID); got != 1 {
		t.Fatalf("lifecycle reason outbox rows=%d, want one", got)
	}
	reactivate := closeCommand
	reactivate.IdempotencyKey = "account-active-0001"
	reactivate.ExpectedVersion = 2
	reactivate.TargetStatus = accountdomain.StatusActive
	if _, err := service.ChangeStatus(context.Background(), reactivate); !errors.Is(err, accounts.ErrTerminalStatus) {
		t.Fatalf("terminal transition error=%v", err)
	}

	nonZero := closeCommand
	nonZero.IdempotencyKey = "account-close-00002"
	nonZero.AccountID = testSourceID
	if _, err := service.ChangeStatus(context.Background(), nonZero); !errors.Is(err, accounts.ErrNonZeroClose) {
		t.Fatalf("non-zero close error=%v", err)
	}

	otherTenant := "00000000-0000-0000-0000-000000000777"
	if _, err := database.Exec(`INSERT INTO tenants(id,external_reference) VALUES($1,'other-tenant')`, otherTenant); err != nil {
		t.Fatal(err)
	}
	crossTenant := createAccountCommand("account-create-0011", "cross-tenant")
	crossTenant.TenantID = otherTenant
	if _, err := service.Create(context.Background(), crossTenant); !errors.Is(err, accounts.ErrAccountNotFound) {
		t.Fatalf("cross-tenant create error=%v, want non-disclosing not found", err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM accounts WHERE tenant_id=$1`, otherTenant); got != 0 {
		t.Fatalf("cross-tenant account count=%d, want zero", got)
	}
}

func TestAccountClosureBlocksObligationsAndPreservesSettledHistory(t *testing.T) {
	service, database := requireAccountCommandService(t)
	ctx := context.Background()
	seedINRAccountCommandFixture(t, database)
	if _, err := database.ExecContext(ctx, `INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, testTenantID, testSourceID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance') ON CONFLICT DO NOTHING`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'INR','local_demo_single_operator',false,1,100000,200000,500000)`, testTenantID); err != nil {
		t.Fatal(err)
	}
	pendingTarget, err := service.Create(ctx, createAccountCommand("account-create-obligation-01", "pending-obligation"))
	if err != nil {
		t.Fatal(err)
	}
	fundingRepository, err := db.NewFundingRepository(database, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	fundingService, err := appfunding.NewService(fundingRepository, appfunding.PolicyLocalDemoSingleOperator, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New("INR", 100)
	if _, err = fundingService.Request(ctx, appfunding.RequestCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: pendingTarget.Result.AccountID,
		Amount: amount, ExternalReference: "pending-close-evidence", EvidenceReference: "evidence://pending-close",
		IdempotencyKey: "funding-close-obligation-01", CorrelationID: "00000000-0000-0000-0000-000000000099",
	}); err != nil {
		t.Fatal(err)
	}
	closePending := accounts.ChangeAccountStatusCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", IdempotencyKey: "account-close-obligation-01", AccountID: pendingTarget.Result.AccountID, ExpectedVersion: 1, TargetStatus: accountdomain.StatusClosed, Reason: "Closure must wait for pending funding evidence"}
	if _, err = service.ChangeStatus(ctx, closePending); !errors.Is(err, accounts.ErrOperationalObligations) {
		t.Fatalf("pending obligation close error=%v", err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM audit_events WHERE target_id=$1 AND sanitized_metadata->>'denial_code'='operational_obligations'`, pendingTarget.Result.AccountID); got != 1 {
		t.Fatalf("operational obligation denial audits=%d, want one", got)
	}

	settled, err := service.Create(ctx, createAccountCommand("account-create-history-01", "settled-history"))
	if err != nil {
		t.Fatal(err)
	}
	transferRepository, err := db.NewTransferRepository(database, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	transferRepository.WithPilotCurrency("INR")
	transferService, err := transfers.NewService(transferRepository, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []transfers.Command{
		{TenantID: testTenantID, ActorSubjectID: testActorID, DebitAccountID: testSourceID, CreditAccountID: identifier.UUID(settled.Result.AccountID), Amount: amount, IdempotencyKey: "closed-history-transfer-01", CorrelationID: "00000000-0000-0000-0000-000000000099"},
		{TenantID: testTenantID, ActorSubjectID: testActorID, DebitAccountID: identifier.UUID(settled.Result.AccountID), CreditAccountID: testSourceID, Amount: amount, IdempotencyKey: "closed-history-transfer-02", CorrelationID: "00000000-0000-0000-0000-000000000099"},
	} {
		if _, err = transferService.Submit(ctx, command); err != nil {
			t.Fatal(err)
		}
	}
	closed, err := service.ChangeStatus(ctx, accounts.ChangeAccountStatusCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", IdempotencyKey: "account-close-history-01", AccountID: settled.Result.AccountID, ExpectedVersion: 1, TargetStatus: accountdomain.StatusClosed, Reason: "Settled account retention proof"})
	if err != nil || closed.Result.Status != "closed" {
		t.Fatalf("closed=%#v err=%v", closed, err)
	}
	historyRepository, err := db.NewTransactionHistoryRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	history, _, err := historyRepository.ListAccountHistory(ctx, testTenantID, testActorID, settled.Result.AccountID, "", 10)
	if err != nil || len(history) != 2 {
		t.Fatalf("closed account history=%#v err=%v", history, err)
	}
}

func TestAccountMetadataUsesOptimisticVersionAndStableReplay(t *testing.T) {
	service, database := requireAccountCommandService(t)
	created, err := service.Create(context.Background(), createAccountCommand("account-create-0015", "metadata-before"))
	if err != nil {
		t.Fatal(err)
	}
	command := accounts.UpdateAccountMetadataCommand{
		TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099",
		IdempotencyKey: "account-update-001", AccountID: created.Result.AccountID, ExpectedVersion: 1,
		DisplayName: "Payroll भारत", Reference: "metadata-after", Category: "payroll",
	}
	updated, err := service.UpdateMetadata(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Result.Version != "2" || updated.Result.DisplayName != "Payroll भारत" || updated.Result.Reference != "metadata-after" || updated.Result.Category != "payroll" {
		t.Fatalf("unexpected metadata result: %#v", updated.Result)
	}
	replayed, err := service.UpdateMetadata(context.Background(), command)
	if err != nil || !replayed.Replayed || !reflect.DeepEqual(replayed.Result, updated.Result) {
		t.Fatalf("metadata replay=%#v error=%v", replayed, err)
	}
	changed := command
	changed.DisplayName = "Changed intent"
	if _, err := service.UpdateMetadata(context.Background(), changed); !errors.Is(err, accounts.ErrIdempotencyConflict) {
		t.Fatalf("changed metadata retry error=%v, want idempotency conflict", err)
	}
	stale := command
	stale.IdempotencyKey = "account-update-002"
	stale.ExpectedVersion = 1
	if _, err := service.UpdateMetadata(context.Background(), stale); !errors.Is(err, accounts.ErrVersionConflict) {
		t.Fatalf("stale metadata error=%v, want version conflict", err)
	}
	if got := countRows(t, database, `SELECT count(*) FROM accounts WHERE id=$1 AND version=2 AND display_name='Payroll भारत' AND external_reference='metadata-after' AND category='payroll'`, created.Result.AccountID); got != 1 {
		t.Fatalf("committed metadata rows=%d, want one", got)
	}
}

func TestConcurrentCloseAndTransferSerializeOnAccountProjectionLocks(t *testing.T) {
	_, database := requireTransferService(t, 100)
	seedINRAccountCommandFixture(t, database)
	accountRepository, _ := db.NewAccountCommandRepository(database, func() time.Time { return accountCommandTime })
	accountService, _ := accounts.NewCommandService(accountRepository, func() time.Time { return accountCommandTime })
	created, err := accountService.Create(context.Background(), createAccountCommand("account-create-0020", "race-target"))
	if err != nil {
		t.Fatal(err)
	}
	transferRepository, err := db.NewTransferRepository(database, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	transferRepository.WithPilotCurrency("INR")
	transferService, err := transfers.NewService(transferRepository, func() time.Time { return accountCommandTime })
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New("INR", 1)
	transfer := transfers.Command{TenantID: testTenantID, ActorSubjectID: testActorID, DebitAccountID: testSourceID, CreditAccountID: identifier.UUID(created.Result.AccountID), Amount: amount, IdempotencyKey: "account-race-xfer1", CorrelationID: "00000000-0000-0000-0000-000000000099"}
	closeCommand := accounts.ChangeAccountStatusCommand{TenantID: testTenantID, ActorSubjectID: testActorID, CorrelationID: "00000000-0000-0000-0000-000000000099", IdempotencyKey: "account-race-close", AccountID: created.Result.AccountID, ExpectedVersion: 1, TargetStatus: accountdomain.StatusClosed, Reason: "Concurrent close safety test"}

	start := make(chan struct{})
	var wait sync.WaitGroup
	var closeErr, transferErr error
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, closeErr = accountService.ChangeStatus(context.Background(), closeCommand)
	}()
	go func() {
		defer wait.Done()
		<-start
		_, transferErr = transferService.Submit(context.Background(), transfer)
	}()
	close(start)
	wait.Wait()
	if (closeErr == nil) == (transferErr == nil) {
		t.Fatalf("close error=%v transfer error=%v; exactly one command must commit", closeErr, transferErr)
	}
	var status string
	var available, ledger int64
	if err := database.QueryRow(`SELECT a.status,b.available_minor,b.ledger_minor FROM accounts a JOIN account_balance_projections b ON b.account_id=a.id WHERE a.id=$1`, created.Result.AccountID).Scan(&status, &available, &ledger); err != nil {
		t.Fatal(err)
	}
	if closeErr == nil {
		if !errors.Is(transferErr, db.ErrAccountInactive) || status != "closed" || available != 0 || ledger != 0 {
			t.Fatalf("close-won state status=%s available=%d ledger=%d transferErr=%v", status, available, ledger, transferErr)
		}
	} else if status != "active" || available != 1 || ledger != 1 || !errors.Is(closeErr, accounts.ErrNonZeroClose) {
		t.Fatalf("transfer-won state status=%s available=%d ledger=%d closeErr=%v", status, available, ledger, closeErr)
	}
}

func seedINRAccountCommandFixture(t *testing.T, database *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if err := seedTransferFixture(ctx, database, 100); err != nil {
		t.Fatal(err)
	}
	// The fixture is empty of history, so recreating its configuration as INR
	// does not rewrite financial evidence.
	if _, err := database.ExecContext(ctx, `TRUNCATE TABLE audit_events,outbox_events,idempotency_requests,ledger_postings,journal_transactions,transfers,account_balance_projections,account_opening_balances,account_credit_permissions,account_owners,tenant_transfer_policies,accounts,tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenants(id,external_reference)VALUES($1,'integration-inr-tenant')`, testTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role)VALUES($1,$2,'operator')`, testTenantID, testActorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO accounts(id,tenant_id,currency,status,display_name,category,external_reference)VALUES($1,$3,'INR','active','INR source','operating','inr-source'),($2,$3,'INR','active','INR destination','operating','inr-destination')`, testSourceID, testDestinationID, testTenantID); err != nil {
		t.Fatal(err)
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO account_owners(tenant_id,account_id,subject_id,permission)VALUES($1,$2,$3,'debit')`, []any{testTenantID, testSourceID, testActorID}},
		{`INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id)VALUES($1,$2,$3)`, []any{testTenantID, testDestinationID, testActorID}},
		{`INSERT INTO tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor)VALUES($1,'INR',1,1000000,1000000,1000000,1000000)`, []any{testTenantID}},
		{`INSERT INTO account_balance_projections(account_id,available_minor,ledger_minor,balance_version)VALUES($1,100,100,0),($2,0,0,0)`, []any{testSourceID, testDestinationID}},
		{`INSERT INTO account_opening_balances(account_id,opening_ledger_minor)VALUES($1,100),($2,0)`, []any{testSourceID, testDestinationID}},
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
}
