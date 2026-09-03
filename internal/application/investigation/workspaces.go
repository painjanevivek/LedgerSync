package investigation

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxInvestigationWorkspaces = 50
	MaxWorkspaceReferences     = 21
	MaxWorkspaceHistoryItems   = 100
)

var (
	ErrInvalidWorkspace      = errors.New("invalid investigation workspace")
	ErrWorkspaceNotFound     = errors.New("investigation workspace not found")
	ErrWorkspaceVersion      = errors.New("investigation workspace version conflict")
	ErrWorkspaceLimit        = errors.New("investigation workspace limit reached")
	ErrWorkspaceState        = errors.New("invalid investigation workspace state")
	workspaceReference       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{7,127}$`)
	workspaceRelationship    = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	workspaceSubject         = regexp.MustCompile(`^[^\x00-\x1f\x7f-\x9f]{1,255}$`)
	workspaceDisallowedTitle = regexp.MustCompile(`(?i)(?:https?://|bearer[[:space:]]|password|secret|token[=:]|api[_ -]?key|@)`)
)

type WorkspaceSummary struct {
	ID        string     `json:"investigation_id"`
	Title     string     `json:"title"`
	Taxonomy  string     `json:"taxonomy"`
	Status    string     `json:"status"`
	Version   string     `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

type WorkspacePage struct {
	Investigations []WorkspaceSummary `json:"investigations"`
	GeneratedAt    time.Time          `json:"generated_at"`
}

type WorkspaceQueryContext struct {
	Kind       string `json:"kind"`
	RecordType string `json:"record_type"`
	Value      string `json:"value"`
}

type WorkspaceReference struct {
	RelationshipType string    `json:"relationship_type"`
	SourceRecordType string    `json:"source_record_type,omitempty"`
	SourceRecordID   string    `json:"source_record_id,omitempty"`
	RecordType       string    `json:"record_type"`
	RecordID         string    `json:"record_id"`
	TargetPath       string    `json:"target_path"`
	CapturedAt       time.Time `json:"captured_at"`
}

type WorkspaceHistoryItem struct {
	Action                 string    `json:"action"`
	ActorIsCurrentOperator bool      `json:"actor_is_current_operator"`
	Version                string    `json:"version"`
	Status                 string    `json:"status"`
	OccurredAt             time.Time `json:"occurred_at"`
}

type WorkspaceHistoricalContext struct {
	QueryContext           WorkspaceQueryContext  `json:"query_context"`
	References             []WorkspaceReference   `json:"references"`
	WithheldReferenceCount int                    `json:"withheld_reference_count"`
	History                []WorkspaceHistoryItem `json:"history"`
	HistoryTruncated       bool                   `json:"history_truncated"`
}

type WorkspaceCurrentEvidence struct {
	Root          *SearchResult  `json:"root,omitempty"`
	Relationships []Relationship `json:"relationships"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Truncated     bool           `json:"truncated"`
	Available     bool           `json:"available"`
}

type Workspace struct {
	WorkspaceSummary
	HistoricalContext WorkspaceHistoricalContext `json:"historical_context"`
	CurrentEvidence   WorkspaceCurrentEvidence   `json:"current_evidence"`
}

type WorkspaceReceipt struct {
	InvestigationID string    `json:"investigation_id"`
	Outcome         string    `json:"outcome"`
	Version         string    `json:"version"`
	OccurredAt      time.Time `json:"occurred_at"`
}

type WorkspaceCreate struct {
	TenantID, ActorID, Title, Taxonomy, QueryKind, QueryValue, RootRecordType, RootRecordID, CorrelationID string
	Access                                                                                                 SearchAccess
	OccurredAt                                                                                             time.Time
}

type WorkspaceHandoff struct {
	TenantID, ActorID, InvestigationID, TargetSubjectID, CorrelationID string
	ExpectedVersion                                                    int64
	Access                                                             SearchAccess
	OccurredAt                                                         time.Time
}

type WorkspaceStatusChange struct {
	TenantID, ActorID, InvestigationID, TargetStatus, CorrelationID string
	ExpectedVersion                                                 int64
	Access                                                          SearchAccess
	OccurredAt                                                      time.Time
}

type WorkspaceRepository interface {
	ListWorkspaces(context.Context, string, string, SearchAccess) (WorkspacePage, error)
	CreateWorkspace(context.Context, WorkspaceCreate) (Workspace, error)
	GetWorkspace(context.Context, string, string, string, SearchAccess) (Workspace, error)
	HandoffWorkspace(context.Context, WorkspaceHandoff) (WorkspaceReceipt, error)
	ChangeWorkspaceStatus(context.Context, WorkspaceStatusChange) (WorkspaceReceipt, error)
}

func NormalizeWorkspaceTitle(value string) (string, error) {
	if !utf8.ValidString(value) || value != strings.TrimSpace(value) || utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 80 || workspaceDisallowedTitle.MatchString(value) {
		return "", ErrInvalidWorkspace
	}
	for _, character := range value {
		if unicode.IsControl(character) || character == '<' || character == '>' {
			return "", ErrInvalidWorkspace
		}
	}
	return value, nil
}

func NormalizeWorkspaceSubject(value string) (string, error) {
	if value != strings.TrimSpace(value) || !workspaceSubject.MatchString(value) {
		return "", ErrInvalidWorkspace
	}
	return value, nil
}

func NormalizeWorkspaceCreate(command WorkspaceCreate) (WorkspaceCreate, error) {
	var err error
	command.Title, err = NormalizeWorkspaceTitle(command.Title)
	if err != nil || !WorkspaceTaxonomy(command.Taxonomy) || !WorkspaceRecordType(command.RootRecordType) || !WorkspaceRecordAllowed(command.RootRecordType, command.Access) || !canonicalSavedViewUUID.MatchString(strings.ToLower(command.RootRecordID)) {
		return WorkspaceCreate{}, ErrInvalidWorkspace
	}
	command.RootRecordID = strings.ToLower(command.RootRecordID)
	if command.QueryValue != strings.TrimSpace(command.QueryValue) || len(command.QueryValue) < 8 || len(command.QueryValue) > 128 {
		return WorkspaceCreate{}, ErrInvalidWorkspace
	}
	switch command.QueryKind {
	case "immutable_id":
		command.QueryValue = strings.ToLower(command.QueryValue)
		if command.QueryValue != command.RootRecordID {
			return WorkspaceCreate{}, ErrInvalidWorkspace
		}
	case "approved_reference":
		if !workspaceReference.MatchString(command.QueryValue) {
			return WorkspaceCreate{}, ErrInvalidWorkspace
		}
	default:
		return WorkspaceCreate{}, ErrInvalidWorkspace
	}
	return command, nil
}

func WorkspaceTaxonomy(value string) bool {
	switch value {
	case "account_state", "transfer_delivery", "funding", "reconciliation", "correction", "other":
		return true
	default:
		return false
	}
}

func WorkspaceRecordType(value string) bool {
	switch value {
	case "account", "transfer", "funding", "event", "reconciliation_run", "reconciliation_mismatch", "correction":
		return true
	default:
		return false
	}
}

func WorkspaceRecordAllowed(value string, access SearchAccess) bool {
	switch value {
	case "account":
		return access.Accounts
	case "transfer":
		return access.Transfers
	case "funding":
		return access.Funding
	case "event":
		return access.Events
	case "reconciliation_run", "reconciliation_mismatch":
		return access.Reconciliation
	case "correction":
		return access.Corrections
	default:
		return false
	}
}

func WorkspaceTargetPath(recordType, recordID string) string {
	switch recordType {
	case "account":
		return "/accounts/" + recordID
	case "transfer":
		return "/transfers/" + recordID
	case "funding":
		return "/funding/" + recordID
	case "event":
		return "/events/" + recordID
	case "reconciliation_run":
		return "/reconciliation/" + recordID
	case "correction":
		return "/corrections/" + recordID
	default:
		return ""
	}
}

func ParseWorkspaceVersion(value string) (int64, error) {
	if value == "" || len(value) > 19 || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, ErrInvalidWorkspace
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, ErrInvalidWorkspace
	}
	return parsed, nil
}

func ValidWorkspaceRelationship(value string) bool { return workspaceRelationship.MatchString(value) }
