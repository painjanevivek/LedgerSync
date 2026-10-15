package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"testing"
	"time"

	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
)

const modelApprover = "model-independent-finance"

type modelMismatch struct {
	Class, Detail string
	Step          int
}

func (e *modelMismatch) Error() string {
	return fmt.Sprintf("model %s mismatch at step %d: %s", e.Class, e.Step, e.Detail)
}
func mismatch(class, format string, args ...any) error {
	return &modelMismatch{Class: class, Detail: fmt.Sprintf(format, args...)}
}

type modelTransfer struct {
	Intent     oracle.Intent
	Command    transfers.Command
	TransferID string
	Index      int
	Corrected  bool
	Status     string
}
type financialModelSuite struct {
	admin, api  *sql.DB
	transfers   *transfers.Service
	funding     *appfunding.Service
	corrections *appcorrections.Service
	ledger      *oracle.Ledger
	bindings    map[string]string
	records     []modelTransfer
	immutable   map[string]string
}

func newFinancialModelSuite(t *testing.T) *financialModelSuite {
	t.Helper()
	_, admin := requireTransferService(t, 10000)
	requireWorkloadRoles(t, admin)
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	api.db.SetMaxOpenConns(4)
	transferRepo, _ := db.NewTransferRepository(api.db, time.Now)
	transferService, _ := transfers.NewService(transferRepo, time.Now)
	fundingRepo, _ := db.NewFundingRepository(api.db, time.Now)
	fundingService, _ := appfunding.NewService(fundingRepo, appfunding.PolicyProductionDualControl, time.Now)
	correctionRepo, _ := db.NewTransferCorrectionRepository(api.db, time.Now)
	correctionService, _ := appcorrections.NewService(correctionRepo, time.Now)
	suite := &financialModelSuite{admin: admin, api: api.db, transfers: transferService, funding: fundingService, corrections: correctionService}
	t.Cleanup(func() {
		_, _ = admin.Exec(`DROP TRIGGER IF EXISTS r08_model_failure ON outbox_events; DROP FUNCTION IF EXISTS r08_model_failure()`)
	})
	return suite
}

func (s *financialModelSuite) reset(ctx context.Context) error {
	if _, err := s.admin.ExecContext(ctx, `DROP TRIGGER IF EXISTS r08_model_failure ON outbox_events; DROP FUNCTION IF EXISTS r08_model_failure()`); err != nil {
		return err
	}
	if err := seedTransferFixture(ctx, s.admin, 10000); err != nil {
		return err
	}
	if _, err := s.admin.ExecContext(ctx, `INSERT INTO account_owners(tenant_id,account_id,subject_id,permission) VALUES($1,$2,$3,'debit')`, testTenantID, testDestinationID, testActorID); err != nil {
		return err
	}
	if _, err := s.admin.ExecContext(ctx, `INSERT INTO account_credit_permissions(tenant_id,account_id,subject_id) VALUES($1,$2,$3)`, testTenantID, testSourceID, testActorID); err != nil {
		return err
	}
	if _, err := s.admin.ExecContext(ctx, `INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,$2,'finance'),($1,$3,'finance')`, testTenantID, testActorID, modelApprover); err != nil {
		return err
	}
	if _, err := s.admin.ExecContext(ctx, `INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'USD','production_dual_control',true,1,1000000,1000000000,1000000000)`, testTenantID); err != nil {
		return err
	}
	s.ledger = oracle.New(map[string]oracle.Account{"a": {Minor: "10000"}, "b": {Minor: "2000"}})
	s.bindings = map[string]string{"a": testSourceID, "b": testDestinationID}
	s.records = nil
	s.immutable = map[string]string{}
	return nil
}

func (s *financialModelSuite) check(ctx context.Context) error {
	if err := s.ledger.Check(); err != nil {
		return err
	}
	rows, err := s.admin.QueryContext(ctx, `SELECT a.id::text,p.ledger_minor::text,p.available_minor::text,p.balance_version::text FROM accounts a JOIN account_balance_projections p ON p.account_id=a.id WHERE a.tenant_id=$1`, testTenantID)
	if err != nil {
		return err
	}
	actual := map[string][3]string{}
	for rows.Next() {
		var id, ledger, available, version string
		if err := rows.Scan(&id, &ledger, &available, &version); err != nil {
			_ = rows.Close()
			return err
		}
		actual[id] = [3]string{ledger, available, version}
	}
	err = rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if len(actual) != len(s.ledger.Accounts) {
		return mismatch("account-count", "database=%d oracle=%d", len(actual), len(s.ledger.Accounts))
	}
	for alias, expected := range s.ledger.Accounts {
		got, ok := actual[s.bindings[alias]]
		if !ok || got[0] != expected.Minor || got[1] != expected.Minor {
			return mismatch("balance", "account=%s actual=%v expected=%s", alias, got, expected.Minor)
		}
		if got[2] != strconv.FormatUint(expected.Version, 10) {
			return mismatch("version", "account=%s actual=%s expected=%d", alias, got[2], expected.Version)
		}
	}
	var journals, postings, unbalanced int
	if err := s.admin.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM journal_transactions),(SELECT count(*) FROM ledger_postings),(SELECT count(*) FROM (SELECT journal_transaction_id FROM ledger_postings GROUP BY journal_transaction_id HAVING count(*)<>2 OR count(DISTINCT currency)<>1 OR sum(CASE WHEN direction='debit' THEN -amount_minor::numeric ELSE amount_minor::numeric END)<>0) bad)`).Scan(&journals, &postings, &unbalanced); err != nil {
		return err
	}
	if journals != s.ledger.Journals || postings != 2*s.ledger.Journals || unbalanced != 0 {
		return mismatch("journal", "journals=%d postings=%d unbalanced=%d expected_journals=%d", journals, postings, unbalanced, s.ledger.Journals)
	}
	rows, err = s.admin.QueryContext(ctx, `SELECT 'journal:'||id::text,md5(row_to_json(j)::text) FROM journal_transactions j UNION ALL SELECT 'posting:'||id::text,md5(row_to_json(p)::text) FROM ledger_postings p UNION ALL SELECT 'transfer:'||id::text,md5(row_to_json(t)::text) FROM transfers t WHERE status='posted'`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	seen := map[string]bool{}
	for rows.Next() {
		var id, digest string
		if err := rows.Scan(&id, &digest); err != nil {
			return err
		}
		if before, ok := s.immutable[id]; ok && before != digest {
			return mismatch("immutable", "old financial record changed: %s", id)
		}
		s.immutable[id] = digest
		seen[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id := range s.immutable {
		if !seen[id] {
			return mismatch("immutable", "old financial record missing: %s", id)
		}
	}
	return nil
}

func (s *financialModelSuite) compareTransfer(got transfers.Submission, expected oracle.Outcome, intent oracle.Intent) error {
	if got.Result.Status != expected.Status || got.Replayed != expected.Replayed {
		return mismatch("outcome", "actual=%s replay=%t expected=%s replay=%t", got.Result.Status, got.Replayed, expected.Status, expected.Replayed)
	}
	if expected.Status == "posted" {
		for alias, account := range map[string]oracle.Account{intent.From: expected.From, intent.To: expected.To} {
			id := s.bindings[alias]
			version, present := got.Result.MinimumBalanceVersions[id]
			// The public read-your-writes receipt requires the debit account.
			// It does not grant the sender a recipient read capability.
			if (alias == intent.From && !present) || (present && version != int64(account.Version)) {
				return mismatch("replay", "stored version changed for %s", alias)
			}
			if balance, exists := got.Result.Balances[id]; exists && (strconv.FormatInt(balance.PostedMinor, 10) != account.Minor || balance.Version != int64(account.Version)) {
				return mismatch("replay", "stored balance changed for %s", alias)
			}
		}
	}
	return nil
}
