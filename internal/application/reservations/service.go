// Package reservations owns bounded internal reservation commands. Financial
// mutation and the database clock stay inside controlled database functions.
package reservations

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/idempotency"
)

var (
	ErrInvalid     = errors.New("invalid reservation command")
	ErrForbidden   = errors.New("reservation authority denied")
	ErrConflict    = errors.New("reservation intent or state conflicts")
	ErrNotFound    = errors.New("reservation not found")
	ErrUnavailable = errors.New("reservation dependency unavailable")
	ErrLimit       = errors.New("reservation capacity reached")
)

const PageSize = 50

type Scope struct{ TenantID, ActorSubjectID string }
type Meta struct {
	Scope
	IdempotencyKey, CorrelationID string
}
type Create struct {
	Meta
	DebitAccountID, CreditAccountID, AmountMinor, Currency string
	TTLSeconds                                             int
}
type Decide struct {
	Meta
	HoldID, Decision string
}
type Receipt struct {
	ID                   string     `json:"id"`
	State                string     `json:"state"`
	Replayed             bool       `json:"replayed"`
	SourceBalanceVersion string     `json:"source_balance_version,omitempty"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	TransferID           string     `json:"transfer_id,omitempty"`
	RejectionCode        string     `json:"rejection_code,omitempty"`
}
type Hold struct {
	ID              string     `json:"id"`
	DebitAccountID  string     `json:"debit_account_id"`
	CreditAccountID string     `json:"credit_account_id"`
	AmountMinor     string     `json:"amount_minor"`
	Currency        string     `json:"currency"`
	State           string     `json:"state"`
	Version         string     `json:"version"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	TransferID      string     `json:"transfer_id,omitempty"`
}
type Page struct {
	Holds      []Hold `json:"holds"`
	NextCursor string `json:"next_cursor,omitempty"`
}
type Repository interface {
	Create(context.Context, Create) (Receipt, error)
	Decide(context.Context, Decide) (Receipt, error)
	Read(context.Context, Scope, string) (Hold, error)
	List(context.Context, Scope, string) (Page, error)
	Snapshot(context.Context, Scope) (Snapshot, error)
}
type Service struct{ repo Repository }

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, ErrInvalid
	}
	return &Service{repo}, nil
}
func ValidID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}
func validScope(s Scope) bool {
	return ValidID(s.TenantID) && s.ActorSubjectID != "" && len(s.ActorSubjectID) <= 256 && utf8.ValidString(s.ActorSubjectID) && strings.TrimSpace(s.ActorSubjectID) == s.ActorSubjectID && !strings.ContainsFunc(s.ActorSubjectID, unicode.IsControl)
}
func validMeta(m Meta) bool {
	return validScope(m.Scope) && ValidID(m.CorrelationID) && idempotency.ValidateKey(m.IdempotencyKey) == nil && strings.TrimSpace(m.IdempotencyKey) == m.IdempotencyKey
}
func ValidateCreate(c Create) error {
	n, err := strconv.ParseInt(c.AmountMinor, 10, 64)
	if !validMeta(c.Meta) || !ValidID(c.DebitAccountID) || !ValidID(c.CreditAccountID) || c.DebitAccountID == c.CreditAccountID || err != nil || n <= 0 || strconv.FormatInt(n, 10) != c.AmountMinor || len(c.Currency) != 3 || c.TTLSeconds < 1 || c.TTLSeconds > 86400 {
		return ErrInvalid
	}
	for _, r := range c.Currency {
		if r < 'A' || r > 'Z' {
			return ErrInvalid
		}
	}
	return nil
}
func (s *Service) Create(ctx context.Context, c Create) (Receipt, error) {
	if err := ValidateCreate(c); err != nil {
		return Receipt{}, err
	}
	return s.repo.Create(ctx, c)
}
func (s *Service) Decide(ctx context.Context, c Decide) (Receipt, error) {
	if !validMeta(c.Meta) || !ValidID(c.HoldID) || (c.Decision != "capture" && c.Decision != "release") {
		return Receipt{}, ErrInvalid
	}
	return s.repo.Decide(ctx, c)
}
func (s *Service) Read(ctx context.Context, scope Scope, id string) (Hold, error) {
	if !validScope(scope) || !ValidID(id) {
		return Hold{}, ErrInvalid
	}
	return s.repo.Read(ctx, scope, id)
}
func (s *Service) List(ctx context.Context, scope Scope, cursor string) (Page, error) {
	if !validScope(scope) || len(cursor) > 256 {
		return Page{}, ErrInvalid
	}
	return s.repo.List(ctx, scope, cursor)
}
