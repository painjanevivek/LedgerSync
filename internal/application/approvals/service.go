// Package approvals defines the tenant-scoped read model for independent
// funding and correction decisions. It composes approval evidence without
// weakening either domain's command or separation-of-duty rules.
package approvals

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidQuery = errors.New("invalid approval query")
	ErrForbidden    = errors.New("approval query forbidden")
)

type Domain string

const (
	DomainFunding    Domain = "funding"
	DomainCorrection Domain = "correction"
)

type StepUpStatus string

const (
	StepUpNotRequired StepUpStatus = "not_required"
	StepUpSatisfied   StepUpStatus = "satisfied"
	StepUpRequired    StepUpStatus = "required"
)

type Item struct {
	Domain              Domain       `json:"domain"`
	RecordID            string       `json:"record_id"`
	RequesterSubjectID  string       `json:"requester_subject_id"`
	RequestedAt         string       `json:"requested_at"`
	AgeSeconds          string       `json:"age_seconds"`
	Status              string       `json:"status"`
	AmountMinor         string       `json:"amount_minor"`
	Currency            string       `json:"currency"`
	RelatedAccountID    string       `json:"related_account_id,omitempty"`
	RelatedTransferID   string       `json:"related_transfer_id,omitempty"`
	EvidenceComplete    bool         `json:"evidence_complete"`
	SelfApprovalBlocked bool         `json:"self_approval_blocked"`
	ActionableByMe      bool         `json:"actionable_by_me"`
	RequiredScope       string       `json:"required_scope"`
	StepUpStatus        StepUpStatus `json:"step_up_status"`
	ApprovalExpiresAt   string       `json:"approval_expires_at,omitempty"`
	SafeNextAction      string       `json:"safe_next_action"`
}

type Query struct {
	Domain                Domain
	StatusDomain          Domain
	Status                string
	Requester             string
	Age                   string
	RequestedAfter        time.Time
	RequestedBefore       time.Time
	ActionableOnly        bool
	Cursor                string
	Limit                 int
	CanApproveFunding     bool
	CanApproveCorrections bool
	StepUpAuthenticatedAt time.Time
	Now                   time.Time
}

type Page struct {
	Items      []Item `json:"items"`
	PageCount  int    `json:"page_count"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type Repository interface {
	List(context.Context, string, string, Query) (Page, error)
}

type Service struct {
	repository Repository
	clock      func() time.Time
}

func NewService(repository Repository, clock func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, errors.New("approval repository is required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository: repository, clock: clock}, nil
}

func (s *Service) List(ctx context.Context, tenantID, actorID string, query Query) (Page, error) {
	tenantID = strings.TrimSpace(tenantID)
	actorID = strings.TrimSpace(actorID)
	query.Requester = strings.TrimSpace(query.Requester)
	query.Status = strings.TrimSpace(query.Status)
	query.Cursor = strings.TrimSpace(query.Cursor)
	query.Age = strings.TrimSpace(query.Age)
	if query.Now.IsZero() {
		query.Now = s.clock().UTC()
	} else {
		query.Now = query.Now.UTC()
	}
	if tenantID == "" || actorID == "" || query.Limit < 1 || query.Limit > 100 || len(query.Requester) > 255 || len(query.Cursor) > 2048 || !validDomain(query.Domain, true) || !validDomain(query.StatusDomain, true) || !validStatus(query.StatusDomain, query.Status) || !validAge(query.Age) || (query.Domain != "" && query.StatusDomain != "" && query.Domain != query.StatusDomain) || (!query.RequestedAfter.IsZero() && !query.RequestedBefore.IsZero() && query.RequestedAfter.After(query.RequestedBefore)) {
		return Page{}, ErrInvalidQuery
	}
	if !query.CanApproveFunding && !query.CanApproveCorrections {
		return Page{}, ErrForbidden
	}
	if (query.Domain == DomainFunding && !query.CanApproveFunding) ||
		(query.Domain == DomainCorrection && !query.CanApproveCorrections) ||
		(query.StatusDomain == DomainFunding && !query.CanApproveFunding) ||
		(query.StatusDomain == DomainCorrection && !query.CanApproveCorrections) {
		return Page{}, ErrForbidden
	}
	return s.repository.List(ctx, tenantID, actorID, query)
}

func validDomain(domain Domain, allowEmpty bool) bool {
	return (allowEmpty && domain == "") || domain == DomainFunding || domain == DomainCorrection
}

func validStatus(domain Domain, status string) bool {
	if status == "" {
		return domain == ""
	}
	if domain == DomainFunding {
		return member(status, "requested", "approved", "posted", "rejected", "compensated")
	}
	if domain == DomainCorrection {
		return member(status, "requested", "approved", "rejected", "cancelled", "expired", "posted")
	}
	return false
}

func validAge(age string) bool {
	return member(age, "", "under_24h", "over_24h", "over_7d", "over_30d")
}

func member(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func StepUpIsRecent(authenticatedAt, now time.Time) bool {
	return !authenticatedAt.IsZero() && !authenticatedAt.After(now.Add(time.Minute)) && now.Sub(authenticatedAt) <= 10*time.Minute
}
