package accounts

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrAccountNotFound = errors.New("account not found or not authorized")
var ErrInvalidQuery = errors.New("invalid account query")

type Summary struct {
	AccountID         string
	Currency          string
	Status            string
	DisplayName       string
	Category          string
	ExternalReference string
	Balance           Balance
	AuditContext      []AuditEvent
}

type AuditEvent struct {
	EventID        string
	EventType      string
	ActorSubjectID string
	Outcome        string
	CorrelationID  string
	OccurredAt     time.Time
}

type Query struct {
	Cursor   string
	Limit    int
	Search   string
	Status   string
	Category string
}

type Page struct {
	Accounts   []Summary
	NextCursor string
}

type OwnedAccountRepository interface {
	ListOwnedPage(context.Context, string, string, Query) (Page, error)
	GetOwned(context.Context, string, string, string) (Summary, error)
}

type Service struct{ repository OwnedAccountRepository }

func NewService(repository OwnedAccountRepository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("owned account repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) ListOwned(ctx context.Context, tenantID, actorID string, query Query) (Page, error) {
	if s == nil || tenantID == "" || actorID == "" {
		return Page{}, ErrAccountNotFound
	}
	query.Search = strings.TrimSpace(query.Search)
	query.Status = strings.TrimSpace(strings.ToLower(query.Status))
	query.Category = strings.TrimSpace(strings.ToLower(query.Category))
	if query.Limit == 0 {
		query.Limit = 25
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Cursor) > 512 || len(query.Search) > 128 || !allowedStatus(query.Status) || !allowedCategory(query.Category) {
		return Page{}, ErrInvalidQuery
	}
	return s.repository.ListOwnedPage(ctx, tenantID, actorID, query)
}

func (s *Service) GetOwned(ctx context.Context, tenantID, actorID, accountID string) (Summary, error) {
	if s == nil || tenantID == "" || actorID == "" || strings.TrimSpace(accountID) == "" {
		return Summary{}, ErrAccountNotFound
	}
	return s.repository.GetOwned(ctx, tenantID, actorID, accountID)
}

func allowedStatus(value string) bool {
	return value == "" || value == "active" || value == "frozen" || value == "closed"
}

func allowedCategory(value string) bool {
	switch value {
	case "", "operating", "customer_funds", "payroll", "payables", "expenses", "reserve":
		return true
	default:
		return false
	}
}
