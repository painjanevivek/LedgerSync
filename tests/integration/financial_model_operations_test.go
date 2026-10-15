package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
)

func (s *financialModelSuite) intent(seed int64, index int, op oracle.Operation) (oracle.Intent, transfers.Command) {
	from, to := "a", "b"
	if op.Reverse {
		from, to = to, from
	}
	key := fmt.Sprintf("model-%d-step-%03d-%s", seed, index, op.Kind)
	intent := oracle.Intent{Tenant: testTenantID, Actor: testActorID, Operation: "transfer", Key: key, From: from, To: to, Currency: "USD", Minor: strconv.FormatInt(op.Minor, 10)}
	amount, _ := money.New("USD", op.Minor)
	command := transfers.Command{TenantID: identifier.UUID(testTenantID), ActorSubjectID: testActorID, DebitAccountID: identifier.UUID(s.bindings[from]), CreditAccountID: identifier.UUID(s.bindings[to]), Amount: amount, IdempotencyKey: key, CorrelationID: uuid.NewString()}
	return intent, command
}
func (s *financialModelSuite) record(index int, intent oracle.Intent, command transfers.Command, result transfers.Submission) {
	s.records = append(s.records, modelTransfer{Intent: intent, Command: command, TransferID: result.Result.TransferID, Index: index, Status: result.Result.Status})
}
func (s *financialModelSuite) reference(reference int) *modelTransfer {
	if len(s.records) == 0 {
		return nil
	}
	for i := range s.records {
		if s.records[i].Index == reference {
			return &s.records[i]
		}
	}
	return &s.records[reference%len(s.records)]
}

func (s *financialModelSuite) step(ctx context.Context, seed int64, index int, op oracle.Operation) error {
	intent, command := s.intent(seed, index, op)
	switch op.Kind {
	case "transfer", "insufficient", "lost-response":
		expected := s.ledger.Move(intent)
		if op.Kind == "lost-response" {
			if _, err := s.transfers.Submit(ctx, command); err != nil {
				return err
			}
			repository, _ := db.NewTransferRepository(s.api, time.Now)
			s.transfers, _ = transfers.NewService(repository, time.Now)
			expected.Replayed = true
		}
		result, err := s.transfers.Submit(ctx, command)
		if err != nil {
			return err
		}
		if err := s.compareTransfer(result, expected, intent); err != nil {
			return err
		}
		s.record(index, intent, command, result)
	case "replay", "conflict":
		saved := s.reference(op.Reference)
		if saved == nil {
			return nil
		}
		intent, command = saved.Intent, saved.Command
		if op.Kind == "conflict" {
			amount, _ := strconv.ParseInt(intent.Minor, 10, 64)
			intent.Minor = strconv.FormatInt(amount+1, 10)
			command.Amount, _ = money.New("USD", amount+1)
		}
		expected := s.ledger.Move(intent)
		result, err := s.transfers.Submit(ctx, command)
		if expected.Status == "conflict" {
			if !errors.Is(err, transfers.ErrIdempotencyConflict) {
				return mismatch("outcome", "changed intent error=%v", err)
			}
			return nil
		}
		if err != nil {
			return err
		}
		if result.Result.TransferID != saved.TransferID {
			return mismatch("replay", "same intent returned another transfer")
		}
		return s.compareTransfer(result, expected, intent)
	case "fund":
		return s.fund(ctx, intent)
	case "correct":
		return s.correct(ctx, seed, index, op)
	case "rollback":
		if _, err := s.admin.ExecContext(ctx, `CREATE FUNCTION r08_model_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic model rollback failure'; END $$; CREATE TRIGGER r08_model_failure BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION r08_model_failure()`); err != nil {
			return err
		}
		_, commandErr := s.transfers.Submit(ctx, command)
		_, cleanupErr := s.admin.ExecContext(ctx, `DROP TRIGGER r08_model_failure ON outbox_events; DROP FUNCTION r08_model_failure()`)
		if cleanupErr != nil {
			return cleanupErr
		}
		if commandErr == nil {
			return mismatch("rollback", "injected failure did not reject transaction")
		}
		if err := s.check(ctx); err != nil {
			return err
		}
		expected := s.ledger.Move(intent)
		result, err := s.transfers.Submit(ctx, command)
		if err != nil {
			return err
		}
		if err := s.compareTransfer(result, expected, intent); err != nil {
			return err
		}
		s.record(index, intent, command, result)
	case "concurrent":
		expected := s.ledger.Move(intent)
		type response struct {
			result transfers.Submission
			err    error
		}
		responses := make(chan response, 4)
		var wait sync.WaitGroup
		for range 4 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, err := s.transfers.Submit(ctx, command)
				responses <- response{result, err}
			}()
		}
		wait.Wait()
		close(responses)
		var result transfers.Submission
		first, replays := 0, 0
		id := ""
		for response := range responses {
			if response.err != nil {
				return response.err
			}
			result = response.result
			if id != "" && id != result.Result.TransferID {
				return mismatch("replay", "concurrent intent returned different IDs")
			}
			id = result.Result.TransferID
			if result.Replayed {
				replays++
			} else {
				first++
			}
			observation := expected
			observation.Replayed = result.Replayed
			if err := s.compareTransfer(result, observation, intent); err != nil {
				return err
			}
		}
		if first != 1 || replays != 3 {
			return mismatch("replay", "concurrent originals=%d replays=%d", first, replays)
		}
		s.record(index, intent, command, result)
	default:
		return fmt.Errorf("unsupported generated operation %q", op.Kind)
	}
	return nil
}

func (s *financialModelSuite) fund(ctx context.Context, intent oracle.Intent) error {
	destination := intent.From
	amount, _ := strconv.ParseInt(intent.Minor, 10, 64)
	value, _ := money.New("USD", amount)
	request := appfunding.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, DestinationAccountID: s.bindings[destination], Amount: value, ExternalReference: intent.Key, EvidenceReference: "synthetic://model/" + intent.Key, IdempotencyKey: intent.Key + "-request", CorrelationID: uuid.NewString()}
	created, err := s.funding.Request(ctx, request)
	if err != nil {
		return err
	}
	approved, err := s.funding.Approve(ctx, appfunding.DecisionCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, FundingEventID: created.Event.FundingEventID, Reason: "Independent synthetic model evidence review", CorrelationID: uuid.NewString()})
	if err != nil {
		return err
	}
	if approved.Status != "approved" {
		return mismatch("funding", "approval did not produce approved state")
	}
	s.ledger.AddZeroSystem("clearing")
	intent.Operation = "funding"
	intent.Actor = modelApprover
	intent.Key += "-post"
	intent.From = "clearing"
	intent.To = destination
	expected := s.ledger.Move(intent)
	if expected.Status != "posted" {
		return mismatch("funding", "oracle did not permit bounded synthetic funding")
	}
	post := appfunding.ActionCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, FundingEventID: created.Event.FundingEventID, IdempotencyKey: intent.Key, CorrelationID: uuid.NewString()}
	result, err := s.funding.Post(ctx, post)
	if err != nil {
		return err
	}
	if result.Replayed || result.Event.Status != "posted" {
		return mismatch("funding", "funding post outcome differs")
	}
	if prior, ok := s.bindings["clearing"]; ok && prior != result.Event.SystemAccountID {
		return mismatch("funding", "clearing account changed")
	}
	s.bindings["clearing"] = result.Event.SystemAccountID
	replay, err := s.funding.Post(ctx, post)
	if err != nil {
		return err
	}
	if !replay.Replayed || replay.Event.JournalTransactionID != result.Event.JournalTransactionID {
		return mismatch("replay", "funding replay changed journal")
	}
	return nil
}

func (s *financialModelSuite) correct(ctx context.Context, seed int64, index int, op oracle.Operation) error {
	eligible := []int{}
	for i, record := range s.records {
		if record.Status != "posted" || record.Corrected {
			continue
		}
		available, _ := strconv.ParseInt(s.ledger.Accounts[record.Intent.To].Minor, 10, 64)
		amount, _ := strconv.ParseInt(record.Intent.Minor, 10, 64)
		if available >= amount {
			eligible = append(eligible, i)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	position := eligible[op.Reference%len(eligible)]
	original := s.records[position]
	key := fmt.Sprintf("model-%d-correction-%03d", seed, index)
	now := time.Now().UTC()
	created, err := s.corrections.Request(ctx, appcorrections.RequestCommand{TenantID: testTenantID, ActorSubjectID: testActorID, OriginalTransferID: original.TransferID, ReasonCode: "operational_error", OperatorNote: "Synthetic generated sequence", IdempotencyKey: key + "-request", CorrelationID: uuid.NewString(), StepUpAuthenticatedAt: now})
	if err != nil {
		return err
	}
	approved, err := s.corrections.Approve(ctx, appcorrections.DecisionCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, CorrectionID: created.Event.CorrectionID, Reason: "Independent exact model correction review", CorrelationID: uuid.NewString(), StepUpAuthenticatedAt: now})
	if err != nil {
		return err
	}
	if approved.Status != "approved" {
		return mismatch("correction", "approval state differs")
	}
	intent := original.Intent
	intent.Operation = "correction"
	intent.Actor = modelApprover
	intent.Key = key + "-post"
	intent.From, intent.To = intent.To, intent.From
	expected := s.ledger.Move(intent)
	if expected.Status != "posted" {
		return mismatch("correction", "oracle precondition changed")
	}
	post := appcorrections.PostCommand{TenantID: testTenantID, ActorSubjectID: modelApprover, CorrectionID: created.Event.CorrectionID, IdempotencyKey: intent.Key, CorrelationID: uuid.NewString(), StepUpAuthenticatedAt: now}
	result, err := s.corrections.Post(ctx, post)
	if err != nil {
		return err
	}
	if result.Replayed || result.Event.Status != "posted" || result.Event.CompensationTransferID == original.TransferID {
		return mismatch("correction", "compensation outcome differs")
	}
	replay, err := s.corrections.Post(ctx, post)
	if err != nil {
		return err
	}
	if !replay.Replayed || replay.Event.CompensationTransferID != result.Event.CompensationTransferID {
		return mismatch("replay", "compensation replay changed financial effect")
	}
	s.records[position].Corrected = true
	return nil
}
