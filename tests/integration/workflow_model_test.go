package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	accountdomain "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/account"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
)

func TestGeneratedWorkflowModel(t *testing.T) {
	for _, seed := range []int64{17, 29} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			suite := newFinancialModelSuite(t)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if err := suite.reset(ctx); err != nil {
				t.Fatal(err)
			}
			// Local account creation supports INR. Keep the reference workflow
			// and its isolated policy fixture in that same supported currency.
			if _, err := suite.admin.ExecContext(ctx, `INSERT INTO tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'INR',1,1000000,10000000,10000000,10000000) ON CONFLICT(tenant_id,currency) DO NOTHING`, testTenantID); err != nil {
				t.Fatal(err)
			}
			accountRepo, err := db.NewAccountCommandRepository(suite.api, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			accountService, err := accounts.NewCommandService(accountRepo, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			model := oracle.NewWorkflow()
			model.Grant(testActorID, testTenantID, true)
			model.Grant(modelApprover, testTenantID, true)
			bindings, requests := map[string]string{}, map[string]string{}
			commands := oracle.GenerateWorkflow(seed, 64, testTenantID, testActorID, modelApprover)
			for index, c := range commands {
				expected := model.Apply(c) // Select the expected result before the real call.
				var commandErr error
				actual := oracle.WorkflowResult{}
				correlation := uuid.NewString()
				switch c.Kind {
				case "create":
					result, err := accountService.Create(ctx, accounts.CreateAccountCommand{TenantID: c.Tenant, ActorSubjectID: c.Actor, CorrelationID: correlation, IdempotencyKey: c.Key, DisplayName: "Synthetic workflow account", Reference: c.Key, Category: "operating", Currency: "INR"})
					commandErr = err
					if err == nil {
						bindings[c.Account] = result.Result.AccountID
						actual.Status, actual.Replayed = "created", result.Replayed
						actual.Version, _ = strconv.ParseUint(result.Result.Version, 10, 64)
					}
				case "freeze", "reactivate", "close":
					result, err := accountService.ChangeStatus(ctx, accounts.ChangeAccountStatusCommand{TenantID: c.Tenant, ActorSubjectID: c.Actor, CorrelationID: correlation, IdempotencyKey: c.Key, AccountID: bindings[c.Account], ExpectedVersion: int64(c.ExpectedVersion), TargetStatus: map[string]accountdomain.Status{"freeze": accountdomain.StatusFrozen, "reactivate": accountdomain.StatusActive, "close": accountdomain.StatusClosed}[c.Kind], Reason: "Synthetic stateful lifecycle check"})
					commandErr = err
					if err == nil {
						actual.Status, actual.Replayed = "changed", result.Replayed
						actual.Version, _ = strconv.ParseUint(result.Result.Version, 10, 64)
					}
				case "request-funding":
					minor, _ := strconv.ParseInt(c.Minor, 10, 64)
					amount, _ := money.New("INR", minor)
					result, err := suite.funding.Request(ctx, appfunding.RequestCommand{TenantID: c.Tenant, ActorSubjectID: c.Actor, DestinationAccountID: bindings[c.Account], Amount: amount, ExternalReference: c.Key, EvidenceReference: "synthetic://workflow/" + c.Key, IdempotencyKey: c.Key, CorrelationID: correlation})
					commandErr = err
					if err == nil {
						requests[c.Key] = result.Event.FundingEventID
						actual.Status = result.Event.Status
						if result.Event.Status == "requested" {
							actual.Status = "pending" // Independent model vocabulary.
						}
					}
				case "approve", "reject":
					command := appfunding.DecisionCommand{TenantID: c.Tenant, ActorSubjectID: c.Actor, FundingEventID: requests[c.Reference], Reason: "Synthetic independent model review", CorrelationID: correlation}
					if c.Kind == "approve" {
						result, err := suite.funding.Approve(ctx, command)
						commandErr, actual.Status = err, result.Status
					} else {
						result, err := suite.funding.Reject(ctx, command)
						commandErr, actual.Status = err, result.Status
					}
				case "post-funding":
					result, err := suite.funding.Post(ctx, appfunding.ActionCommand{TenantID: c.Tenant, ActorSubjectID: c.Actor, FundingEventID: requests[c.Reference], IdempotencyKey: c.Key, CorrelationID: correlation})
					commandErr = err
					if err == nil {
						bindings["clearing:"+c.Tenant] = result.Event.SystemAccountID
						actual.Status, actual.Replayed = result.Event.Status, result.Replayed
					}
				case "transfer":
					minor, _ := strconv.ParseInt(c.Minor, 10, 64)
					amount, _ := money.New("INR", minor)
					result, err := suite.transfers.Submit(ctx, transfers.Command{TenantID: identifier.UUID(c.Tenant), ActorSubjectID: c.Actor, DebitAccountID: identifier.UUID(bindings[c.Account]), CreditAccountID: identifier.UUID(bindings[c.To]), Amount: amount, IdempotencyKey: c.Key, CorrelationID: correlation})
					commandErr = err
					if err == nil {
						actual.Status, actual.Replayed = result.Result.Status, result.Replayed
					}
				}
				if commandErr != nil {
					actual.Status = workflowError(commandErr, expected.Status)
				}
				if actual != expected {
					encoded, _ := json.Marshal(commands[:index+1])
					t.Logf("WORKFLOW_MODEL_FAILURE seed=%d prefix=%s", seed, encoded)
					t.Fatalf("step=%d kind=%s got=%+v expected=%+v error=%v", index, c.Kind, actual, expected, commandErr)
				}
				for alias, expectedAccount := range model.Accounts {
					var state, minor string
					var version uint64
					if err := suite.admin.QueryRowContext(ctx, `SELECT a.status,a.version,p.ledger_minor::text FROM accounts a JOIN account_balance_projections p ON p.account_id=a.id WHERE a.id=$1`, bindings[alias]).Scan(&state, &version, &minor); err != nil || state != expectedAccount.State || version != expectedAccount.Version || minor != model.Ledger.Accounts[alias].Minor {
						t.Fatalf("state/financial mismatch after step=%d account=%s error=%v", index, alias, err)
					}
				}
				if countRows(t, suite.admin, `SELECT count(*) FROM journal_transactions`) != model.Ledger.Journals || countRows(t, suite.admin, `SELECT count(*) FROM ledger_postings`) != 2*model.Ledger.Journals {
					t.Fatal("workflow journal count differs from independent prediction")
				}
			}
			encoded, _ := json.Marshal(commands)
			t.Logf("WORKFLOW_MODEL_REPORT seed=%d steps=%d commands=%s", seed, len(commands), encoded)
		})
	}
}

func workflowError(err error, expected string) string {
	switch {
	case errors.Is(err, accounts.ErrAccountNotFound):
		// Both variants are intentionally the same non-disclosing API error.
		if expected == "forbidden" {
			return "forbidden"
		}
		return "not_found"
	case errors.Is(err, accounts.ErrVersionConflict):
		return "stale"
	case errors.Is(err, accounts.ErrTerminalStatus):
		return "terminal"
	case errors.Is(err, accounts.ErrNonZeroClose):
		return "non_zero"
	case errors.Is(err, accounts.ErrOperationalObligations):
		return "obligations"
	case errors.Is(err, accounts.ErrInvalidTransition):
		return "invalid"
	case errors.Is(err, appfunding.ErrForbidden):
		return "forbidden"
	case errors.Is(err, appfunding.ErrConflict), errors.Is(err, accounts.ErrIdempotencyConflict):
		return "conflict"
	case errors.Is(err, db.ErrAccountInactive):
		return "inactive"
	default:
		return "unclassified_failure"
	}
}
