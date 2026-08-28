package account

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Metadata contains the operator-editable account attributes. Tenant,
// currency, identifier, creation time, and financial history are deliberately
// absent because they are immutable.
type Metadata struct {
	DisplayName       string
	ExternalReference string
	Category          string
}

// FinancialState is captured while the account and projection rows are
// locked. Consistent is true only after the repository has compared the
// immutable opening baseline plus postings with the current projection.
type FinancialState struct {
	AvailableMinor int64
	LedgerMinor    int64
	Consistent     bool
}

// NormalizeMetadata preserves intentional Unicode in display names while
// giving references and categories stable case-insensitive semantics.
func NormalizeMetadata(value Metadata) (Metadata, error) {
	value.DisplayName = strings.TrimSpace(value.DisplayName)
	value.ExternalReference = strings.ToLower(strings.TrimSpace(value.ExternalReference))
	value.Category = strings.ToLower(strings.TrimSpace(value.Category))
	if value.DisplayName == "" || !utf8.ValidString(value.DisplayName) || utf8.RuneCountInString(value.DisplayName) > MaxDisplayNameRunes {
		return Metadata{}, fmt.Errorf("%w: display name must contain 1-%d Unicode characters", ErrInvalidMetadata, MaxDisplayNameRunes)
	}
	for _, character := range value.DisplayName {
		if unicode.IsControl(character) {
			return Metadata{}, fmt.Errorf("%w: display name contains a control character", ErrInvalidMetadata)
		}
	}
	if len(value.ExternalReference) < 3 || len(value.ExternalReference) > MaxReferenceLength {
		return Metadata{}, fmt.Errorf("%w: reference must contain 3-%d characters", ErrInvalidMetadata, MaxReferenceLength)
	}
	for index, character := range value.ExternalReference {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || (index > 0 && (character == '.' || character == '_' || character == '-'))
		if !valid {
			return Metadata{}, fmt.Errorf("%w: reference must be lowercase ASCII letters, digits, dots, underscores, or hyphens", ErrInvalidMetadata)
		}
	}
	if !ValidCategory(value.Category) {
		return Metadata{}, fmt.Errorf("%w: unsupported category", ErrInvalidMetadata)
	}
	return value, nil
}

func ValidCategory(value string) bool {
	switch value {
	case "operating", "customer_funds", "payroll", "payables", "expenses", "reserve":
		return true
	default:
		return false
	}
}

func NewConfigured(id, tenantID, currencyCode string, metadata Metadata, owner Owner, createdAt time.Time) (Account, error) {
	metadata, err := NormalizeMetadata(metadata)
	if err != nil {
		return Account{}, err
	}
	created, err := New(id, tenantID, currencyCode, []Owner{owner}, createdAt)
	if err != nil {
		return Account{}, err
	}
	created.DisplayName = metadata.DisplayName
	created.ExternalReference = metadata.ExternalReference
	created.Category = metadata.Category
	return created, nil
}

// Restore rebuilds an aggregate from an already tenant-scoped repository row.
// It rejects malformed persisted state rather than allowing lifecycle methods
// to operate on it.
func Restore(id, tenantID, currencyCode string, status Status, metadata Metadata, version int64, owners []Owner, createdAt, updatedAt, closedAt time.Time) (Account, error) {
	restored, err := New(id, tenantID, currencyCode, owners, createdAt)
	if err != nil {
		return Account{}, err
	}
	metadata, err = NormalizeMetadata(metadata)
	if err != nil {
		return Account{}, err
	}
	if status != StatusActive && status != StatusFrozen && status != StatusClosed || version < 1 || updatedAt.IsZero() {
		return Account{}, fmt.Errorf("%w: invalid persisted lifecycle state", ErrInvalidAccount)
	}
	if status == StatusClosed && closedAt.IsZero() || status != StatusClosed && !closedAt.IsZero() {
		return Account{}, fmt.Errorf("%w: invalid closed timestamp", ErrInvalidAccount)
	}
	restored.Status = status
	restored.DisplayName = metadata.DisplayName
	restored.ExternalReference = metadata.ExternalReference
	restored.Category = metadata.Category
	restored.Version = version
	restored.UpdatedAt = updatedAt.UTC()
	restored.ClosedAt = closedAt.UTC()
	return restored, nil
}

func (a *Account) UpdateMetadata(metadata Metadata, expectedVersion int64, occurredAt time.Time) error {
	if err := a.prepareMutation(expectedVersion, occurredAt); err != nil {
		return err
	}
	metadata, err := NormalizeMetadata(metadata)
	if err != nil {
		return err
	}
	a.DisplayName = metadata.DisplayName
	a.ExternalReference = metadata.ExternalReference
	a.Category = metadata.Category
	a.advance(occurredAt)
	return nil
}

func (a *Account) Freeze(expectedVersion int64, occurredAt time.Time) error {
	if err := a.prepareMutation(expectedVersion, occurredAt); err != nil {
		return err
	}
	if a.Status != StatusActive {
		return fmt.Errorf("%w: only an active account may be frozen", ErrInvalidTransition)
	}
	a.Status = StatusFrozen
	a.advance(occurredAt)
	return nil
}

func (a *Account) Reactivate(expectedVersion int64, occurredAt time.Time) error {
	if err := a.prepareMutation(expectedVersion, occurredAt); err != nil {
		return err
	}
	if a.Status != StatusFrozen {
		return fmt.Errorf("%w: only a frozen account may be reactivated", ErrInvalidTransition)
	}
	a.Status = StatusActive
	a.advance(occurredAt)
	return nil
}

func (a *Account) Close(expectedVersion int64, state FinancialState, occurredAt time.Time) error {
	if err := a.prepareMutation(expectedVersion, occurredAt); err != nil {
		return err
	}
	if a.Status != StatusActive && a.Status != StatusFrozen {
		return fmt.Errorf("%w: only an active or frozen account may be closed", ErrInvalidTransition)
	}
	if !state.Consistent {
		return ErrFinancialStateUnavailable
	}
	if state.AvailableMinor != 0 || state.LedgerMinor != 0 {
		return ErrNonZeroBalance
	}
	a.Status = StatusClosed
	a.ClosedAt = occurredAt.UTC()
	a.advance(occurredAt)
	return nil
}

func (a *Account) prepareMutation(expectedVersion int64, occurredAt time.Time) error {
	if a == nil || occurredAt.IsZero() {
		return fmt.Errorf("%w: account and occurrence time are required", ErrInvalidAccount)
	}
	if a.Status == StatusClosed {
		return ErrTerminalStatus
	}
	if expectedVersion < 1 || expectedVersion != a.Version {
		return ErrVersionConflict
	}
	if occurredAt.UTC().Before(a.UpdatedAt) {
		return fmt.Errorf("%w: occurrence time cannot precede the current version", ErrInvalidAccount)
	}
	return nil
}

func (a *Account) advance(occurredAt time.Time) {
	a.Version++
	a.UpdatedAt = occurredAt.UTC()
}
