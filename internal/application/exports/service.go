// Package exports streams bounded, exact evidence without materializing a full
// export in memory. Persistence adapters retain tenant/object authorization and
// stable keyset pagination.
package exports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
)

const (
	SchemaVersion   = "2"
	DefaultMaxRows  = 10_000
	DefaultPageSize = 250
)

var (
	ErrInvalidRequest = errors.New("invalid export request")
	ErrUnavailable    = errors.New("export evidence unavailable")
	exactMinorPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	exactCountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
	currencyPattern   = regexp.MustCompile(`^[A-Z]{3}$`)
)

type ReconciliationFilter struct {
	RunID, Status, Cursor string
	From, To              time.Time
	Limit                 int
}

type Repository interface {
	ListTransfers(context.Context, string, investigation.TransferFilter) ([]investigation.TransferSummary, string, error)
	ListAccountHistory(context.Context, string, string, string, string, int) ([]transactions.Entry, string, error)
	ListReconciliationRuns(context.Context, string, ReconciliationFilter) ([]investigation.ReconciliationRun, string, error)
	ListReconciliationMismatches(context.Context, string, string, string, int) ([]investigation.ReconciliationMismatch, string, error)
}

type Service struct {
	repository Repository
	pageSize   int
	maxRows    int
}

type Result struct {
	Rows       int
	Truncated  bool
	SchemaName string
}

func NewService(repository Repository, maxRows, pageSize int) (*Service, error) {
	if repository == nil {
		return nil, errors.New("export repository is required")
	}
	if maxRows == 0 {
		maxRows = DefaultMaxRows
	}
	if pageSize == 0 {
		pageSize = DefaultPageSize
	}
	if maxRows < 1 || maxRows > 100_000 || pageSize < 1 || pageSize > 1_000 {
		return nil, errors.New("export bounds are invalid")
	}
	return &Service{repository: repository, maxRows: maxRows, pageSize: pageSize}, nil
}

func (s *Service) StreamTransfers(ctx context.Context, tenantID string, filter investigation.TransferFilter, requestedRows int, destination io.Writer) (Result, error) {
	limit, err := s.validate(ctx, tenantID, requestedRows, destination)
	if err != nil {
		return Result{}, err
	}
	writer := newQuotedCSVWriter(destination)
	if err := writer.Row([]string{"schema_version", "transfer_id", "source_account_id", "destination_account_id", "amount_minor", "currency", "financial_status", "delivery_status", "created_at_utc", "completed_at_utc", "journal_transaction_id", "rejection_code", "correction_id", "correction_status", "correction_role", "original_transfer_id", "compensation_transfer_id", "original_journal_id", "compensation_journal_id"}); err != nil {
		return Result{}, err
	}
	result := Result{SchemaName: "ledgersync.transfers.v2"}
	filter.Cursor = ""
	for result.Rows < limit {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		filter.Limit = min(s.pageSize, limit-result.Rows)
		items, next, listErr := s.repository.ListTransfers(ctx, tenantID, filter)
		if listErr != nil {
			return result, fmt.Errorf("%w: list transfers", ErrUnavailable)
		}
		for _, item := range items {
			row, rowErr := transferRow(item)
			if rowErr != nil {
				return result, rowErr
			}
			if rowErr = writer.Row(row); rowErr != nil {
				return result, rowErr
			}
			result.Rows++
		}
		if next == "" || len(items) == 0 {
			return result, nil
		}
		if next == filter.Cursor {
			return result, fmt.Errorf("%w: transfer cursor did not advance", ErrUnavailable)
		}
		filter.Cursor = next
	}
	result.Truncated = true
	return result, nil
}

func (s *Service) StreamAccountLedger(ctx context.Context, tenantID, actorID, accountID string, requestedRows int, destination io.Writer) (Result, error) {
	limit, err := s.validate(ctx, tenantID, requestedRows, destination)
	if err != nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(accountID) == "" {
		return Result{}, ErrInvalidRequest
	}
	writer := newQuotedCSVWriter(destination)
	if err := writer.Row([]string{"schema_version", "transfer_id", "direction", "amount_minor", "currency", "status", "occurred_at_utc", "correction_id", "correction_status", "correction_role", "original_transfer_id", "compensation_transfer_id"}); err != nil {
		return Result{}, err
	}
	result, cursor := Result{SchemaName: "ledgersync.account-ledger.v2"}, ""
	for result.Rows < limit {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		items, next, listErr := s.repository.ListAccountHistory(ctx, tenantID, actorID, accountID, cursor, min(s.pageSize, limit-result.Rows))
		if listErr != nil {
			return result, listErr
		}
		for _, item := range items {
			row, rowErr := accountLedgerRow(item)
			if rowErr != nil {
				return result, rowErr
			}
			if rowErr = writer.Row(row); rowErr != nil {
				return result, rowErr
			}
			result.Rows++
		}
		if next == "" || len(items) == 0 {
			return result, nil
		}
		if next == cursor {
			return result, fmt.Errorf("%w: account cursor did not advance", ErrUnavailable)
		}
		cursor = next
	}
	result.Truncated = true
	return result, nil
}

func (s *Service) StreamReconciliation(ctx context.Context, tenantID string, filter ReconciliationFilter, requestedRows int, destination io.Writer) (Result, error) {
	limit, err := s.validate(ctx, tenantID, requestedRows, destination)
	if err != nil {
		return Result{}, err
	}
	writer := newQuotedCSVWriter(destination)
	header := []string{"schema_version", "record_type", "run_id", "status", "correlation_id", "scope", "ledger_watermark", "application_version", "database_schema_version", "checked_account_count", "posting_count", "mismatch_count", "started_at_utc", "completed_at_utc", "mismatch_id", "account_id", "classification", "currency", "expected_minor", "observed_minor", "observed_available_minor", "balance_version", "created_at_utc"}
	if err := writer.Row(header); err != nil {
		return Result{}, err
	}
	result := Result{SchemaName: "ledgersync.reconciliation.v2"}
	filter.Cursor = ""
	for result.Rows < limit {
		filter.Limit = min(s.pageSize, limit-result.Rows)
		runs, next, listErr := s.repository.ListReconciliationRuns(ctx, tenantID, filter)
		if listErr != nil {
			return result, fmt.Errorf("%w: list reconciliation runs", ErrUnavailable)
		}
		for _, run := range runs {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			row, rowErr := reconciliationRunRow(run)
			if rowErr != nil {
				return result, rowErr
			}
			if rowErr = writer.Row(row); rowErr != nil {
				return result, rowErr
			}
			result.Rows++
			mismatchCursor := ""
			for result.Rows < limit {
				mismatches, mismatchNext, mismatchErr := s.repository.ListReconciliationMismatches(ctx, tenantID, run.ID, mismatchCursor, min(s.pageSize, limit-result.Rows))
				if mismatchErr != nil {
					return result, fmt.Errorf("%w: list reconciliation mismatches", ErrUnavailable)
				}
				for _, mismatch := range mismatches {
					mismatchRow, mappingErr := reconciliationMismatchRow(run, mismatch)
					if mappingErr != nil {
						return result, mappingErr
					}
					if mappingErr = writer.Row(mismatchRow); mappingErr != nil {
						return result, mappingErr
					}
					result.Rows++
				}
				if mismatchNext == "" || len(mismatches) == 0 {
					break
				}
				if mismatchNext == mismatchCursor {
					return result, fmt.Errorf("%w: mismatch cursor did not advance", ErrUnavailable)
				}
				mismatchCursor = mismatchNext
			}
			if result.Rows >= limit {
				result.Truncated = true
				return result, nil
			}
		}
		if next == "" || len(runs) == 0 {
			return result, nil
		}
		if next == filter.Cursor {
			return result, fmt.Errorf("%w: reconciliation cursor did not advance", ErrUnavailable)
		}
		filter.Cursor = next
	}
	result.Truncated = true
	return result, nil
}

func (s *Service) validate(ctx context.Context, tenantID string, requestedRows int, destination io.Writer) (int, error) {
	if s == nil || s.repository == nil || ctx == nil || strings.TrimSpace(tenantID) == "" || destination == nil {
		return 0, ErrInvalidRequest
	}
	if requestedRows == 0 {
		requestedRows = s.maxRows
	}
	if requestedRows < 1 || requestedRows > s.maxRows {
		return 0, ErrInvalidRequest
	}
	return requestedRows, nil
}

func transferRow(item investigation.TransferSummary) ([]string, error) {
	if !exactMinorPattern.MatchString(item.AmountMinor) || !currencyPattern.MatchString(item.Currency) {
		return nil, fmt.Errorf("%w: transfer exact fields", ErrUnavailable)
	}
	return []string{SchemaVersion, item.ID, item.DebitAccountID, item.CreditAccountID, item.AmountMinor, item.Currency, safeText(item.FinancialStatus), safeText(item.DeliveryStatus), utc(item.CreatedAt), utc(item.CompletedAt), item.JournalTransactionID, safeText(item.RejectionCode), item.CorrectionID, safeText(item.CorrectionStatus), safeText(item.CorrectionRole), item.OriginalTransferID, item.CompensationTransferID, item.OriginalJournalID, item.CompensationJournalID}, nil
}

func accountLedgerRow(item transactions.Entry) ([]string, error) {
	if !exactMinorPattern.MatchString(item.Amount) || !currencyPattern.MatchString(item.Currency) {
		return nil, fmt.Errorf("%w: account ledger exact fields", ErrUnavailable)
	}
	return []string{SchemaVersion, item.TransferID, safeText(item.Direction), item.Amount, item.Currency, safeText(item.Status), utc(item.OccurredAt), item.CorrectionID, safeText(item.CorrectionStatus), safeText(item.CorrectionRole), item.OriginalTransferID, item.CompensationTransferID}, nil
}

func reconciliationRunRow(item investigation.ReconciliationRun) ([]string, error) {
	for _, count := range []string{item.CheckedAccountCount, item.PostingCount, item.MismatchCount} {
		if !exactCountPattern.MatchString(count) {
			return nil, fmt.Errorf("%w: reconciliation count", ErrUnavailable)
		}
	}
	return []string{SchemaVersion, "run", item.ID, safeText(item.Status), item.CorrelationID, safeText(item.Scope), safeText(item.LedgerWatermark), safeText(item.ApplicationVersion), safeText(item.SchemaVersion), item.CheckedAccountCount, item.PostingCount, item.MismatchCount, utc(item.StartedAt), utc(item.CompletedAt), "", "", "", "", "", "", "", "", ""}, nil
}

func reconciliationMismatchRow(run investigation.ReconciliationRun, item investigation.ReconciliationMismatch) ([]string, error) {
	for _, minor := range []string{item.ExpectedMinor, item.ObservedMinor, item.ObservedAvailableMinor} {
		if minor != "" && !exactMinorPattern.MatchString(minor) {
			return nil, fmt.Errorf("%w: mismatch exact minor", ErrUnavailable)
		}
	}
	if item.Currency != "" && !currencyPattern.MatchString(item.Currency) {
		return nil, fmt.Errorf("%w: mismatch currency", ErrUnavailable)
	}
	if item.BalanceVersion != "" && !exactCountPattern.MatchString(item.BalanceVersion) {
		return nil, fmt.Errorf("%w: mismatch balance version", ErrUnavailable)
	}
	return []string{SchemaVersion, "mismatch", run.ID, safeText(run.Status), run.CorrelationID, "", "", "", "", "", "", "", "", "", item.ID, item.AccountID, safeText(item.Classification), item.Currency, item.ExpectedMinor, item.ObservedMinor, item.ObservedAvailableMinor, item.BalanceVersion, utc(item.CreatedAt)}, nil
}

func utc(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func safeText(value string) string {
	original := strings.TrimLeft(value, " ")
	dangerous := original != "" && strings.ContainsRune("=+-@\t\r", []rune(original)[0])
	value = strings.Map(func(character rune) rune {
		if character == '\r' || character == '\n' || !utf8.ValidRune(character) {
			return ' '
		}
		return character
	}, value)
	if utf8.RuneCountInString(value) > 512 {
		value = string([]rune(value)[:512])
	}
	if dangerous {
		return "'" + value
	}
	return value
}

type quotedCSVWriter struct{ destination io.Writer }

func newQuotedCSVWriter(destination io.Writer) quotedCSVWriter {
	return quotedCSVWriter{destination: destination}
}

func (w quotedCSVWriter) Row(fields []string) error {
	for index, field := range fields {
		if index > 0 {
			if _, err := io.WriteString(w.destination, ","); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w.destination, `"`+strings.ReplaceAll(field, `"`, `""`)+`"`); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w.destination, "\r\n")
	return err
}

func FilterFingerprint(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
