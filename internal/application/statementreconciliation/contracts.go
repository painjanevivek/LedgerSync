package statementreconciliation

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid statement command")
	ErrConflict    = errors.New("statement intent or review version conflicts")
	ErrNotFound    = errors.New("statement evidence not found")
	ErrForbidden   = errors.New("statement authority denied")
	ErrUnavailable = errors.New("statement dependency unavailable")
	ErrLimit       = errors.New("statement capacity limit")
)

const MaxSnapshotItems = 2000
const MaxImports = 100
const ReviewPageSize = 100

type Scope struct{ TenantID, ActorSubjectID string }
type Meta struct {
	Scope
	IdempotencyKey, CorrelationID string
}
type MappingCommand struct {
	Meta
	AccountID, SourceAccountRef string
}
type ImportCommand struct {
	Meta
	MappingID string
	Source    []byte
}
type RunCommand struct {
	Meta
	ImportID string
}
type Receipt struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}
type Mapping struct {
	ID               string    `json:"id"`
	AccountID        string    `json:"account_id"`
	SourceAccountRef string    `json:"source_account_ref"`
	Currency         string    `json:"currency"`
	Version          int       `json:"version"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
}
type ImportSummary struct {
	ID           string    `json:"id"`
	MappingID    string    `json:"mapping_id"`
	SHA256       string    `json:"sha256"`
	RowCount     int       `json:"row_count"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	OpeningMinor int64     `json:"opening_minor,string"`
	ClosingMinor int64     `json:"closing_minor,string"`
	CreatedAt    time.Time `json:"created_at"`
	RunID        string    `json:"run_id,omitempty"`
	RunState     string    `json:"run_state,omitempty"`
}
type AccountOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"`
}
type Workspace struct {
	Mapping      *Mapping        `json:"mapping"`
	Imports      []ImportSummary `json:"imports"`
	Accounts     []AccountOption `json:"accounts"`
	MoreAccounts bool            `json:"more_accounts"`
}
type RunHeader struct {
	ID                      string    `json:"id"`
	ImportID                string    `json:"import_id"`
	MappingID               string    `json:"mapping_id"`
	AccountID               string    `json:"account_id"`
	SourceAccountRef        string    `json:"source_account_ref"`
	State                   string    `json:"state"`
	RuleVersion             int       `json:"rule_version"`
	MappingVersion          int       `json:"mapping_version"`
	Cutoff                  time.Time `json:"cutoff"`
	PeriodStart             time.Time `json:"period_start"`
	PeriodEnd               time.Time `json:"period_end"`
	SourceSHA256            string    `json:"source_sha256"`
	StatementRowCount       int       `json:"statement_row_count"`
	SnapshotItemCount       int       `json:"snapshot_item_count"`
	MatchedRowCount         int       `json:"matched_row_count"`
	UnmatchedStatementCount int       `json:"unmatched_statement_count"`
	UnmatchedLedgerCount    int       `json:"unmatched_ledger_count"`
}
type ReviewItem struct {
	Kind            string    `json:"kind"`
	ItemID          string    `json:"item_id"`
	Version         int64     `json:"version,string"`
	Classification  string    `json:"classification"`
	Status          string    `json:"status"`
	Reference       string    `json:"external_reference"`
	Direction       string    `json:"direction"`
	AmountMinor     int64     `json:"amount_minor,string"`
	Currency        string    `json:"currency"`
	Assignee        string    `json:"assignee"`
	Reason          string    `json:"reason"`
	Comment         string    `json:"comment"`
	Evidence        string    `json:"evidence"`
	LinkedPostingID string    `json:"linked_posting_id"`
	UpdatedBy       string    `json:"updated_by"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type ReviewInput struct {
	Kind            string `json:"kind"`
	ItemID          string `json:"item_id"`
	ExpectedVersion string `json:"expected_version"`
	Status          string `json:"status"`
	Assignee        string `json:"assignee"`
	Reason          string `json:"reason"`
	Comment         string `json:"comment"`
	Evidence        string `json:"evidence"`
	PostingID       string `json:"posting_id,omitempty"`
}
type ReviewCommand struct {
	Meta
	RunID    string
	Decision ReviewInput
}
type RunSnapshot struct {
	Header  RunHeader
	Rows    []Row
	Items   []LedgerItem
	Reviews []ReviewItem
}
type RunPage struct {
	Run        RunHeader    `json:"run"`
	Items      []ReviewItem `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}
type SourceFile struct {
	ID     string
	SHA256 string
	Bytes  []byte
}
type Preview struct {
	SHA256           string    `json:"sha256"`
	SourceAccountRef string    `json:"source_account_ref"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	OpeningMinor     int64     `json:"opening_minor,string"`
	ClosingMinor     int64     `json:"closing_minor,string"`
	RowCount         int       `json:"row_count"`
}

type Repository interface {
	CreateMapping(context.Context, MappingCommand) (Receipt, error)
	AcceptImport(context.Context, ImportCommand) (Receipt, error)
	PrepareRun(context.Context, RunCommand) (Receipt, error)
	CompleteRun(context.Context, Scope, string, MatchResult) (Receipt, error)
	Review(context.Context, ReviewCommand) (Receipt, error)
	Workspace(context.Context, Scope) (Workspace, error)
	Snapshot(context.Context, Scope, string) (RunSnapshot, error)
	Page(context.Context, Scope, string, string) (RunPage, error)
	Source(context.Context, Scope, string) (SourceFile, error)
}
