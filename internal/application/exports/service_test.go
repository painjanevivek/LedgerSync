package exports

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
)

type exportRepositoryStub struct {
	transferPages int
	transfers     []investigation.TransferSummary
	account       []transactions.Entry
	runs          []investigation.ReconciliationRun
	mismatches    []investigation.ReconciliationMismatch
	err           error
}

func (r *exportRepositoryStub) ListTransfers(ctx context.Context, _ string, filter investigation.TransferFilter) ([]investigation.TransferSummary, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if r.err != nil {
		return nil, "", r.err
	}
	r.transferPages++
	start := 0
	if filter.Cursor == "page-2" {
		start = 2
	}
	end := min(start+filter.Limit, len(r.transfers))
	next := ""
	if end < len(r.transfers) {
		next = "page-2"
	}
	return append([]investigation.TransferSummary(nil), r.transfers[start:end]...), next, nil
}

func (r *exportRepositoryStub) ListAccountHistory(ctx context.Context, _, _, _, cursor string, limit int) ([]transactions.Entry, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if r.err != nil {
		return nil, "", r.err
	}
	start := 0
	if cursor != "" {
		start = 1
	}
	end := min(start+limit, len(r.account))
	next := ""
	if end < len(r.account) {
		next = "account-next"
	}
	return append([]transactions.Entry(nil), r.account[start:end]...), next, nil
}

func (r *exportRepositoryStub) ListReconciliationRuns(ctx context.Context, _ string, filter ReconciliationFilter) ([]investigation.ReconciliationRun, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if r.err != nil {
		return nil, "", r.err
	}
	return append([]investigation.ReconciliationRun(nil), r.runs...), "", nil
}

func (r *exportRepositoryStub) ListReconciliationMismatches(ctx context.Context, _, _, _ string, _ int) ([]investigation.ReconciliationMismatch, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if r.err != nil {
		return nil, "", r.err
	}
	return append([]investigation.ReconciliationMismatch(nil), r.mismatches...), "", nil
}

func TestTransferExportPaginatesQuotesExactFieldsAndNeutralizesFormulaCorpus(t *testing.T) {
	when := time.Date(2026, 8, 25, 1, 2, 3, 4, time.UTC)
	formulae := []string{"=SUM(1,1)", "+cmd", "-cmd", "@cmd", "\tcmd", "\rcmd", "   =cmd"}
	for _, formula := range formulae {
		repository := &exportRepositoryStub{transfers: []investigation.TransferSummary{
			{ID: "00000000-0000-4000-8000-000000000001", DebitAccountID: "00000000-0000-4000-8000-000000000011", CreditAccountID: "00000000-0000-4000-8000-000000000012", AmountMinor: "12550", Currency: "INR", FinancialStatus: "posted", DeliveryStatus: "delivered", CreatedAt: when, CompletedAt: when, RejectionCode: formula, CorrectionID: "00000000-0000-4000-8000-000000000021", CorrectionStatus: "posted", CorrectionRole: "original", OriginalTransferID: "00000000-0000-4000-8000-000000000001", CompensationTransferID: "00000000-0000-4000-8000-000000000022"},
			{ID: "00000000-0000-4000-8000-000000000002", DebitAccountID: "00000000-0000-4000-8000-000000000011", CreditAccountID: "00000000-0000-4000-8000-000000000012", AmountMinor: "1", Currency: "INR", FinancialStatus: "posted", DeliveryStatus: "delivered", CreatedAt: when, CompletedAt: when},
			{ID: "00000000-0000-4000-8000-000000000003", DebitAccountID: "00000000-0000-4000-8000-000000000011", CreditAccountID: "00000000-0000-4000-8000-000000000012", AmountMinor: "2", Currency: "INR", FinancialStatus: "posted", DeliveryStatus: "delivered", CreatedAt: when, CompletedAt: when},
		}}
		service, err := NewService(repository, 10, 2)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		result, err := service.StreamTransfers(context.Background(), "tenant", investigation.TransferFilter{}, 3, &output)
		if err != nil || result.Rows != 3 || repository.transferPages != 2 {
			t.Fatalf("formula=%q result=%+v pages=%d err=%v", formula, result, repository.transferPages, err)
		}
		if !strings.Contains(output.String(), `"12550","INR"`) || !strings.Contains(output.String(), `"'`) {
			t.Fatalf("formula=%q was not exact and neutralized:\n%s", formula, output.String())
		}
		if !strings.Contains(output.String(), `"posted","original","00000000-0000-4000-8000-000000000001","00000000-0000-4000-8000-000000000022"`) {
			t.Fatalf("compensation relationship missing from export:\n%s", output.String())
		}
		for _, line := range strings.Split(strings.TrimSpace(output.String()), "\r\n") {
			if !strings.HasPrefix(line, `"`) || !strings.HasSuffix(line, `"`) {
				t.Fatalf("CSV row is not fully quoted: %q", line)
			}
		}
	}
}

func TestAccountAndReconciliationExportsPreserveMinorStrings(t *testing.T) {
	when := time.Date(2026, 8, 25, 1, 2, 3, 0, time.UTC)
	repository := &exportRepositoryStub{
		account:    []transactions.Entry{{TransferID: "00000000-0000-4000-8000-000000000001", Direction: "debit", Amount: "99", Currency: "INR", Status: "posted", OccurredAt: when, CorrectionID: "00000000-0000-4000-8000-000000000021", CorrectionStatus: "posted", CorrectionRole: "compensation", OriginalTransferID: "00000000-0000-4000-8000-000000000020", CompensationTransferID: "00000000-0000-4000-8000-000000000001"}},
		runs:       []investigation.ReconciliationRun{{ID: "00000000-0000-4000-8000-000000000002", Status: "mismatch", CorrelationID: "00000000-0000-4000-8000-000000000003", Scope: "tenant", LedgerWatermark: "1", ApplicationVersion: "v1", SchemaVersion: "000015", CheckedAccountCount: "2", PostingCount: "2", MismatchCount: "1", StartedAt: when, CompletedAt: when}},
		mismatches: []investigation.ReconciliationMismatch{{ID: "00000000-0000-4000-8000-000000000004", AccountID: "00000000-0000-4000-8000-000000000005", Classification: "projection_mismatch", Currency: "INR", ExpectedMinor: "-1", ObservedMinor: "0", ObservedAvailableMinor: "0", BalanceVersion: "2", CreatedAt: when}},
	}
	service, _ := NewService(repository, 10, 2)
	var account bytes.Buffer
	if result, err := service.StreamAccountLedger(context.Background(), "tenant", "actor", "account", 10, &account); err != nil || result.Rows != 1 || !strings.Contains(account.String(), `"99","INR"`) || !strings.Contains(account.String(), `"posted","compensation"`) {
		t.Fatalf("account result=%+v err=%v csv=%s", result, err, account.String())
	}
	var reconciliation bytes.Buffer
	if result, err := service.StreamReconciliation(context.Background(), "tenant", ReconciliationFilter{}, 10, &reconciliation); err != nil || result.Rows != 2 || !strings.Contains(reconciliation.String(), `"INR","-1","0","0","2"`) {
		t.Fatalf("reconciliation result=%+v err=%v csv=%s", result, err, reconciliation.String())
	}
}

func TestExportStopsOnCancellationAndBoundsRows(t *testing.T) {
	repository := &exportRepositoryStub{transfers: []investigation.TransferSummary{{AmountMinor: "1", Currency: "INR"}}}
	service, _ := NewService(repository, 1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.StreamTransfers(ctx, "tenant", investigation.TransferFilter{}, 1, &bytes.Buffer{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled export error=%v", err)
	}
	if _, err := service.StreamTransfers(context.Background(), "tenant", investigation.TransferFilter{}, 2, &bytes.Buffer{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("over-bound export error=%v", err)
	}
}
