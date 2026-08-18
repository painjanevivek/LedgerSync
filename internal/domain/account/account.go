// Package account defines account state and ownership checks used by financial
// write/read use cases. Repository code must still enforce the same predicate.
package account

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
)

var (
	ErrInvalidAccount = errors.New("invalid account")
	ErrUnauthorized   = errors.New("account access denied")
	ErrInactive       = errors.New("account is not active")
)

type Status string

const (
	StatusActive Status = "active"
	StatusFrozen Status = "frozen"
	StatusClosed Status = "closed"
)

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionDebit Permission = "debit"
)

// Owner is an independently authenticated principal granted access to an
// account. A debit permission implies read permission at the authorization
// layer; database enforcement must mirror this representation.
type Owner struct {
	SubjectID  string
	Permission Permission
}

// Account is the authoritative account configuration. Current balances live
// in a separately versioned projection so the ledger stays immutable.
type Account struct {
	ID        string
	TenantID  string
	Currency  money.Currency
	Status    Status
	Owners    []Owner
	CreatedAt time.Time
}

func New(id, tenantID, currencyCode string, owners []Owner, createdAt time.Time) (Account, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(tenantID) == "" {
		return Account{}, fmt.Errorf("%w: id and tenant are required", ErrInvalidAccount)
	}
	currency, err := money.LookupCurrency(currencyCode)
	if err != nil {
		return Account{}, err
	}
	if len(owners) == 0 {
		return Account{}, fmt.Errorf("%w: an owner is required", ErrInvalidAccount)
	}
	seen := make(map[string]struct{}, len(owners))
	for _, owner := range owners {
		if strings.TrimSpace(owner.SubjectID) == "" {
			return Account{}, fmt.Errorf("%w: owner subject is required", ErrInvalidAccount)
		}
		if owner.Permission != PermissionRead && owner.Permission != PermissionDebit {
			return Account{}, fmt.Errorf("%w: invalid owner permission", ErrInvalidAccount)
		}
		if _, duplicate := seen[owner.SubjectID]; duplicate {
			return Account{}, fmt.Errorf("%w: duplicate owner", ErrInvalidAccount)
		}
		seen[owner.SubjectID] = struct{}{}
	}
	return Account{ID: id, TenantID: tenantID, Currency: currency, Status: StatusActive, Owners: owners, CreatedAt: createdAt.UTC()}, nil
}

func (a Account) CanRead(subjectID string) bool {
	for _, owner := range a.Owners {
		if owner.SubjectID == subjectID {
			return true
		}
	}
	return false
}

func (a Account) CanDebit(subjectID string) bool {
	for _, owner := range a.Owners {
		if owner.SubjectID == subjectID && owner.Permission == PermissionDebit {
			return true
		}
	}
	return false
}

// ValidateDebit rejects inactive accounts, non-owners, and currency mismatch
// before any repository locks or balance changes are attempted.
func (a Account) ValidateDebit(subjectID string, amount money.Money) error {
	if a.Status != StatusActive {
		return ErrInactive
	}
	if !a.CanDebit(subjectID) {
		return ErrUnauthorized
	}
	if amount.Currency() != a.Currency {
		return money.ErrCurrencyMismatch
	}
	if !amount.IsPositive() {
		return fmt.Errorf("%w: transfer amount must be positive", money.ErrInvalidAmount)
	}
	return nil
}

func (a Account) ValidateCredit(amount money.Money) error {
	if a.Status != StatusActive {
		return ErrInactive
	}
	if amount.Currency() != a.Currency {
		return money.ErrCurrencyMismatch
	}
	if !amount.IsPositive() {
		return fmt.Errorf("%w: transfer amount must be positive", money.ErrInvalidAmount)
	}
	return nil
}
